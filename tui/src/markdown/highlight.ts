// Lightweight syntax highlighting: a small set of regex rules per
// language, applied line by line. Not a real tokenizer/parser (no
// language grammar, no bracket matching) — good enough to color
// keywords/strings/comments/numbers in a chat code block, not a claim
// of editor-grade correctness.
export type TokenKind = "keyword" | "string" | "comment" | "number" | "plain" | "function" | "type";

export interface Token {
  text: string;
  kind: TokenKind;
}

interface Rule {
  re: RegExp;
  kind: TokenKind;
}

const KEYWORDS: Record<string, string[]> = {
  js: ["const", "let", "var", "function", "return", "if", "else", "for", "while", "class", "extends", "import", "export", "from", "default", "async", "await", "try", "catch", "finally", "throw", "new", "typeof", "instanceof", "yield", "switch", "case", "break", "continue", "null", "undefined", "true", "false", "this", "super"],
  ts: [],
  python: ["def", "return", "if", "elif", "else", "for", "while", "class", "import", "from", "as", "try", "except", "finally", "raise", "with", "lambda", "yield", "async", "await", "pass", "break", "continue", "None", "True", "False", "self", "not", "and", "or", "in", "is"],
  go: ["func", "return", "if", "else", "for", "range", "package", "import", "var", "const", "type", "struct", "interface", "map", "chan", "go", "defer", "select", "switch", "case", "break", "continue", "nil", "true", "false", "const", "iota"],
  rust: ["fn", "let", "mut", "return", "if", "else", "for", "while", "loop", "match", "struct", "enum", "impl", "trait", "use", "mod", "pub", "const", "static", "async", "await", "move", "ref", "self", "Self", "true", "false", "None", "Some", "Ok", "Err"],
  sh: ["if", "then", "else", "elif", "fi", "for", "while", "do", "done", "case", "esac", "function", "return", "local", "export", "in", "echo", "set", "exit"],
  bash: [],
  json: [],
  yaml: [],
  diff: [],
};
KEYWORDS.ts = [...KEYWORDS.js!, "interface", "type", "enum", "implements", "public", "private", "protected", "readonly", "as", "namespace", "declare"];
KEYWORDS.bash = KEYWORDS.sh!;

function rulesFor(lang: string): Rule[] {
  const common: Rule[] = [
    { re: /"(?:[^"\\]|\\.)*"/, kind: "string" },
    { re: /'(?:[^'\\]|\\.)*'/, kind: "string" },
    { re: /\b\d+(\.\d+)?\b/, kind: "number" },
  ];
  switch (lang) {
    case "js":
    case "ts":
    case "javascript":
    case "typescript":
      return [{ re: /`(?:[^`\\]|\\.)*`/, kind: "string" }, { re: /\/\/.*$/, kind: "comment" }, ...common];
    case "python":
    case "py":
      return [{ re: /#.*$/, kind: "comment" }, ...common];
    case "go":
    case "rust":
    case "rs":
      return [{ re: /\/\/.*$/, kind: "comment" }, { re: /`[^`]*`/, kind: "string" }, ...common];
    case "sh":
    case "bash":
      return [{ re: /#.*$/, kind: "comment" }, ...common];
    case "json":
      return [{ re: /"(?:[^"\\]|\\.)*"\s*:/, kind: "type" }, ...common];
    case "yaml":
    case "yml":
      return [{ re: /#.*$/, kind: "comment" }, { re: /^[\w-]+(?=:)/, kind: "type" }, ...common];
    case "diff":
      return [];
    default:
      return common;
  }
}

function normalizeLang(lang: string): string {
  const l = lang.toLowerCase().trim();
  if (l === "javascript") return "js";
  if (l === "typescript" || l === "tsx" || l === "jsx") return "ts";
  if (l === "py") return "python";
  if (l === "rs") return "rust";
  if (l === "yml") return "yaml";
  if (l === "shell" || l === "zsh") return "sh";
  return l;
}

/** Tokenizes one source line for `lang` into colored spans. Unknown languages get numbers+quoted-string highlighting only. */
export function highlightLine(line: string, lang: string): Token[] {
  const normalized = normalizeLang(lang);
  if (normalized === "diff") return highlightDiffLine(line);
  const keywords = new Set(KEYWORDS[normalized] ?? []);
  const rules = rulesFor(normalized);
  const tokens: Token[] = [];
  let i = 0;
  while (i < line.length) {
    // Try word-boundary keyword match first.
    const wordMatch = /^[A-Za-z_$][A-Za-z0-9_$]*/.exec(line.slice(i));
    if (wordMatch && keywords.has(wordMatch[0])) {
      tokens.push({ text: wordMatch[0], kind: "keyword" });
      i += wordMatch[0].length;
      continue;
    }
    let matched = false;
    for (const rule of rules) {
      const re = new RegExp(rule.re.source, rule.re.flags.includes("g") ? rule.re.flags : rule.re.flags + "");
      re.lastIndex = 0;
      const sub = line.slice(i);
      const m = new RegExp("^(?:" + rule.re.source + ")").exec(sub);
      if (m && m[0].length > 0) {
        tokens.push({ text: m[0], kind: rule.kind });
        i += m[0].length;
        matched = true;
        break;
      }
    }
    if (matched) continue;
    if (wordMatch) {
      tokens.push({ text: wordMatch[0], kind: "plain" });
      i += wordMatch[0].length;
      continue;
    }
    // Single plain character.
    const last = tokens[tokens.length - 1];
    if (last && last.kind === "plain") last.text += line[i];
    else tokens.push({ text: line[i]!, kind: "plain" });
    i++;
  }
  return tokens;
}

function highlightDiffLine(line: string): Token[] {
  if (line.startsWith("+")) return [{ text: line, kind: "string" }]; // green-ish via string color in theme mapping
  if (line.startsWith("-")) return [{ text: line, kind: "keyword" }]; // red-ish
  if (line.startsWith("@@")) return [{ text: line, kind: "comment" }];
  return [{ text: line, kind: "plain" }];
}
