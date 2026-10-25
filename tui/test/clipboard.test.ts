import { describe, expect, test, afterEach } from "bun:test";
import { copyToClipboard, toBase64, wrapForTmux } from "../src/selection/clipboard.js";
import { osc52Copy } from "../src/term/ansi.js";

describe("clipboard: OSC 52 payload", () => {
  test("encodes text as base64", () => {
    expect(toBase64("hello")).toBe(Buffer.from("hello").toString("base64"));
  });

  test("round-trips UTF-8 text (not just ASCII)", () => {
    const b64 = toBase64("café 中文");
    expect(Buffer.from(b64, "base64").toString("utf8")).toBe("café 中文");
  });
});

describe("clipboard: tmux passthrough wrapping", () => {
  const original = process.env.TMUX;
  afterEach(() => {
    if (original === undefined) delete process.env.TMUX;
    else process.env.TMUX = original;
  });

  test("outside tmux, the OSC sequence passes through unwrapped", () => {
    delete process.env.TMUX;
    const osc = osc52Copy("aGVsbG8=");
    expect(wrapForTmux(osc)).toBe(osc);
  });

  test("inside tmux, the sequence is wrapped in a DCS passthrough", () => {
    process.env.TMUX = "/tmp/tmux-1000/default,1234,0";
    const osc = osc52Copy("aGVsbG8=");
    const wrapped = wrapForTmux(osc);
    expect(wrapped.startsWith("\x1bPtmux;")).toBe(true);
    expect(wrapped.endsWith("\x1b\\")).toBe(true);
  });
});

describe("copyToClipboard", () => {
  test("does not throw when stdout is not a TTY (CI/non-interactive)", async () => {
    const fakeStream = { isTTY: false, write: () => true } as unknown as NodeJS.WriteStream;
    const method = await copyToClipboard("hello", fakeStream);
    expect(["osc52", "cli", "none"]).toContain(method);
  });
});
