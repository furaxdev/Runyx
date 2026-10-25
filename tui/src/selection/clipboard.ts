// Clipboard integration: OSC 52 (works over SSH and inside tmux, no
// external process needed) plus a native CLI fallback (pbcopy / wl-copy
// / xclip / xsel / clip.exe), since not every terminal honors OSC 52 and
// a user running locally still expects Cmd/Ctrl+V to work afterward.
import { spawn } from "node:child_process";
import { osc52Copy } from "../term/ansi.js";

export type ClipboardMethod = "osc52" | "cli" | "none";

/**
 * Writes `text` to the system clipboard via OSC 52 (always attempted —
 * harmless if unsupported, since an unrecognized OSC sequence is simply
 * ignored by the terminal) and, if a known CLI clipboard tool is found
 * on PATH, also via that tool. Returns what was attempted, not a
 * guarantee of success: OSC 52's actual effect can't be confirmed
 * without a terminal reply, which most terminals don't send.
 */
export async function copyToClipboard(text: string, output: NodeJS.WriteStream = process.stdout): Promise<ClipboardMethod> {
  let method: ClipboardMethod = "none";
  if (output.isTTY) {
    output.write(wrapForTmux(osc52Copy(toBase64(text))));
    method = "osc52";
  }
  const cliOk = await tryCliCopy(text);
  if (cliOk) method = "cli";
  return method;
}

export function toBase64(text: string): string {
  return Buffer.from(text, "utf8").toString("base64");
}

/** Inside tmux, OSC sequences must be wrapped in a tmux DCS passthrough or tmux swallows them. */
export function wrapForTmux(osc: string): string {
  if (!process.env.TMUX) return osc;
  const escaped = osc.replace(/\x1b/g, "\x1b\x1b");
  return `\x1bPtmux;${escaped}\x1b\\`;
}

interface Tool {
  cmd: string;
  args: string[];
}

function candidateTools(): Tool[] {
  const plat = process.platform;
  if (plat === "darwin") return [{ cmd: "pbcopy", args: [] }];
  if (plat === "win32") return [{ cmd: "clip", args: [] }];
  // Linux/BSD: prefer Wayland, then X11 variants.
  return [
    { cmd: "wl-copy", args: [] },
    { cmd: "xclip", args: ["-selection", "clipboard"] },
    { cmd: "xsel", args: ["--clipboard", "--input"] },
    { cmd: "clip.exe", args: [] }, // WSL
  ];
}

async function tryCliCopy(text: string): Promise<boolean> {
  for (const tool of candidateTools()) {
    const ok = await runWithStdin(tool.cmd, tool.args, text);
    if (ok) return true;
  }
  return false;
}

function runWithStdin(cmd: string, args: string[], input: string): Promise<boolean> {
  return new Promise((resolve) => {
    let child;
    try {
      child = spawn(cmd, args, { stdio: ["pipe", "ignore", "ignore"] });
    } catch {
      resolve(false);
      return;
    }
    child.on("error", () => resolve(false));
    child.on("exit", (code) => resolve(code === 0));
    child.stdin.on("error", () => {
      /* EPIPE if the tool isn't actually usable; "exit"/"error" above settles the promise */
    });
    child.stdin.end(input);
  });
}
