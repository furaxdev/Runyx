import { describe, expect, test } from "bun:test";
import { mkdtempSync, writeFileSync, mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { baseUrlFor, configPath, DEFAULT_LISTEN_ADDR, readListenAddr } from "../src/api/config.js";

describe("baseUrlFor", () => {
  test("talks to loopback on the configured port, even for a 0.0.0.0 bind", () => {
    expect(baseUrlFor("127.0.0.1:11535")).toBe("http://127.0.0.1:11535");
    expect(baseUrlFor("0.0.0.0:9999")).toBe("http://127.0.0.1:9999");
  });
});

describe("readListenAddr", () => {
  test("falls back to the default when config.toml does not exist", () => {
    const original = process.env.XDG_CONFIG_HOME;
    process.env.XDG_CONFIG_HOME = mkdtempSync(join(tmpdir(), "runyx-test-"));
    try {
      expect(readListenAddr()).toBe(DEFAULT_LISTEN_ADDR);
    } finally {
      if (original === undefined) delete process.env.XDG_CONFIG_HOME;
      else process.env.XDG_CONFIG_HOME = original;
    }
  });

  test("reads listen_addr out of a real config.toml", () => {
    const original = process.env.XDG_CONFIG_HOME;
    const dir = mkdtempSync(join(tmpdir(), "runyx-test-"));
    process.env.XDG_CONFIG_HOME = dir;
    try {
      mkdirSync(join(dir, "runyx"), { recursive: true });
      writeFileSync(
        configPath(),
        ['listen_addr = "127.0.0.1:22222"', "telemetry = false", "idle_unload_seconds = 600", ""].join("\n")
      );
      expect(readListenAddr()).toBe("127.0.0.1:22222");
    } finally {
      if (original === undefined) delete process.env.XDG_CONFIG_HOME;
      else process.env.XDG_CONFIG_HOME = original;
    }
  });
});
