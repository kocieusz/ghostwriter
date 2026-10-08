// Package stylo turns text into the stylometric feature vector that a writer's
// profile is built from and drafts are scored against.
package stylo

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	wordRe     = regexp.MustCompile(`\p{L}+(?:'\p{L}+)?`)
	paraSplit  = regexp.MustCompile(`\n[ \t]*\n`)
	softBreak  = regexp.MustCompile(`[ \t]*\n[ \t]*`)
	fenceRe    = regexp.MustCompile("(?s)```.*?```|~~~.*?~~~")
	inlineCode = regexp.MustCompile("`[^`\n]+`")
	urlRe      = regexp.MustCompile(`https?://\S+`)
)

var quoteFolder = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`)

// Normalize folds typographic quotes and apostrophes to ASCII so contractions
// and phrases match whichever the writer's editor produced.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return quoteFolder.Replace(s)
}

// Mask blanks out code and URLs with spaces, keeping byte offsets (and so line
// numbers) intact. Code is never prose, so it must not count toward a voice or
// trip an AI tell.
func Mask(s string) string { return Blank(s, fenceRe, inlineCode, urlRe) }

// MaskCode blanks out code only, leaving URLs for checks that inspect them.
func MaskCode(s string) string { return Blank(s, fenceRe, inlineCode) }

// Blank replaces every match of each pattern with spaces, keeping newlines.
func Blank(s string, res ...*regexp.Regexp) string {
	b := []byte(s)
	for _, re := range res {
		for _, loc := range re.FindAllIndex(b, -1) {
			for i := loc[0]; i < loc[1]; i++ {
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
		}
	}
	return string(b)
}

// Words returns the word tokens of s. Letters are Unicode-aware, so non-English
// writers still get sentence and paragraph statistics even though the phrase
// lists are English.
func Words(s string) []string {
	return wordRe.FindAllString(s, -1)
}

// Lower returns the lowercased word tokens of s.
func Lower(s string) []string {
	ws := Words(s)
	for i, w := range ws {
		ws[i] = strings.ToLower(w)
	}
	return ws
}

// Paragraphs splits on blank lines.
func Paragraphs(s string) []string {
	var out []string
	for _, p := range paraSplit.Split(s, -1) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Sentences splits every paragraph into sentences, unwrapping soft line breaks
// first. A sentence ends at a run of . ! or ? (plus any closing quotes or
// brackets) followed by whitespace.
func Sentences(s string) []string {
	var out []string
	for _, p := range Paragraphs(s) {
		out = append(out, SplitSentences(softBreak.ReplaceAllString(p, " "))...)
	}
	return out
}

// SplitSentences splits a single line of prose into sentences.
func SplitSentences(p string) []string {
	var out []string
	rs := []rune(p)
	start := 0
	for i := 0; i < len(rs); i++ {
		if rs[i] != '.' && rs[i] != '!' && rs[i] != '?' {
			continue
		}
		j := i + 1
		for j < len(rs) && strings.ContainsRune(`.!?"')]`, rs[j]) {
			j++
		}
		if j < len(rs) && !unicode.IsSpace(rs[j]) {
			i = j - 1
			continue
		}
		if s := strings.TrimSpace(string(rs[start:j])); s != "" {
			out = append(out, s)
		}
		start = j
		i = j - 1
	}
	if s := strings.TrimSpace(string(rs[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

// CountPhrases counts whole-token occurrences of each phrase in toks (already
// lowercased). Matching on tokens rather than substrings keeps "i think" from
// matching inside "hi think" and "so" from matching inside "also".
func CountPhrases(toks []string, phrases []string) int {
	total := 0
	for _, p := range phrases {
		pt := strings.Fields(p)
		if len(pt) == 0 {
			continue
		}
	scan:
		for i := 0; i+len(pt) <= len(toks); i++ {
			for k, w := range pt {
				if toks[i+k] != w {
					continue scan
				}
			}
			total++
		}
	}
	return total
}

// StartsWithAny reports whether the sentence opens with one of the phrases.
func StartsWithAny(sentence string, phrases []string) bool {
	toks := Lower(sentence)
	for _, p := range phrases {
		pt := strings.Fields(p)
		if len(pt) <= len(toks) && equalTokens(toks[:len(pt)], pt) {
			return true
		}
	}
	return false
}

func equalTokens(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FirstWord returns the lowercased first word token of s, or "".
func FirstWord(s string) string {
	if w := wordRe.FindString(s); w != "" {
		return strings.ToLower(w)
	}
	return ""
}
