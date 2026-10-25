// Markdown AST types. Every node carries [start, end) byte offsets into
// the *original* source text it was parsed from — this is what makes
// selection "content-anchored": a selection range is stored as
// (messageId, startOffset, endOffset) into that raw source, never as
// screen coordinates, so it stays correct across scrolling and
// continued streaming (new text only ever appends, never shifts
// existing offsets).

export interface SourceSpan {
  start: number;
  end: number;
}

export type InlineNode =
  | ({ kind: "text"; text: string } & SourceSpan)
  | ({ kind: "bold"; children: InlineNode[] } & SourceSpan)
  | ({ kind: "italic"; children: InlineNode[] } & SourceSpan)
  | ({ kind: "boldItalic"; children: InlineNode[] } & SourceSpan)
  | ({ kind: "strike"; children: InlineNode[] } & SourceSpan)
  | ({ kind: "code"; text: string } & SourceSpan)
  | ({ kind: "link"; text: string; url: string } & SourceSpan);

export type BlockNode =
  | ({ kind: "heading"; level: 1 | 2 | 3 | 4 | 5 | 6; children: InlineNode[] } & SourceSpan)
  | ({ kind: "paragraph"; children: InlineNode[] } & SourceSpan)
  | ({ kind: "list"; ordered: boolean; items: ListItem[] } & SourceSpan)
  | ({ kind: "blockquote"; children: BlockNode[] } & SourceSpan)
  | ({ kind: "codeBlock"; lang: string; code: string; closed: boolean } & SourceSpan)
  | ({ kind: "table"; header: InlineNode[][]; align: ("left" | "center" | "right")[]; rows: InlineNode[][][] } & SourceSpan)
  | ({ kind: "rule" } & SourceSpan)
  | ({ kind: "blank" } & SourceSpan);

export interface ListItem extends SourceSpan {
  children: BlockNode[];
  checked?: boolean | undefined; // undefined = not a task item
  depth: number;
}
