// A lightweight time-driven spinner — braille frames by default, ASCII
// fallback for --plain/no-unicode.
const BRAILLE_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const ASCII_FRAMES = ["|", "/", "-", "\\"];

export function spinnerFrame(nowMs: number, ascii = false): string {
  const frames = ascii ? ASCII_FRAMES : BRAILLE_FRAMES;
  const i = Math.floor(nowMs / 80) % frames.length;
  return frames[i]!;
}
