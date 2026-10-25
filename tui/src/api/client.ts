// Thin client for the Runyx daemon's HTTP API, used by the TUI screens.
// Mirrors web/src/lib/api.ts's streamChat behavior deliberately (same
// SSE parsing, same optional maxTokens/temperature, same abort-signal
// support) since both clients talk to the exact same
// /v1/chat/completions proxy — see docs/DECISIONS.md's "streaming
// cursor never detached" entry for why an unbounded, unwatched stream
// is a real bug class, not just a demo-CI quirk.
import type {
  ApprovalRequest,
  ChatMessageIn,
  Conversation,
  DashboardStats,
  Message,
  ModelEntry,
  PairingQR,
  PermissionsSnapshot,
  RegistryEntry,
} from "./types.js";

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function asJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new ApiError(res.status, text || `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

export class RunyxClient {
  constructor(private baseUrl: string) {}

  private url(path: string): string {
    return `${this.baseUrl}${path}`;
  }

  async healthz(): Promise<boolean> {
    try {
      const res = await fetch(this.url("/healthz"), { signal: AbortSignal.timeout(2000) });
      return res.ok;
    } catch {
      return false;
    }
  }

  // --- models (Ollama-compatible) ---

  async listModels(): Promise<ModelEntry[]> {
    const res = await fetch(this.url("/api/tags"));
    const body = await asJSON<{ models: ModelEntry[] }>(res);
    return body.models ?? [];
  }

  // --- registry (curated catalog) ---

  async listRegistry(): Promise<RegistryEntry[]> {
    const res = await fetch(this.url("/api/runyx/registry"));
    return asJSON<RegistryEntry[]>(res);
  }

  /**
   * Pulls a registry model, reporting NDJSON progress lines as the
   * server sends them. Resolves once the server sends `done: true`;
   * rejects with the server's own error message if `error` is set on
   * that terminal line. signal lets the caller cancel mid-download
   * (the server's internal/downloader stops mid-chunk and leaves a
   * resumable .part file -- a later pull of the same model picks the
   * saved state back up, it doesn't restart from zero).
   */
  async pullModel(
    id: string,
    onProgress: (p: { downloaded: number; total: number }) => void,
    signal?: AbortSignal
  ): Promise<void> {
    const res = await fetch(this.url("/api/runyx/models/pull"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
      ...(signal ? { signal } : {}),
    });
    if (!res.ok || !res.body) {
      throw new ApiError(res.status, `pull request failed: HTTP ${res.status}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop() ?? "";
      for (const line of lines) {
        if (!line.trim()) continue;
        const chunk = JSON.parse(line);
        if (chunk.error) throw new Error(chunk.error);
        if (chunk.done) return;
        onProgress({ downloaded: chunk.downloaded ?? 0, total: chunk.total ?? 0 });
      }
    }
  }

  // --- conversations/messages ---

  async listConversations(): Promise<Conversation[]> {
    const res = await fetch(this.url("/api/runyx/conversations"));
    return asJSON<Conversation[]>(res);
  }

  async newConversation(title: string, model: string, projectId = ""): Promise<Conversation> {
    const res = await fetch(this.url("/api/runyx/conversations"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Title: title, Model: model, ProjectID: projectId }),
    });
    return asJSON<Conversation>(res);
  }

  async getMessages(conversationId: string, branch = ""): Promise<Message[]> {
    const qs = branch ? `?branch=${encodeURIComponent(branch)}` : "";
    const res = await fetch(this.url(`/api/runyx/conversations/${conversationId}/messages${qs}`));
    return asJSON<Message[]>(res);
  }

  async addMessage(conversationId: string, role: string, content: string): Promise<Message> {
    const res = await fetch(this.url(`/api/runyx/conversations/${conversationId}/messages`), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Role: role, Content: content }),
    });
    return asJSON<Message>(res);
  }

  // --- dashboard / permissions ---

  async dashboardStats(): Promise<DashboardStats> {
    const res = await fetch(this.url("/api/runyx/dashboard"));
    return asJSON<DashboardStats>(res);
  }

  async permissions(): Promise<PermissionsSnapshot> {
    const res = await fetch(this.url("/api/runyx/permissions"));
    return asJSON<PermissionsSnapshot>(res);
  }

  async revokeDevice(id: string): Promise<void> {
    const res = await fetch(this.url(`/api/runyx/permissions/devices/${id}`), { method: "DELETE" });
    if (!res.ok) throw new ApiError(res.status, await res.text().catch(() => `HTTP ${res.status}`));
  }

  async pairingQRCode(deviceName: string): Promise<PairingQR> {
    const res = await fetch(this.url("/api/runyx/pairing/qrcode-matrix"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ DeviceName: deviceName }),
    });
    return asJSON<PairingQR>(res);
  }

  // --- pending approvals (shell commands today; Runyx Code's tools later) ---

  async listApprovals(): Promise<ApprovalRequest[]> {
    const res = await fetch(this.url("/api/runyx/approvals"));
    return asJSON<ApprovalRequest[]>(res);
  }

  /** hash must match the request's current `hash` field exactly, or the daemon refuses the decision. */
  async decideApproval(id: string, hash: string, allow: boolean): Promise<void> {
    const res = await fetch(this.url(`/api/runyx/approvals/${id}/decide`), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ Hash: hash, Allow: allow }),
    });
    if (!res.ok) throw new ApiError(res.status, await res.text().catch(() => `HTTP ${res.status}`));
  }

  // --- streaming chat (OpenAI-compatible proxy) ---

  /**
   * Streams a chat completion, calling onDelta for each content chunk.
   * Resolves once the server sends "data: [DONE]" or closes the
   * connection — never on its own if the stream simply never ends;
   * callers that need a stall guard pass `signal` and watch for it
   * externally (see Chat screen), the same contract as the web UI's
   * streamChat.
   */
  async streamChat(
    model: string,
    messages: ChatMessageIn[],
    onDelta: (text: string) => void,
    signal?: AbortSignal,
    opts?: { maxTokens?: number; temperature?: number }
  ): Promise<void> {
    const res = await fetch(this.url("/v1/chat/completions"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        model,
        messages,
        stream: true,
        ...(opts?.maxTokens !== undefined ? { max_tokens: opts.maxTokens } : {}),
        ...(opts?.temperature !== undefined ? { temperature: opts.temperature } : {}),
      }),
      ...(signal ? { signal } : {}),
    });
    if (!res.ok || !res.body) {
      throw new ApiError(res.status, `chat request failed: HTTP ${res.status}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop() ?? "";
      for (const line of lines) {
        if (!line.startsWith("data: ")) continue;
        const data = line.slice(6);
        if (data === "[DONE]") return;
        try {
          const chunk = JSON.parse(data);
          const delta = chunk.choices?.[0]?.delta?.content;
          if (delta) onDelta(delta);
        } catch {
          // ignore malformed keep-alive lines
        }
      }
    }
  }
}
