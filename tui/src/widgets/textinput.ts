// A multi-line editable text buffer (the chat composer's core) with
// cursor movement, insertion/deletion, a simple undo-free edit history
// stack for Up/Down "recall previous message" use, and a render
// function that wraps for display and draws a visible cursor.
import type { Screen } from "../term/buffer.js";
import type { Rect } from "../layout/flex.js";
import type { RGB } from "../term/color.js";
import { stringWidth } from "../term/width.js";
import { wrapText } from "./text.js";

export class TextBuffer {
  private lines: string[] = [""];
  cursorRow = 0;
  cursorCol = 0; // code-point index within the line

  get lineCount(): number {
    return this.lines.length;
  }

  value(): string {
    return this.lines.join("\n");
  }

  setValue(text: string): void {
    this.lines = text.split("\n");
    if (this.lines.length === 0) this.lines = [""];
    this.cursorRow = this.lines.length - 1;
    this.cursorCol = codePoints(this.lines[this.cursorRow]!).length;
  }

  clear(): void {
    this.setValue("");
  }

  isEmpty(): boolean {
    return this.lines.length === 1 && this.lines[0] === "";
  }

  lineAt(i: number): string {
    return this.lines[i] ?? "";
  }

  /** Inserts text at the cursor, splitting into multiple lines on embedded "\n" (paste). */
  insert(text: string): void {
    const parts = text.split("\n");
    const line = this.lines[this.cursorRow]!;
    const cps = codePoints(line);
    const before = cps.slice(0, this.cursorCol).join("");
    const after = cps.slice(this.cursorCol).join("");
    if (parts.length === 1) {
      this.lines[this.cursorRow] = before + parts[0] + after;
      this.cursorCol += codePoints(parts[0]!).length;
      return;
    }
    const newLines = [before + parts[0], ...parts.slice(1, -1), parts[parts.length - 1] + after];
    this.lines.splice(this.cursorRow, 1, ...newLines);
    this.cursorRow += parts.length - 1;
    this.cursorCol = codePoints(parts[parts.length - 1]!).length;
  }

  newline(): void {
    this.insert("\n");
  }

  backspace(): void {
    const cps = codePoints(this.lines[this.cursorRow]!);
    if (this.cursorCol > 0) {
      cps.splice(this.cursorCol - 1, 1);
      this.lines[this.cursorRow] = cps.join("");
      this.cursorCol--;
    } else if (this.cursorRow > 0) {
      const prevLen = codePoints(this.lines[this.cursorRow - 1]!).length;
      this.lines[this.cursorRow - 1] = this.lines[this.cursorRow - 1]! + this.lines[this.cursorRow]!;
      this.lines.splice(this.cursorRow, 1);
      this.cursorRow--;
      this.cursorCol = prevLen;
    }
  }

  deleteForward(): void {
    const cps = codePoints(this.lines[this.cursorRow]!);
    if (this.cursorCol < cps.length) {
      cps.splice(this.cursorCol, 1);
      this.lines[this.cursorRow] = cps.join("");
    } else if (this.cursorRow < this.lines.length - 1) {
      this.lines[this.cursorRow] = this.lines[this.cursorRow]! + this.lines[this.cursorRow + 1]!;
      this.lines.splice(this.cursorRow + 1, 1);
    }
  }

  moveLeft(): void {
    if (this.cursorCol > 0) this.cursorCol--;
    else if (this.cursorRow > 0) {
      this.cursorRow--;
      this.cursorCol = codePoints(this.lines[this.cursorRow]!).length;
    }
  }
  moveRight(): void {
    const len = codePoints(this.lines[this.cursorRow]!).length;
    if (this.cursorCol < len) this.cursorCol++;
    else if (this.cursorRow < this.lines.length - 1) {
      this.cursorRow++;
      this.cursorCol = 0;
    }
  }
  moveUp(): void {
    if (this.cursorRow > 0) {
      this.cursorRow--;
      this.cursorCol = Math.min(this.cursorCol, codePoints(this.lines[this.cursorRow]!).length);
    }
  }
  moveDown(): void {
    if (this.cursorRow < this.lines.length - 1) {
      this.cursorRow++;
      this.cursorCol = Math.min(this.cursorCol, codePoints(this.lines[this.cursorRow]!).length);
    }
  }
  moveHome(): void {
    this.cursorCol = 0;
  }
  moveEnd(): void {
    this.cursorCol = codePoints(this.lines[this.cursorRow]!).length;
  }
}

function codePoints(s: string): string[] {
  return Array.from(s);
}

export interface TextInputStyle {
  fg?: RGB | null;
  bg?: RGB | null;
  placeholderFg?: RGB | null;
  cursorFg?: RGB | null;
  cursorBg?: RGB | null;
}

/** Renders the buffer wrapped to rect.width, drawing a visible cursor. Returns the total wrapped-line count (for sizing). */
export function drawTextInput(screen: Screen, rect: Rect, buf: TextBuffer, placeholder: string, style: TextInputStyle = {}): number {
  if (buf.isEmpty() && placeholder) {
    screen.writeText(rect.row, rect.col, placeholder, { fg: style.placeholderFg ?? null, bg: style.bg ?? null });
  }
  let screenRow = rect.row;
  let cursorScreenRow = rect.row;
  let cursorScreenCol = rect.col;
  let totalWrapped = 0;
  for (let i = 0; i < buf.lineCount; i++) {
    const line = buf.lineAt(i);
    const wrapped = wrapText(line, rect.width);
    totalWrapped += wrapped.length || 1;
    if (i === buf.cursorRow) {
      // Find which wrapped sub-line the cursor's column falls on.
      let remaining = buf.cursorCol;
      let subRow = 0;
      for (const w of wrapped) {
        const len = Array.from(w).length;
        if (remaining <= len) break;
        remaining -= len;
        subRow++;
      }
      cursorScreenRow = screenRow + subRow;
      cursorScreenCol = rect.col + stringWidth(Array.from(wrapped[subRow] ?? "").slice(0, remaining).join(""));
    }
    for (const w of wrapped.length ? wrapped : [""]) {
      if (screenRow < rect.row + rect.height) {
        screen.writeText(screenRow, rect.col, w, { fg: style.fg ?? null, bg: style.bg ?? null });
      }
      screenRow++;
    }
  }
  if (cursorScreenRow < rect.row + rect.height) {
    const cell = screen.backAt(cursorScreenRow, cursorScreenCol);
    screen.setCluster(cursorScreenRow, cursorScreenCol, cell?.text && cell.text !== "" ? cell.text : " ", 1, {
      fg: style.cursorFg ?? null,
      bg: style.cursorBg ?? null,
    });
  }
  return totalWrapped;
}
