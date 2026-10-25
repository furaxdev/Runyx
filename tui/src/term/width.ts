// Terminal-cell width for a single Unicode code point: 0 (combining /
// zero-width), 1 (narrow) or 2 (wide, East-Asian "Wide"/"Fullwidth" or
// emoji-presentation). This is a compact, hand-written approximation of
// the ranges published in Unicode's EastAsianWidth.txt and the
// combining-class General_Category values — not an exhaustive port of
// any particular library, and not a claim of 100% conformance for every
// code point in Unicode. Good enough for terminal layout; CJK, Hangul,
// combining accents and common emoji are covered, obscure scripts may
// default to width 1 where a terminal would actually render 2 (or vice
// versa) — flagged as a known gap in docs/DECISIONS.md.
//
// Grapheme *clustering* (so "e" + combining acute renders as one cell,
// a flag emoji's two regional indicators as one, etc.) is handled
// separately via Intl.Segmenter in graphemes() below — width.ts only
// scores a single code point once a cluster's primary code point is
// known.

interface Range {
  lo: number;
  hi: number;
}

function inRanges(cp: number, ranges: readonly Range[]): boolean {
  let lo = 0;
  let hi = ranges.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    const r = ranges[mid]!;
    if (cp < r.lo) hi = mid - 1;
    else if (cp > r.hi) lo = mid + 1;
    else return true;
  }
  return false;
}

// Zero-width: combining marks, formatting chars, variation selectors,
// zero-width space/joiner/non-joiner, default-ignorable controls.
const ZERO_WIDTH: readonly Range[] = [
  { lo: 0x0000, hi: 0x001f }, // C0 controls
  { lo: 0x007f, hi: 0x009f }, // DEL + C1 controls
  { lo: 0x0300, hi: 0x036f }, // combining diacritical marks
  { lo: 0x0483, hi: 0x0489 }, // Cyrillic combining
  { lo: 0x0591, hi: 0x05bd }, // Hebrew points
  { lo: 0x05bf, hi: 0x05bf },
  { lo: 0x05c1, hi: 0x05c2 },
  { lo: 0x05c4, hi: 0x05c5 },
  { lo: 0x05c7, hi: 0x05c7 },
  { lo: 0x0610, hi: 0x061a }, // Arabic marks
  { lo: 0x064b, hi: 0x065f },
  { lo: 0x0670, hi: 0x0670 },
  { lo: 0x06d6, hi: 0x06dc },
  { lo: 0x06df, hi: 0x06e4 },
  { lo: 0x06e7, hi: 0x06e8 },
  { lo: 0x06ea, hi: 0x06ed },
  { lo: 0x0711, hi: 0x0711 },
  { lo: 0x0730, hi: 0x074a },
  { lo: 0x07a6, hi: 0x07b0 },
  { lo: 0x0816, hi: 0x0819 },
  { lo: 0x081b, hi: 0x0823 },
  { lo: 0x0825, hi: 0x0827 },
  { lo: 0x0829, hi: 0x082d },
  { lo: 0x0859, hi: 0x085b },
  { lo: 0x08e3, hi: 0x0903 },
  { lo: 0x093a, hi: 0x093c },
  { lo: 0x093e, hi: 0x094f },
  { lo: 0x0951, hi: 0x0957 },
  { lo: 0x0962, hi: 0x0963 },
  { lo: 0x1ab0, hi: 0x1aff }, // combining diacritical marks extended
  { lo: 0x1dc0, hi: 0x1dff }, // combining diacritical marks supplement
  { lo: 0x200b, hi: 0x200f }, // ZWSP, ZWJ, ZWNJ, LRM/RLM
  { lo: 0x202a, hi: 0x202e }, // directional formatting
  { lo: 0x2060, hi: 0x2064 }, // word joiner, invisible operators
  { lo: 0x2066, hi: 0x206f },
  { lo: 0x20d0, hi: 0x20ff }, // combining marks for symbols
  { lo: 0xfe00, hi: 0xfe0f }, // variation selectors
  { lo: 0xfe20, hi: 0xfe2f }, // combining half marks
  { lo: 0xfeff, hi: 0xfeff }, // BOM / ZWNBSP
  { lo: 0x1d167, hi: 0x1d169 },
  { lo: 0x1d173, hi: 0x1d182 },
  { lo: 0x1d185, hi: 0x1d18b },
  { lo: 0x1d1aa, hi: 0x1d1ad },
  { lo: 0xe0100, hi: 0xe01ef }, // variation selectors supplement
];

// Wide (2 cells): East-Asian Wide/Fullwidth blocks + common emoji.
const WIDE: readonly Range[] = [
  { lo: 0x1100, hi: 0x115f }, // Hangul Jamo
  { lo: 0x231a, hi: 0x231b }, // watch, hourglass
  { lo: 0x2329, hi: 0x232a },
  { lo: 0x23e9, hi: 0x23ec },
  { lo: 0x23f0, hi: 0x23f0 },
  { lo: 0x23f3, hi: 0x23f3 },
  { lo: 0x25fd, hi: 0x25fe },
  { lo: 0x2614, hi: 0x2615 },
  { lo: 0x2648, hi: 0x2653 },
  { lo: 0x267f, hi: 0x267f },
  { lo: 0x2693, hi: 0x2693 },
  { lo: 0x26a1, hi: 0x26a1 },
  { lo: 0x26aa, hi: 0x26ab },
  { lo: 0x26bd, hi: 0x26be },
  { lo: 0x26c4, hi: 0x26c5 },
  { lo: 0x26ce, hi: 0x26ce },
  { lo: 0x26d4, hi: 0x26d4 },
  { lo: 0x26ea, hi: 0x26ea },
  { lo: 0x26f2, hi: 0x26f3 },
  { lo: 0x26f5, hi: 0x26f5 },
  { lo: 0x26fa, hi: 0x26fa },
  { lo: 0x26fd, hi: 0x26fd },
  { lo: 0x2705, hi: 0x2705 },
  { lo: 0x270a, hi: 0x270b },
  { lo: 0x2728, hi: 0x2728 },
  { lo: 0x274c, hi: 0x274c },
  { lo: 0x274e, hi: 0x274e },
  { lo: 0x2753, hi: 0x2755 },
  { lo: 0x2757, hi: 0x2757 },
  { lo: 0x2795, hi: 0x2797 },
  { lo: 0x27b0, hi: 0x27b0 },
  { lo: 0x27bf, hi: 0x27bf },
  { lo: 0x2b1b, hi: 0x2b1c },
  { lo: 0x2b50, hi: 0x2b50 },
  { lo: 0x2b55, hi: 0x2b55 },
  { lo: 0x2e80, hi: 0x2fdf }, // CJK Radicals, Kangxi Radicals
  { lo: 0x2ff0, hi: 0x303e }, // CJK ideographic description, CJK symbols/punctuation
  { lo: 0x3041, hi: 0x33ff }, // Hiragana..CJK Compat
  { lo: 0x3400, hi: 0x4dbf }, // CJK Extension A
  { lo: 0x4e00, hi: 0x9fff }, // CJK Unified Ideographs
  { lo: 0xa000, hi: 0xa4cf }, // Yi
  { lo: 0xac00, hi: 0xd7a3 }, // Hangul Syllables
  { lo: 0xf900, hi: 0xfaff }, // CJK Compatibility Ideographs
  { lo: 0xfe30, hi: 0xfe4f }, // CJK Compatibility Forms
  { lo: 0xff00, hi: 0xff60 }, // Fullwidth Forms
  { lo: 0xffe0, hi: 0xffe6 },
  { lo: 0x16fe0, hi: 0x16fe4 },
  { lo: 0x17000, hi: 0x18d08 }, // Tangut
  { lo: 0x1aff0, hi: 0x1b2fb }, // Kana extended
  { lo: 0x1f004, hi: 0x1f004 },
  { lo: 0x1f0cf, hi: 0x1f0cf },
  { lo: 0x1f18e, hi: 0x1f18e },
  { lo: 0x1f191, hi: 0x1f19a },
  { lo: 0x1f200, hi: 0x1f320 }, // squared CJK, enclosed ideographic, misc symbols
  { lo: 0x1f32d, hi: 0x1f335 },
  { lo: 0x1f337, hi: 0x1f37c },
  { lo: 0x1f37e, hi: 0x1f393 },
  { lo: 0x1f3a0, hi: 0x1f3ca },
  { lo: 0x1f3cf, hi: 0x1f3d3 },
  { lo: 0x1f3e0, hi: 0x1f3f0 },
  { lo: 0x1f3f4, hi: 0x1f3f4 },
  { lo: 0x1f3f8, hi: 0x1f43e },
  { lo: 0x1f440, hi: 0x1f440 },
  { lo: 0x1f442, hi: 0x1f4fc },
  { lo: 0x1f4ff, hi: 0x1f53d },
  { lo: 0x1f54b, hi: 0x1f54e },
  { lo: 0x1f550, hi: 0x1f567 },
  { lo: 0x1f57a, hi: 0x1f57a },
  { lo: 0x1f595, hi: 0x1f596 },
  { lo: 0x1f5a4, hi: 0x1f5a4 },
  { lo: 0x1f5fb, hi: 0x1f64f }, // emoticons
  { lo: 0x1f680, hi: 0x1f6c5 }, // transport
  { lo: 0x1f6cc, hi: 0x1f6cc },
  { lo: 0x1f6d0, hi: 0x1f6d2 },
  { lo: 0x1f6d5, hi: 0x1f6d7 },
  { lo: 0x1f6dc, hi: 0x1f6df },
  { lo: 0x1f6eb, hi: 0x1f6ec },
  { lo: 0x1f6f4, hi: 0x1f6fc },
  { lo: 0x1f7e0, hi: 0x1f7eb },
  { lo: 0x1f7f0, hi: 0x1f7f0 },
  { lo: 0x1f90c, hi: 0x1f93a },
  { lo: 0x1f93c, hi: 0x1f945 },
  { lo: 0x1f947, hi: 0x1f9ff }, // supplemental symbols/pictographs
  { lo: 0x1fa70, hi: 0x1faff }, // symbols and pictographs extended-A
  { lo: 0x20000, hi: 0x3fffd }, // CJK Extension B+ / Compatibility Supplement
];

/** Returns the terminal cell width (0, 1, or 2) of a single code point. */
export function codePointWidth(cp: number): 0 | 1 | 2 {
  if (cp === 0) return 0;
  if (inRanges(cp, ZERO_WIDTH)) return 0;
  if (inRanges(cp, WIDE)) return 2;
  return 1;
}

/** Sum of codePointWidth() over every code point in s. Ignores clustering. */
export function stringWidth(s: string): number {
  let w = 0;
  for (const ch of s) w += codePointWidth(ch.codePointAt(0) ?? 0);
  return w;
}

const segmenter: Intl.Segmenter | null =
  typeof Intl !== "undefined" && "Segmenter" in Intl ? new Intl.Segmenter(undefined, { granularity: "grapheme" }) : null;

/**
 * Splits s into user-perceived grapheme clusters (so combining accents,
 * ZWJ emoji sequences, regional-indicator flag pairs etc. stay single
 * terminal cells together), each annotated with its rendered width.
 */
export function graphemes(s: string): { text: string; width: 1 | 2 }[] {
  if (s === "") return [];
  const out: { text: string; width: 1 | 2 }[] = [];
  const iter = segmenter ? Array.from(segmenter.segment(s), (seg) => seg.segment) : splitWithoutSegmenter(s);
  for (const cluster of iter) {
    const w = clusterWidth(cluster);
    out.push({ text: cluster, width: w });
  }
  return out;
}

function clusterWidth(cluster: string): 1 | 2 {
  // A cluster's visible width is dominated by its first non-zero-width
  // code point (subsequent ones are combining marks / joiners / VS16).
  for (const ch of cluster) {
    const w = codePointWidth(ch.codePointAt(0) ?? 0);
    if (w === 2) return 2;
    if (w === 1) return 1;
  }
  return 1;
}

// Fallback for a runtime with no Intl.Segmenter (shouldn't happen on
// Bun or Node >=16, kept defensively): splits on code points only, so
// combining marks render as separate cells rather than crashing.
function splitWithoutSegmenter(s: string): string[] {
  return Array.from(s);
}
