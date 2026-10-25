// Terminal lifecycle: raw mode, alternate screen, mouse/paste/focus
// reporting, resize handling, a dirty-flag render loop capped at 60fps
// (coalesced to ~30fps while a "streaming" flag is set, so token-by-
// token chat output doesn't redraw every single token), and guaranteed
// cleanup — normal exit, Ctrl+C, SIGTERM, and uncaught exceptions all
// restore the terminal to a usable state before the process dies.
import { EventEmitter } from "node:events";
import {
  ALT_SCREEN_OFF,
  ALT_SCREEN_ON,
  BRACKETED_PASTE_OFF,
  BRACKETED_PASTE_ON,
  CURSOR_HIDE,
  CURSOR_SHOW,
  FOCUS_REPORT_OFF,
  FOCUS_REPORT_ON,
  KITTY_KEYBOARD_POP,
  KITTY_KEYBOARD_PUSH,
  MOUSE_OFF,
  MOUSE_ON,
  RESET_SGR,
} from "./ansi.js";
import { type ColorLevel, detectColorLevel, detectTerminalTheme, DARK_THEME, LIGHT_THEME, type Theme } from "./color.js";
import { Screen } from "./buffer.js";
import { InputParser, type InputEvent } from "./input.js";

export interface TerminalEvents {
  key: (e: Extract<InputEvent, { type: "key" }>) => void;
  mouse: (e: Extract<InputEvent, { type: "mouse" }>) => void;
  paste: (e: Extract<InputEvent, { type: "paste" }>) => void;
  focus: (e: Extract<InputEvent, { type: "focus" }>) => void;
  resize: (size: { width: number; height: number }) => void;
}

const FRAME_MS_IDLE = 1000 / 60;
const FRAME_MS_STREAMING = 1000 / 30;
const ESCAPE_TIMEOUT_MS = 25;

export class Terminal extends EventEmitter {
  screen: Screen;
  colorLevel: ColorLevel;
  theme: Theme = DARK_THEME;

  private input = process.stdin;
  private output = process.stdout;
  private parser = new InputParser();
  private escapeTimer: NodeJS.Timeout | null = null;
  private started = false;
  private dirty = false;
  private streaming = false;
  private renderTimer: ReturnType<typeof setTimeout> | null = null;
  private drawFn: ((s: Screen) => void) | null = null;
  private firstFrame = true;
  private cleanupRan = false;
  private onDataBound = this.onData.bind(this);
  private onResizeBound = this.onResize.bind(this);
  private signalHandlers: Array<[NodeJS.Signals, () => void]> = [];

  constructor() {
    super();
    this.colorLevel = detectColorLevel();
    this.screen = new Screen(this.output.columns ?? 80, (this.output.rows ?? 24) - 0);
  }

  get supportsMouse(): boolean {
    return this.output.isTTY === true;
  }

  async start(): Promise<void> {
    if (this.started) return;
    this.started = true;

    if (this.output.isTTY) {
      this.output.write(ALT_SCREEN_ON + CURSOR_HIDE + BRACKETED_PASTE_ON + FOCUS_REPORT_ON);
      if (this.supportsMouse) this.output.write(MOUSE_ON);
      this.output.write(KITTY_KEYBOARD_PUSH);
    }
    if (this.input.isTTY) {
      this.input.setRawMode?.(true);
    }
    this.input.setEncoding("utf8");
    this.input.resume();
    this.input.on("data", this.onDataBound);
    this.output.on("resize", this.onResizeBound);

    for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"] as NodeJS.Signals[]) {
      const handler = () => {
        this.stop();
        process.exit(130);
      };
      this.signalHandlers.push([sig, handler]);
      process.on(sig, handler);
    }
    process.on("uncaughtException", this.onFatal);
    process.on("exit", this.onExitSync);

    try {
      this.theme = (await detectTerminalTheme(this.input, this.output)) === "light" ? LIGHT_THEME : DARK_THEME;
    } catch {
      this.theme = DARK_THEME;
    }
  }

  stop(): void {
    if (this.cleanupRan) return;
    this.cleanupRan = true;
    if (this.renderTimer) clearTimeout(this.renderTimer);
    if (this.escapeTimer) clearTimeout(this.escapeTimer);
    for (const [sig, handler] of this.signalHandlers) process.removeListener(sig, handler);
    process.removeListener("uncaughtException", this.onFatal);
    process.removeListener("exit", this.onExitSync);
    this.input.removeListener("data", this.onDataBound);
    this.output.removeListener("resize", this.onResizeBound);
    if (this.input.isTTY) this.input.setRawMode?.(false);
    this.input.pause();
    if (this.output.isTTY) {
      this.output.write(
        RESET_SGR + KITTY_KEYBOARD_POP + (this.supportsMouse ? MOUSE_OFF : "") + FOCUS_REPORT_OFF + BRACKETED_PASTE_OFF + CURSOR_SHOW + ALT_SCREEN_OFF
      );
    }
    this.started = false;
  }

  private onFatal = (err: unknown): void => {
    this.stop();
    // eslint-disable-next-line no-console
    console.error(err);
    process.exit(1);
  };
  private onExitSync = (): void => {
    this.stop();
  };

  private onResize(): void {
    this.screen.resize(this.output.columns ?? 80, this.output.rows ?? 24);
    this.emit("resize", { width: this.screen.width, height: this.screen.height });
    this.requestRender();
  }

  private onData(chunk: string): void {
    if (this.escapeTimer) {
      clearTimeout(this.escapeTimer);
      this.escapeTimer = null;
    }
    const events = this.parser.feed(chunk);
    for (const e of events) this.dispatch(e);
    // A lone trailing ESC might be a real Escape keypress or the start
    // of a sequence that got split across chunks — give it a short
    // grace period before deciding it was standalone Escape.
    this.escapeTimer = setTimeout(() => {
      for (const e of this.parser.flushPendingEscape()) this.dispatch(e);
    }, ESCAPE_TIMEOUT_MS);
  }

  private dispatch(e: InputEvent): void {
    if (e.type === "key") this.emit("key", e);
    else if (e.type === "mouse") this.emit("mouse", e);
    else if (e.type === "paste") this.emit("paste", e);
    else if (e.type === "focus") this.emit("focus", e);
  }

  /** Registers the draw callback invoked on every render. */
  setDrawFn(fn: (s: Screen) => void): void {
    this.drawFn = fn;
  }

  /** Marks that token streaming is in progress, coalescing redraws to ~30fps. */
  setStreaming(streaming: boolean): void {
    this.streaming = streaming;
  }

  /** Schedules a redraw on the next frame tick, if one isn't already pending. */
  requestRender(): void {
    this.dirty = true;
    if (this.renderTimer) return;
    const frameMs = this.streaming ? FRAME_MS_STREAMING : FRAME_MS_IDLE;
    this.renderTimer = setTimeout(() => this.flush(), frameMs);
  }

  /** Forces an immediate synchronous render (used sparingly — e.g. on startup). */
  renderNow(): void {
    if (this.renderTimer) {
      clearTimeout(this.renderTimer);
      this.renderTimer = null;
    }
    this.flush();
  }

  private flush(): void {
    this.renderTimer = null;
    if (!this.dirty || !this.drawFn) return;
    this.dirty = false;
    this.screen.clear();
    this.drawFn(this.screen);
    const out = this.firstFrame ? this.screen.fullRepaint(this.colorLevel) : this.screen.diff(this.colorLevel, true);
    this.firstFrame = false;
    if (out) this.output.write(out);
  }
}

/** Renders the current frame's grid as SVG — used by `--dump-frame` and visual tests. */
export function screenToSVG(screen: Screen, theme: Theme, cellWidthPx = 9, cellHeightPx = 18): string {
  const w = screen.width * cellWidthPx;
  const h = screen.height * cellHeightPx;
  const bg = `rgb(${theme.background.join(",")})`;
  let body = `<rect width="${w}" height="${h}" fill="${bg}"/>`;
  for (let row = 0; row < screen.height; row++) {
    for (let col = 0; col < screen.width; col++) {
      const cell = screen.frontAt(row, col);
      if (!cell || cell.width === 0) continue;
      const x = col * cellWidthPx;
      const y = row * cellHeightPx;
      if (cell.bg) {
        body += `<rect x="${x}" y="${y}" width="${cellWidthPx * cell.width}" height="${cellHeightPx}" fill="rgb(${cell.bg.join(",")})"/>`;
      }
      if (cell.text && cell.text !== " ") {
        const fg = cell.fg ? `rgb(${cell.fg.join(",")})` : `rgb(${theme.text.join(",")})`;
        const weight = cell.attrs.bold ? "bold" : "normal";
        const style = cell.attrs.italic ? "italic" : "normal";
        const esc = escapeXML(cell.text);
        body += `<text x="${x}" y="${y + cellHeightPx * 0.8}" font-family="monospace" font-size="${cellHeightPx * 0.85}" font-weight="${weight}" font-style="${style}" fill="${fg}">${esc}</text>`;
      }
    }
  }
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${body}</svg>`;
}

function escapeXML(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
