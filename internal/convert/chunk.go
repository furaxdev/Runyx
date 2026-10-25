package convert

import "strings"

// Chunk splits text into overlapping, roughly token-sized pieces so that
// content exceeding a model's context window can be retrieved
// selectively (internal/rag, item 16) instead of truncated. Splits occur
// on paragraph boundaries where possible to avoid cutting sentences.
//
// maxChars approximates a token budget (≈4 characters/token for most
// tokenizers); overlapChars repeats a little context between chunks to
// avoid separating a reference from its antecedent.
func Chunk(text string, maxChars, overlapChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = 2000
	}
	if overlapChars < 0 || overlapChars >= maxChars {
		overlapChars = 0
	}
	if len(text) <= maxChars {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
		}
	}

	for _, p := range paragraphs {
		if cur.Len()+len(p)+2 > maxChars && cur.Len() > 0 {
			flush()
			tail := lastNChars(cur.String(), overlapChars)
			cur.Reset()
			cur.WriteString(tail)
		}
		if len(p) > maxChars {
			// A single paragraph bigger than the budget: hard-split it.
			for len(p) > maxChars {
				chunks = append(chunks, p[:maxChars])
				p = p[maxChars-overlapChars:]
			}
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	flush()
	return chunks
}

func lastNChars(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
