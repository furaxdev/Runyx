import { describe, expect, test } from "bun:test";
import { TextBuffer } from "../src/widgets/textinput.js";

describe("TextBuffer editing", () => {
  test("insert at cursor", () => {
    const b = new TextBuffer();
    b.insert("hello");
    expect(b.value()).toBe("hello");
    expect(b.cursorCol).toBe(5);
  });

  test("insert in the middle", () => {
    const b = new TextBuffer();
    b.insert("helo");
    b.cursorCol = 3;
    b.insert("l");
    expect(b.value()).toBe("hello");
  });

  test("backspace removes the char before the cursor", () => {
    const b = new TextBuffer();
    b.insert("hello");
    b.backspace();
    expect(b.value()).toBe("hell");
    expect(b.cursorCol).toBe(4);
  });

  test("backspace at column 0 joins with the previous line", () => {
    const b = new TextBuffer();
    b.insert("abc\ndef");
    b.cursorRow = 1;
    b.cursorCol = 0;
    b.backspace();
    expect(b.value()).toBe("abcdef");
    expect(b.cursorRow).toBe(0);
    expect(b.cursorCol).toBe(3);
  });

  test("deleteForward removes the char at the cursor", () => {
    const b = new TextBuffer();
    b.insert("hello");
    b.cursorCol = 0;
    b.deleteForward();
    expect(b.value()).toBe("ello");
  });

  test("newline (paste with embedded \\n) splits into multiple lines", () => {
    const b = new TextBuffer();
    b.insert("line one\nline two");
    expect(b.value()).toBe("line one\nline two");
    expect(b.lineCount).toBe(2);
  });

  test("moveLeft/Right crosses line boundaries", () => {
    const b = new TextBuffer();
    b.insert("ab\ncd");
    b.cursorRow = 1;
    b.cursorCol = 0;
    b.moveLeft();
    expect(b.cursorRow).toBe(0);
    expect(b.cursorCol).toBe(2);
    b.moveRight();
    expect(b.cursorRow).toBe(1);
    expect(b.cursorCol).toBe(0);
  });

  test("moveUp/Down clamps column to the shorter line's length", () => {
    const b = new TextBuffer();
    b.insert("longline\nhi");
    b.cursorRow = 0;
    b.cursorCol = 8;
    b.moveDown();
    expect(b.cursorRow).toBe(1);
    expect(b.cursorCol).toBe(2); // clamped to "hi".length
  });

  test("isEmpty is true only for a single empty line", () => {
    const b = new TextBuffer();
    expect(b.isEmpty()).toBe(true);
    b.insert("x");
    expect(b.isEmpty()).toBe(false);
  });

  test("setValue replaces the whole buffer and moves cursor to the end", () => {
    const b = new TextBuffer();
    b.setValue("abc\ndef");
    expect(b.cursorRow).toBe(1);
    expect(b.cursorCol).toBe(3);
  });

  test("insert handles multi-codepoint graphemes without corrupting the string", () => {
    const b = new TextBuffer();
    b.insert("café"); // café
    expect(b.value()).toBe("café");
  });
});
