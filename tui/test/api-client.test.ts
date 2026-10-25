// Same regression shape as web/src/lib/api.test.ts: the TUI's client
// talks to the identical /v1/chat/completions proxy, so it needs the
// same guarantee that a well-formed SSE stream's promise actually
// resolves (an unbounded/never-ending stream is the underlying bug
// behind the demo CI's "cursor never detaches" failure — see
// docs/DECISIONS.md).
import { afterEach, describe, expect, test } from "bun:test";
import { RunyxClient, ApiError } from "../src/api/client.js";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function sseResponse(lines: string[]): Response {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const line of lines) controller.enqueue(encoder.encode(line));
      controller.close();
    },
  });
  return new Response(body, { status: 200 });
}

describe("RunyxClient.streamChat", () => {
  test("resolves on [DONE], collecting every delta", async () => {
    let capturedBody: any;
    globalThis.fetch = (async (_url: string, init?: RequestInit) => {
      capturedBody = JSON.parse(init!.body as string);
      return sseResponse([
        'data: {"choices":[{"delta":{"content":"Hel"}}]}\n\n',
        'data: {"choices":[{"delta":{"content":"lo"}}]}\n\n',
        "data: [DONE]\n\n",
      ]);
    }) as unknown as typeof fetch;

    const client = new RunyxClient("http://127.0.0.1:11535");
    let collected = "";
    await client.streamChat("qwen", [{ role: "user", content: "hi" }], (d) => (collected += d));

    expect(collected).toBe("Hello");
    expect(capturedBody.model).toBe("qwen");
    expect(capturedBody.max_tokens).toBeUndefined();
  });

  test("resolves on connection close without an explicit [DONE]", async () => {
    globalThis.fetch = (async () => sseResponse(['data: {"choices":[{"delta":{"content":"ok"}}]}\n\n'])) as unknown as typeof fetch;

    const client = new RunyxClient("http://127.0.0.1:11535");
    let collected = "";
    await client.streamChat("qwen", [{ role: "user", content: "hi" }], (d) => (collected += d));
    expect(collected).toBe("ok");
  });

  test("includes max_tokens/temperature only when provided", async () => {
    let capturedBody: any;
    globalThis.fetch = (async (_url: string, init?: RequestInit) => {
      capturedBody = JSON.parse(init!.body as string);
      return sseResponse(["data: [DONE]\n\n"]);
    }) as unknown as typeof fetch;

    const client = new RunyxClient("http://127.0.0.1:11535");
    await client.streamChat("qwen", [{ role: "user", content: "hi" }], () => {}, undefined, {
      maxTokens: 64,
      temperature: 0,
    });
    expect(capturedBody.max_tokens).toBe(64);
    expect(capturedBody.temperature).toBe(0);
  });

  test("a non-OK response throws ApiError instead of hanging", async () => {
    globalThis.fetch = (async () => new Response("boom", { status: 500 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    await expect(client.streamChat("qwen", [], () => {})).rejects.toBeInstanceOf(ApiError);
  });
});

describe("RunyxClient JSON helpers", () => {
  test("listModels unwraps the {models:[...]} envelope", async () => {
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ models: [{ name: "qwen", modified_at: "", size: 1, digest: "x" }] }), {
        status: 200,
      })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    const models = await client.listModels();
    expect(models).toEqual([{ name: "qwen", modified_at: "", size: 1, digest: "x" }]);
  });

  test("a 404 surfaces as ApiError with the status code", async () => {
    globalThis.fetch = (async () => new Response("not found", { status: 404 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    try {
      await client.listConversations();
      throw new Error("expected rejection");
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError);
      expect((e as ApiError).status).toBe(404);
    }
  });
});

describe("RunyxClient approvals", () => {
  test("listApprovals returns the pending array as-is", async () => {
    const pending = [{ id: "a1", kind: "command", summary: "echo hi", detail: "echo hi", hash: "abc", created_at: "", expires_at: "" }];
    globalThis.fetch = (async () => new Response(JSON.stringify(pending), { status: 200 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    expect(await client.listApprovals()).toEqual(pending);
  });

  test("decideApproval sends the hash and allow flag", async () => {
    let capturedBody: any;
    let capturedUrl = "";
    globalThis.fetch = (async (url: string, init?: RequestInit) => {
      capturedUrl = url;
      capturedBody = JSON.parse(init!.body as string);
      return new Response(null, { status: 200 });
    }) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    await client.decideApproval("a1", "abc", true);
    expect(capturedUrl).toBe("http://127.0.0.1:11535/api/runyx/approvals/a1/decide");
    expect(capturedBody).toEqual({ Hash: "abc", Allow: true });
  });

  test("decideApproval throws ApiError on a non-OK response (e.g. hash mismatch)", async () => {
    globalThis.fetch = (async () => new Response("conflict", { status: 409 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    await expect(client.decideApproval("a1", "wrong", true)).rejects.toBeInstanceOf(ApiError);
  });
});

describe("RunyxClient pullModel", () => {
  function ndjsonResponse(lines: object[]): Response {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const line of lines) controller.enqueue(encoder.encode(JSON.stringify(line) + "\n"));
        controller.close();
      },
    });
    return new Response(body, { status: 200 });
  }

  test("reports progress and resolves on done:true", async () => {
    globalThis.fetch = (async () =>
      ndjsonResponse([
        { downloaded: 1000, total: 4000, done: false },
        { downloaded: 4000, total: 4000, done: false },
        { done: true, name: "qwen" },
      ])) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    const progress: number[] = [];
    await client.pullModel("qwen-id", (p) => progress.push(p.downloaded));
    expect(progress).toEqual([1000, 4000]);
  });

  test("rejects when the final NDJSON line carries an error", async () => {
    globalThis.fetch = (async () => ndjsonResponse([{ error: "disk full", done: true }])) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    await expect(client.pullModel("qwen-id", () => {})).rejects.toThrow("disk full");
  });

  test("throws ApiError on a non-OK response", async () => {
    globalThis.fetch = (async () => new Response("nope", { status: 404 })) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    await expect(client.pullModel("qwen-id", () => {})).rejects.toBeInstanceOf(ApiError);
  });
});

describe("RunyxClient pairingQRCode", () => {
  test("sends the device name and returns url + modules as-is", async () => {
    let capturedBody: any;
    const payload = { url: "runyx://pair?d=abc", modules: [[true, false], [false, true]] };
    globalThis.fetch = (async (_url: string, init?: RequestInit) => {
      capturedBody = JSON.parse(init!.body as string);
      return new Response(JSON.stringify(payload), { status: 200 });
    }) as unknown as typeof fetch;
    const client = new RunyxClient("http://127.0.0.1:11535");
    const qr = await client.pairingQRCode("tui-device");
    expect(capturedBody).toEqual({ DeviceName: "tui-device" });
    expect(qr).toEqual(payload);
  });
});
