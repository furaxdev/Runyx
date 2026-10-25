import { describe, expect, test } from "bun:test";
import { InputParser, type KeyEvent, type MouseEvent, type PasteEvent } from "../src/term/input.js";

describe("InputParser keyboard", () => {
  test("plain ASCII character", () => {
    const p = new InputParser();
    const [e] = p.feed("a") as [KeyEvent];
    expect(e.key).toBe("a");
    expect(e.ctrl).toBe(false);
  });

  test("Enter from CR", () => {
    const p = new InputParser();
    const [e] = p.feed("\r") as [KeyEvent];
    expect(e.key).toBe("Enter");
  });

  test("Ctrl+C", () => {
    const p = new InputParser();
    const [e] = p.feed("\x03") as [KeyEvent];
    expect(e.key).toBe("c");
    expect(e.ctrl).toBe(true);
  });

  test("arrow keys (legacy xterm)", () => {
    const p = new InputParser();
    expect((p.feed("\x1b[A")[0] as KeyEvent).key).toBe("Up");
    expect((p.feed("\x1b[B")[0] as KeyEvent).key).toBe("Down");
    expect((p.feed("\x1b[C")[0] as KeyEvent).key).toBe("Right");
    expect((p.feed("\x1b[D")[0] as KeyEvent).key).toBe("Left");
  });

  test("arrow key with ctrl modifier", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[1;5C") as [KeyEvent]; // Ctrl+Right
    expect(e.key).toBe("Right");
    expect(e.ctrl).toBe(true);
  });

  test("Delete / Home / End via tilde sequences", () => {
    const p = new InputParser();
    expect((p.feed("\x1b[3~")[0] as KeyEvent).key).toBe("Delete");
    expect((p.feed("\x1b[1~")[0] as KeyEvent).key).toBe("Home");
    expect((p.feed("\x1b[4~")[0] as KeyEvent).key).toBe("End");
  });

  test("function keys F1-F4 (application mode)", () => {
    const p = new InputParser();
    expect((p.feed("\x1bOP")[0] as KeyEvent).key).toBe("F1");
    expect((p.feed("\x1bOQ")[0] as KeyEvent).key).toBe("F2");
  });

  test("F5 via CSI tilde", () => {
    const p = new InputParser();
    expect((p.feed("\x1b[15~")[0] as KeyEvent).key).toBe("F5");
  });

  test("Alt+char from ESC-prefixed byte", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1bx") as [KeyEvent];
    expect(e.key).toBe("x");
    expect(e.alt).toBe(true);
  });

  test("Kitty protocol basic form: CSI <codepoint>u", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[97u") as [KeyEvent]; // 97 = 'a'
    expect(e.key).toBe("a");
  });

  test("split sequence across two feed() calls", () => {
    const p = new InputParser();
    expect(p.feed("\x1b[1")).toEqual([]);
    const events = p.feed(";5C");
    expect((events[0] as KeyEvent).key).toBe("Right");
    expect((events[0] as KeyEvent).ctrl).toBe(true);
  });

  test("lone Escape flushed after timeout via flushPendingEscape", () => {
    const p = new InputParser();
    expect(p.feed("\x1b")).toEqual([]); // ambiguous, waiting for more
    const flushed = p.flushPendingEscape();
    expect((flushed[0] as KeyEvent).key).toBe("Escape");
  });
});

describe("InputParser mouse (SGR 1006)", () => {
  test("left click down", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[<0;10;5M") as [MouseEvent];
    expect(e.type).toBe("mouse");
    expect(e.action).toBe("down");
    expect(e.button).toBe("left");
    expect(e.col).toBe(9);
    expect(e.row).toBe(4);
  });

  test("release (lowercase m)", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[<0;10;5m") as [MouseEvent];
    expect(e.action).toBe("up");
  });

  test("wheel up", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[<64;1;1M") as [MouseEvent];
    expect(e.action).toBe("wheel");
    expect(e.button).toBe("wheelUp");
  });

  test("motion/hover (bit 32 set)", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[<32;3;3M") as [MouseEvent];
    expect(e.action).toBe("move");
  });

  test("shift+click sets modifier", () => {
    const p = new InputParser();
    const [e] = p.feed("\x1b[<4;1;1M") as [MouseEvent]; // cb=4 => shift bit + left button
    expect(e.shift).toBe(true);
    expect(e.button).toBe("left");
  });
});

describe("InputParser bracketed paste", () => {
  test("captures pasted text verbatim, including embedded newlines", () => {
    const p = new InputParser();
    const events = p.feed("\x1b[200~hello\nworld\x1b[201~");
    const paste = events[0] as PasteEvent;
    expect(paste.type).toBe("paste");
    expect(paste.text).toBe("hello\nworld");
  });

  test("paste split across chunks", () => {
    const p = new InputParser();
    expect(p.feed("\x1b[200~partial")).toEqual([]);
    const events = p.feed(" rest\x1b[201~");
    expect((events[0] as PasteEvent).text).toBe("partial rest");
  });

  test("keys inside a bracketed paste are not interpreted as key events", () => {
    const p = new InputParser();
    const events = p.feed("\x1b[200~\x1b[A\x1b[201~"); // literal arrow-key bytes as pasted text
    expect(events.length).toBe(1);
    expect(events[0]!.type).toBe("paste");
    expect((events[0] as PasteEvent).text).toBe("\x1b[A");
  });
});

describe("InputParser focus events", () => {
  test("focus in / out", () => {
    const p = new InputParser();
    expect(p.feed("\x1b[I")[0]).toEqual({ type: "focus", focused: true });
    expect(p.feed("\x1b[O")[0]).toEqual({ type: "focus", focused: false });
  });
});
