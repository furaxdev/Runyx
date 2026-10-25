// Resolves the daemon's listen address the same way the Go daemon
// itself does (internal/config.Default/Dir/Path): an XDG-style
// per-platform config directory, holding a flat config.toml. No TOML
// dependency is pulled in for this — config.toml as written by Runyx
// (see internal/config) is always a flat table of `key = value` lines,
// so a few regex lines cover it without a parser library, consistent
// with the TUI's zero-runtime-dependency goal.
import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

export const DEFAULT_LISTEN_ADDR = "127.0.0.1:11535";

/** Mirrors Go's os.UserConfigDir() for the three platforms Runyx ships on. */
export function userConfigDir(): string {
  const plat = process.platform;
  if (plat === "darwin") return join(homedir(), "Library", "Application Support");
  if (plat === "win32") return process.env.AppData ?? join(homedir(), "AppData", "Roaming");
  return process.env.XDG_CONFIG_HOME || join(homedir(), ".config");
}

export function configPath(): string {
  return join(userConfigDir(), "runyx", "config.toml");
}

/** Reads `listen_addr` out of config.toml, falling back to Runyx's own default if the file or key is missing. */
export function readListenAddr(): string {
  const path = configPath();
  if (!existsSync(path)) return DEFAULT_LISTEN_ADDR;
  try {
    const text = readFileSync(path, "utf8");
    const match = text.match(/^\s*listen_addr\s*=\s*"([^"]*)"\s*$/m);
    return match?.[1] || DEFAULT_LISTEN_ADDR;
  } catch {
    return DEFAULT_LISTEN_ADDR;
  }
}

export function baseUrlFor(listenAddr: string): string {
  // config.toml may bind "0.0.0.0:PORT" for a non-default deployment;
  // that's not dialable as-is, so talk to loopback on the same port.
  const port = listenAddr.split(":").pop();
  return `http://127.0.0.1:${port}`;
}
