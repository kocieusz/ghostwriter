// Package facts checks that a draft keeps the facts of its source material
// and invents none. A voice rewrite may reshape every sentence, but a number,
// name or quotation that is not in the brief is a fabrication, and one that
// silently disappears is a lost claim.
package facts

import (
	"regexp"
	"sort"
	"strings"

	"github.com/kocieusz/ghostwriter/internal/stylo"
)

var (
	numberRe = regexp.MustCompile(`\d(?:[\d,.:/]*\d)?%?`)
	quoteRe  = regexp.MustCompile(`"([^"\n]{12,300})"`)
	nameRe   = regexp.MustCompile(`\p{Lu}[\p{L}'-]+(?:\s+\p{Lu}[\p{L}'-]+)*`)
)

// Report lists what the draft added or dropped relative to the source.
type Report struct {
	AddedNumbers   []string `json:"added_numbers,omitempty"`
	AddedNames     []string `json:"added_names,omitempty"`
	AddedQuotes    []string `json:"added_quotes,omitempty"`
	DroppedNumbers []string `json:"dropped_numbers,omitempty"`
	DroppedNames   []string `json:"dropped_names,omitempty"`
}

// Fabricated reports whether the draft invents a number or quotation, the two
// additions that are never a matter of style.
func (r Report) Fabricated() bool {
	return len(r.AddedNumbers) > 0 || len(r.AddedQuotes) > 0
}

// Penalty converts the report to points off the overall score.
func (r Report) Penalty() float64 {
	p := 5*float64(len(r.AddedNumbers)+len(r.AddedQuotes)) +
		3*float64(len(r.AddedNames)) +
		2*float64(len(r.DroppedNumbers)) +
		1*float64(len(r.DroppedNames))
	return min(p, 30)
}

// Compare checks draft against source.
func Compare(source, draft string) Report {
	source, draft = prep(source), prep(draft)
	srcLower := strings.ToLower(source)
	srcNums, dNums := numbers(source), numbers(draft)
	srcNames, dNames := names(source), names(draft)

	var r Report
	r.AddedNumbers = minus(dNums, srcNums)
	r.DroppedNumbers = minus(srcNums, dNums)
	for _, n := range sortedKeys(dNames) {
		// a name is only new if none of its words appears in the source at
		// all, so "Smith" after "Dr. Anna Smith" is not an addition
		if !anyWordIn(n, srcLower) {
			r.AddedNames = append(r.AddedNames, n)
		}
	}
	draftLower := strings.ToLower(draft)
	for _, n := range sortedKeys(srcNames) {
		if !anyWordIn(n, draftLower) {
			r.DroppedNames = append(r.DroppedNames, n)
		}
	}
	for _, m := range quoteRe.FindAllStringSubmatch(draft, -1) {
		if !strings.Contains(srcLower, strings.ToLower(strings.TrimSpace(m[1]))) {
			r.AddedQuotes = append(r.AddedQuotes, m[1])
		}
	}
	return r
}

func prep(s string) string { return stylo.Mask(stylo.Normalize(s)) }

func numbers(s string) map[string]bool {
	out := map[string]bool{}
	for _, m := range numberRe.FindAllString(s, -1) {
		m = strings.TrimRight(strings.ReplaceAll(m, ",", ""), ".")
		if m != "" {
			out[m] = true
		}
	}
	return out
}

// names collects capitalised words and runs that do not open a sentence or a
// line, skipping the pronoun I and a few words that are routinely capitalised.
func names(s string) map[string]bool {
	out := map[string]bool{}
	for _, sent := range stylo.Sentences(s) {
		first := true
		for _, loc := range nameRe.FindAllStringIndex(sent, -1) {
			n := sent[loc[0]:loc[1]]
			atStart := strings.TrimLeft(sent[:loc[0]], ` "'(*-#>0123456789.)`) == ""
			if first && atStart {
				// drop the sentence-initial word, keep the rest of the run
				parts := strings.Fields(n)
				n = strings.Join(parts[1:], " ")
			}
			first = false
			if n == "" || commonCaps[n] {
				continue
			}
			out[n] = true
		}
	}
	return out
}

var commonCaps = map[string]bool{
	"I": true, "I'm": true, "I've": true, "I'd": true, "I'll": true, "OK": true,
	"Monday": true, "Tuesday": true, "Wednesday": true, "Thursday": true, "Friday": true, "Saturday": true, "Sunday": true,
}

func anyWordIn(name, lowerText string) bool {
	for w := range strings.FieldsSeq(strings.ToLower(name)) {
		if len(w) > 2 && containsWord(lowerText, w) {
			return true
		}
	}
	return false
}

func containsWord(text, w string) bool {
	re := regexp.MustCompile(`(?:^|[^\p{L}])` + regexp.QuoteMeta(w) + `(?:[^\p{L}]|$)`)
	return re.MatchString(text)
}

func minus(a, b map[string]bool) []string {
	var out []string
	for _, k := range sortedKeys(a) {
		if !b[k] {
			out = append(out, k)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
