// Finds a running `runyx serve`, or starts one: there is no existing
// "is it running, what port" mechanism in the Go daemon beyond the
// configured listen_addr (checked cmd_serve.go and internal/config), so
// this probes GET /healthz on the configured address and, if that
// fails, spawns `runyx serve` itself (detached) and polls until healthy
// or a timeout — rather than Runyx gaining a new lockfile/PID-discovery
// protocol for this. See docs/DECISIONS.md.
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { baseUrlFor, readListenAddr } from "./config.js";

export class DaemonUnavailableError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "DaemonUnavailableError";
  }
}

async function probeHealthz(baseUrl: string, timeoutMs = 1500): Promise<boolean> {
  try {
    const res = await fetch(`${baseUrl}/healthz`, { signal: AbortSignal.timeout(timeoutMs) });
    return res.ok;
  } catch {
    return false;
  }
}

/** Looks for a `runyx` binary next to the running TUI executable, then on $PATH. */
function findRunyxBinary(): string | null {
  const exeName = process.platform === "win32" ? "runyx.exe" : "runyx";
  const besideExe = join(dirname(process.execPath), exeName);
  if (existsSync(besideExe)) return besideExe;

  const pathDirs = (process.env.PATH ?? "").split(process.platform === "win32" ? ";" : ":");
  for (const dir of pathDirs) {
    const candidate = join(dir, exeName);
    if (existsSync(candidate)) return candidate;
  }
  return null;
}

function spawnDaemon(binary: string): void {
  const child = spawn(binary, ["serve"], {
    detached: true,
    stdio: "ignore",
  });
  child.unref();
}

export interface EnsureDaemonOptions {
  /** Total time to wait for /healthz to go green, including a just-spawned daemon's startup. */
  timeoutMs?: number;
  pollIntervalMs?: number;
}

/**
 * Resolves the daemon's base URL, starting it if it isn't already
 * running. Throws DaemonUnavailableError if no `runyx` binary can be
 * found at all, or if it never becomes healthy within the timeout.
 */
export async function ensureDaemon(opts: EnsureDaemonOptions = {}): Promise<string> {
  const timeoutMs = opts.timeoutMs ?? 15000;
  const pollIntervalMs = opts.pollIntervalMs ?? 300;
  const baseUrl = baseUrlFor(readListenAddr());

  if (await probeHealthz(baseUrl)) return baseUrl;

  const binary = findRunyxBinary();
  if (!binary) {
    throw new DaemonUnavailableError(
      "runyx is not running and no `runyx` binary was found next to this program or on $PATH. Start `runyx serve` manually."
    );
  }
  spawnDaemon(binary);

  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await probeHealthz(baseUrl)) return baseUrl;
    await new Promise((r) => setTimeout(r, pollIntervalMs));
  }
  throw new DaemonUnavailableError(`runyx serve did not become healthy within ${timeoutMs}ms`);
}
