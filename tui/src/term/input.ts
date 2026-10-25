// Parses raw stdin bytes into structured input events: key presses
// (legacy xterm sequences + a basic subset of the Kitty keyboard
// protocol when the terminal sends it), SGR mouse events (1006, with
// 1002 button-event and 1003 any-motion), bracketed paste, and focus
// in/out. A small state machine (not a regex-per-chunk pass) so a
// sequence split across two stdin chunks (a real possibility over SSH)
// is handled correctly.

export interface KeyEvent {
  type: "key";
  key: string; // "a", "Enter", "Up", "F5", "Escape", ...
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
  raw: string;
}
export type MouseButton = "left" | "middle" | "right" | "wheelUp" | "wheelDown" | "none";
export type MouseAction = "down" | "up" | "move" | "wheel";
export interface MouseEvent {
  type: "mouse";
  action: MouseAction;
  button: MouseButton;
  row: number;
  col: number;
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
}
export interface PasteEvent {
  type: "paste";
  text: string;
}
export interface FocusEvent {
  type: "focus";
  focused: boolean;
}
export type InputEvent = KeyEvent | MouseEvent | PasteEvent | FocusEvent;

const NAMED_KEYS: Record<string, string> = {
  A: "Up",
  B: "Down",
  C: "Right",
  D: "Left",
  H: "Home",
  F: "End",
  P: "F1",
  Q: "F2",
  R: "F3",
  S: "F4",
};
const TILDE_KEYS: Record<string, string> = {
  "1": "Home",
  "2": "Insert",
  "3": "Delete",
  "4": "End",
  "5": "PageUp",
  "6": "PageDown",
  "7": "Home",
  "8": "End",
  "11": "F1",
  "12": "F2",
  "13": "F3",
  "14": "F4",
  "15": "F5",
  "17": "F6",
  "18": "F7",
  "19": "F8",
  "20": "F9",
  "21": "F10",
  "23": "F11",
  "24": "F12",
};

function modsFromCode(n: number): { shift: boolean; alt: boolean; ctrl: boolean } {
  const m = Math.max(0, n - 1);
  return { shift: !!(m & 1), alt: !!(m & 2), ctrl: !!(m & 4) };
}

export class InputParser {
  private buf = "";
  private pasting = false;
  private pasteBuf = "";

  /** Feeds raw bytes (decoded as UTF-8 by the caller's stream) and returns any complete events. */
  feed(chunk: string): InputEvent[] {
    this.buf += chunk;
    const events: InputEvent[] = [];
    while (this.buf.length > 0) {
      const consumed = this.tryConsumeOne(events);
      if (consumed === 0) break; // need more bytes
      this.buf = this.buf.slice(consumed);
    }
    return events;
  }

  /** Call after a short idle timeout to flush a lone ESC as an Escape keypress. */
  flushPendingEscape(): InputEvent[] {
    if (this.buf === "\x1b") {
      this.buf = "";
      return [{ type: "key", key: "Escape", ctrl: false, alt: false, shift: false, raw: "\x1b" }];
    }
    return [];
  }

  private tryConsumeOne(events: InputEvent[]): number {
    const s = this.buf;
    if (this.pasting) {
      const end = s.indexOf("\x1b[201~");
      if (end === -1) {
        this.pasteBuf += s;
        return s.length; // consumed everything, still pasting
      }
      this.pasteBuf += s.slice(0, end);
      events.push({ type: "paste", text: this.pasteBuf });
      this.pasting = false;
      this.pasteBuf = "";
      return end + "\x1b[201~".length;
    }

    const c0 = s.charCodeAt(0);

    if (s.startsWith("\x1b[200~")) {
      this.pasting = true;
      this.pasteBuf = "";
      return 6;
    }
    if (s.startsWith("\x1b[I")) {
      events.push({ type: "focus", focused: true });
      return 3;
    }
    if (s.startsWith("\x1b[O")) {
      events.push({ type: "focus", focused: false });
      return 3;
    }
    if (s.startsWith("\x1b[<")) return this.consumeSGRMouse(s, events);
    if (s.startsWith("\x1bO") && s.length >= 3) {
      const letter = s[2]!;
      if (NAMED_KEYS[letter]) {
        events.push(keyEvent(NAMED_KEYS[letter]!, false, false, false, s.slice(0, 3)));
        return 3;
      }
    }
    if (s.startsWith("\x1b[")) return this.consumeCSI(s, events);
    if (c0 === 0x1b) {
      if (s.length === 1) return 0; // ambiguous: lone ESC or start of a sequence — wait for more / timeout
      // ESC followed by a printable char: treat as Alt+<char>.
      const ch = s[1]!;
      events.push(keyEvent(ch, false, true, /^[A-Z]$/.test(ch), s.slice(0, 2)));
      return 2;
    }
    return this.consumePlain(s, events);
  }

  private consumeCSI(s: string, events: InputEvent[]): number {
    const m = /^\x1b\[((?:\d+;?)*)([A-Za-z~])/.exec(s);
    if (!m) return s.length >= 32 ? 1 : 0; // malformed/too long: drop one byte to resync, else wait
    const params = m[1]!;
    const final = m[2]!;
    const len = m[0]!.length;
    const parts = params.split(";").filter((p) => p !== "");
    if (final === "~") {
      const code = parts[0] ?? "1";
      const modCode = parts[1] ? parseInt(parts[1], 10) : 1;
      const mods = modsFromCode(modCode);
      const key = TILDE_KEYS[code] ?? `Unknown~${code}`;
      events.push(keyEvent(key, mods.ctrl, mods.alt, mods.shift, m[0]!));
      return len;
    }
    if (final === "u") {
      // Kitty keyboard protocol basic form: CSI <unicode-codepoint>[;modifiers]u
      const codepoint = parseInt(parts[0] ?? "0", 10);
      const modCode = parts[1] ? parseInt(parts[1], 10) : 1;
      const mods = modsFromCode(modCode);
      const key = codepoint > 0 ? String.fromCodePoint(codepoint) : "Unknown";
      events.push(keyEvent(key, mods.ctrl, mods.alt, mods.shift, m[0]!));
      return len;
    }
    if (NAMED_KEYS[final]) {
      const modCode = parts[parts.length - 1] ? parseInt(parts[parts.length - 1]!, 10) : 1;
      const mods = parts.length > 1 ? modsFromCode(modCode) : { shift: false, alt: false, ctrl: false };
      events.push(keyEvent(NAMED_KEYS[final]!, mods.ctrl, mods.alt, mods.shift, m[0]!));
      return len;
    }
    if (final === "Z") {
      events.push(keyEvent("Tab", false, false, true, m[0]!)); // shift-tab (CSI Z)
      return len;
    }
    // Unrecognized CSI sequence: consume and ignore.
    return len;
  }

  private consumeSGRMouse(s: string, events: InputEvent[]): number {
    const m = /^\x1b\[<(\d+);(\d+);(\d+)([mM])/.exec(s);
    if (!m) return s.length >= 32 ? 1 : 0;
    const cb = parseInt(m[1]!, 10);
    const col = parseInt(m[2]!, 10) - 1;
    const row = parseInt(m[3]!, 10) - 1;
    const pressed = m[4] === "M";
    const len = m[0]!.length;

    const shift = !!(cb & 4);
    const alt = !!(cb & 8);
    const ctrl = !!(cb & 16);
    const motion = !!(cb & 32);
    const wheel = (cb & 64) !== 0;

    let action: MouseAction;
    let button: MouseButton;
    if (wheel) {
      action = "wheel";
      button = (cb & 1) === 0 ? "wheelUp" : "wheelDown";
    } else if (motion) {
      action = "move";
      button = buttonFromCb(cb);
    } else {
      action = pressed ? "down" : "up";
      button = buttonFromCb(cb);
    }
    events.push({ type: "mouse", action, button, row, col, shift, alt, ctrl });
    return len;
  }

  private consumePlain(s: string, events: InputEvent[]): number {
    const c0 = s.charCodeAt(0);
    if (c0 === 0x0d || c0 === 0x0a) {
      events.push(keyEvent("Enter", false, false, false, s[0]!));
      return 1;
    }
    if (c0 === 0x09) {
      events.push(keyEvent("Tab", false, false, false, s[0]!));
      return 1;
    }
    if (c0 === 0x7f || c0 === 0x08) {
      events.push(keyEvent("Backspace", false, false, false, s[0]!));
      return 1;
    }
    if (c0 >= 0x01 && c0 <= 0x1a) {
      // Ctrl+A..Ctrl+Z (Ctrl+C=0x03, Ctrl+D=0x04, etc.)
      const letter = String.fromCharCode(c0 + 0x60);
      events.push(keyEvent(letter, true, false, false, s[0]!));
      return 1;
    }
    // One UTF-8 character (JS strings are UTF-16; take one code point).
    const cp = s.codePointAt(0)!;
    const ch = String.fromCodePoint(cp);
    events.push(keyEvent(ch, false, false, /^[A-Z]$/.test(ch), ch));
    return ch.length;
  }
}

function buttonFromCb(cb: number): MouseButton {
  const b = cb & 3;
  if (b === 0) return "left";
  if (b === 1) return "middle";
  if (b === 2) return "right";
  return "none";
}

function keyEvent(key: string, ctrl: boolean, alt: boolean, shift: boolean, raw: string): KeyEvent {
  return { type: "key", key, ctrl, alt, shift, raw };
}
