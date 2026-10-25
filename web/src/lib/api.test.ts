// Regression test for the "cursor never disappears" demo failure: Chat.svelte
// keeps `streaming = true` (and the `.cursor` element mounted) until
// streamChat's promise resolves, so a fake SSE stream that properly
// terminates must make that promise resolve — otherwise the UI's
// streaming cursor would never detach, same as the real bug that showed up
// as a Playwright timeout in the demo CI.
import { describe, expect, test, vi, afterEach } from "vitest";
import { api } from "./api";

function sseResponse(lines: string[], opts: { closeWithoutDone?: boolean } = {}): Response {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const line of lines) {
        controller.enqueue(encoder.encode(line));
      }
      // A real server either sends "data: [DONE]\n\n" and then closes the
      // connection, or (the bug this guards against) just hangs without
      // ever closing — simulated by never calling controller.close().
      if (!opts.closeWithoutDone) controller.close();
    },
  });
  return new Response(body, { status: 200 });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("streamChat: well-formed SSE stream termination", () => {
  test("resolves once [DONE] is seen, collecting every delta first", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      sseResponse([
        'data: {"choices":[{"delta":{"content":"Hel"}}]}\n\n',
        'data: {"choices":[{"delta":{"content":"lo"}}]}\n\n',
        "data: [DONE]\n\n",
      ])
    );
    vi.stubGlobal("fetch", fetchMock);

    let collected = "";
    await api.streamChat("qwen", [{ role: "user", content: "hi" }], (delta) => {
      collected += delta;
    });

    expect(collected).toBe("Hello");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  test("resolves on connection close even without an explicit [DONE] line", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      sseResponse(['data: {"choices":[{"delta":{"content":"ok"}}]}\n\n'])
    );
    vi.stubGlobal("fetch", fetchMock);

    let collected = "";
    await api.streamChat("qwen", [{ role: "user", content: "hi" }], (delta) => {
      collected += delta;
    });

    expect(collected).toBe("ok");
  });

  test("passes max_tokens/temperature through when provided (demo determinism)", async () => {
    const fetchMock = vi.fn().mockResolvedValue(sseResponse(["data: [DONE]\n\n"]));
    vi.stubGlobal("fetch", fetchMock);

    await api.streamChat("qwen", [{ role: "user", content: "hi" }], () => {}, undefined, {
      maxTokens: 64,
      temperature: 0,
    });

    const [, init] = fetchMock.mock.calls[0]!;
    const sentBody = JSON.parse((init as RequestInit).body as string);
    expect(sentBody.max_tokens).toBe(64);
    expect(sentBody.temperature).toBe(0);
  });
});

describe("streamChat: abort", () => {
  test("an aborted signal rejects instead of hanging forever", async () => {
    const controller = new AbortController();
    const fetchMock = vi.fn().mockImplementation((_url: string, init: RequestInit) => {
      return new Promise((_resolve, reject) => {
        init.signal?.addEventListener("abort", () => {
          const err = new Error("aborted");
          err.name = "AbortError";
          reject(err);
        });
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    const promise = api.streamChat("qwen", [{ role: "user", content: "hi" }], () => {}, controller.signal);
    controller.abort();

    await expect(promise).rejects.toThrow();
  });
});
