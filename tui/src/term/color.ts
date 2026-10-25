// Color handling: an RGB color degrades to 256-color or 16-color ANSI
// depending on what the terminal supports, honors NO_COLOR, and
// supplies the design's light/dark token palettes (same accent -
// "ink blue" - as the web UI's CSS variables).

export type RGB = readonly [r: number, g: number, b: number];

export type ColorLevel = "none" | "16" | "256" | "truecolor";

export function detectColorLevel(env: NodeJS.ProcessEnv = process.env, isTTY = process.stdout.isTTY): ColorLevel {
  if (env.NO_COLOR !== undefined) return "none";
  if (env.RUNYX_TUI_MONOCHROME === "1") return "none";
  if (!isTTY) return "none";
  if (env.COLORTERM === "truecolor" || env.COLORTERM === "24bit") return "truecolor";
  const term = env.TERM ?? "";
  if (/^(xterm-kitty|wezterm|alacritty)/.test(term)) return "truecolor";
  if (env.TERM_PROGRAM === "iTerm.app" || env.TERM_PROGRAM === "WezTerm" || env.TERM_PROGRAM === "vscode") return "truecolor";
  if (env.WT_SESSION) return "truecolor"; // Windows Terminal
  if (/-256color$/.test(term)) return "256";
  if (term === "screen" || term === "tmux" || /^screen-|^tmux-/.test(term)) return "256";
  if (term === "" || term === "dumb") return "none";
  return "16";
}

/** Nearest 256-color palette index for an RGB triple. */
export function rgbTo256(rgb: RGB): number {
  const [r, g, b] = rgb;
  // Grayscale ramp (232-255) when r≈g≈b.
  if (Math.abs(r - g) < 8 && Math.abs(g - b) < 8 && Math.abs(r - b) < 8) {
    if (r < 8) return 16;
    if (r > 248) return 231;
    return 232 + Math.round(((r - 8) / 247) * 24);
  }
  const toIdx = (v: number) => Math.round((v / 255) * 5);
  return 16 + 36 * toIdx(r) + 6 * toIdx(g) + toIdx(b);
}

const ANSI16: readonly { rgb: RGB; idx: number }[] = [
  { rgb: [0, 0, 0], idx: 30 },
  { rgb: [205, 49, 49], idx: 31 },
  { rgb: [13, 188, 121], idx: 32 },
  { rgb: [229, 229, 16], idx: 33 },
  { rgb: [36, 114, 200], idx: 34 },
  { rgb: [188, 63, 188], idx: 35 },
  { rgb: [17, 168, 205], idx: 36 },
  { rgb: [229, 229, 229], idx: 37 },
  { rgb: [102, 102, 102], idx: 90 },
  { rgb: [241, 76, 76], idx: 91 },
  { rgb: [35, 209, 139], idx: 92 },
  { rgb: [245, 245, 67], idx: 93 },
  { rgb: [59, 142, 234], idx: 94 },
  { rgb: [214, 112, 214], idx: 95 },
  { rgb: [41, 184, 219], idx: 96 },
  { rgb: [229, 229, 229], idx: 97 },
];

export function rgbTo16(rgb: RGB, background: boolean): number {
  let best = ANSI16[0]!;
  let bestDist = Infinity;
  for (const c of ANSI16) {
    const d = (c.rgb[0] - rgb[0]) ** 2 + (c.rgb[1] - rgb[1]) ** 2 + (c.rgb[2] - rgb[2]) ** 2;
    if (d < bestDist) {
      bestDist = d;
      best = c;
    }
  }
  return background ? best.idx + 10 : best.idx;
}

export interface Theme {
  name: "light" | "dark";
  background: RGB;
  surface: RGB;
  surfaceElevated: RGB;
  border: RGB;
  text: RGB;
  textMuted: RGB;
  accent: RGB; // "ink blue", matching web/src/lib/theme.css's --accent
  accentContrast: RGB;
  danger: RGB;
  success: RGB;
  warning: RGB;
}

// Same accent ("ink blue") as the web UI's design tokens.
export const DARK_THEME: Theme = {
  name: "dark",
  background: [15, 17, 21],
  surface: [22, 25, 31],
  surfaceElevated: [30, 33, 41],
  border: [48, 52, 61],
  text: [230, 232, 236],
  textMuted: [148, 154, 166],
  accent: [74, 123, 224], // ink blue
  accentContrast: [255, 255, 255],
  danger: [224, 94, 94],
  success: [94, 196, 140],
  warning: [224, 176, 74],
};

export const LIGHT_THEME: Theme = {
  name: "light",
  background: [250, 249, 246],
  surface: [255, 255, 255],
  surfaceElevated: [244, 243, 239],
  border: [223, 220, 213],
  text: [30, 32, 36],
  textMuted: [112, 118, 128],
  accent: [37, 92, 196], // ink blue, darkened for contrast on light bg
  accentContrast: [255, 255, 255],
  danger: [188, 56, 56],
  success: [42, 138, 92],
  warning: [168, 120, 24],
};

/**
 * Best-effort terminal background detection via OSC 11 query. Resolves
 * with "dark" if the terminal doesn't answer within `timeoutMs` (most
 * terminals that don't support it simply stay silent) — dark is this
 * product's existing default (see web's theme.css default).
 */
export function detectTerminalTheme(
  input: NodeJS.ReadStream,
  output: NodeJS.WriteStream,
  timeoutMs = 200
): Promise<"light" | "dark"> {
  return new Promise((resolve) => {
    if (!input.isTTY || !output.isTTY) {
      resolve("dark");
      return;
    }
    let settled = false;
    const finish = (v: "light" | "dark") => {
      if (settled) return;
      settled = true;
      input.removeListener("data", onData);
      clearTimeout(timer);
      resolve(v);
    };
    const timer = setTimeout(() => finish("dark"), timeoutMs);
    const onData = (chunk: Buffer) => {
      const s = chunk.toString("latin1");
      // Expect: ESC ] 11 ; rgb:RRRR/GGGG/BBBB ESC \  (or BEL-terminated)
      const m = /]11;rgb:([0-9a-f]+)\/([0-9a-f]+)\/([0-9a-f]+)/i.exec(s);
      if (m) {
        const r = parseInt(m[1]!.slice(0, 2), 16);
        const g = parseInt(m[2]!.slice(0, 2), 16);
        const b = parseInt(m[3]!.slice(0, 2), 16);
        const luma = 0.299 * r + 0.587 * g + 0.114 * b;
        finish(luma > 128 ? "light" : "dark");
      }
    };
    input.on("data", onData);
    output.write("\x1b]11;?\x1b\\");
  });
}
