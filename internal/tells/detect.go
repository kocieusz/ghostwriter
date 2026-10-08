package tells

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/kocieusz/ghostwriter/internal/stylo"
)

// Hit is one sighting of a rule in a text.
type Hit struct {
	Rule  string `json:"rule"`
	Line  int    `json:"line"`
	Match string `json:"match"`
}

var (
	quotedRe     = regexp.MustCompile(`"[^"\n]{1,300}"`)
	blockquoteRe = regexp.MustCompile(`(?m)^[ \t]*>.*$`)
	headingRe    = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+)$`)
)

// Detect returns every hit in text, in document order.
func Detect(text string) []Hit {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	raw := stylo.MaskCode(text)
	prose := stylo.Blank(stylo.Normalize(stylo.Mask(text)), quotedRe, blockquoteRe)
	hits := []Hit{}
	for _, r := range Rules {
		src := prose
		if r.Raw {
			src = raw
		}
		var spans [][2]int
		if r.check != nil {
			spans = r.check(src)
		} else {
			for _, loc := range r.re.FindAllStringIndex(src, -1) {
				spans = append(spans, [2]int{loc[0], loc[1]})
			}
		}
		for _, sp := range spans {
			hits = append(hits, Hit{
				Rule:  r.ID,
				Line:  1 + strings.Count(src[:sp[0]], "\n"),
				Match: clip(strings.TrimSpace(src[sp[0]:sp[1]]), 80),
			})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Line < hits[j].Line })
	return hits
}

// Counts tallies hits per rule.
func Counts(hits []Hit) map[string]int {
	c := map[string]int{}
	for _, h := range hits {
		c[h.Rule]++
	}
	return c
}

// Rates converts hit counts to occurrences per 1000 words for every rule, so a
// corpus document reports zeros as well as sightings.
func Rates(text string) map[string]float64 {
	n := float64(max(stylo.WordCount(text), 1))
	c := Counts(Detect(text))
	out := make(map[string]float64, len(Rules))
	for _, r := range Rules {
		out[r.ID] = 1000 * float64(c[r.ID]) / n
	}
	return out
}

// Baseline is how often the writer's own corpus shows a rule: the mean rate
// per 1000 words and the share of documents with at least one sighting.
type Baseline struct {
	Rate    float64 `json:"rate"`
	DocFrac float64 `json:"doc_frac"`
}

// Finding is the verdict on one rule for one draft.
type Finding struct {
	Rule     string  `json:"rule"`
	Name     string  `json:"name"`
	Strength string  `json:"strength"`
	Count    int     `json:"count"`
	Allowed  float64 `json:"allowed"` // what the writer's own rate would produce
	Excess   float64 `json:"excess"`
	Penalty  float64 `json:"penalty"`
	Fix      string  `json:"fix"`
	Examples []Hit   `json:"examples,omitempty"`
}

// Assessment sums the tell findings for a draft.
type Assessment struct {
	Penalty  float64   `json:"penalty"` // points to subtract from 100
	HardFail bool      `json:"hard_fail"`
	Findings []Finding `json:"findings"`
}

var perHit = map[Strength]float64{Weak: 1.5, Medium: 3, Strong: 6, Hard: 30}
var ruleCap = map[Strength]float64{Weak: 6, Medium: 12, Strong: 18, Hard: 30}

// MaxPenalty bounds the total tell penalty so resemblance still matters.
const MaxPenalty = 60

// Assess judges hits against the writer's baseline. A rule only costs points
// for sightings beyond what the writer's own rate would produce in a text this
// long, plus one free sighting for anything the writer uses at all. With no
// baseline (nil map) every sighting is excess, the strictest reading.
//
// Weak rules only cost points when at least one other rule is also over the
// writer's rate.
func Assess(hits []Hit, words int, base map[string]Baseline) Assessment {
	counts := Counts(hits)
	examples := map[string][]Hit{}
	for _, h := range hits {
		if len(examples[h.Rule]) < 3 {
			examples[h.Rule] = append(examples[h.Rule], h)
		}
	}
	a := Assessment{Findings: []Finding{}} // [] not null in JSON
	overNonWeak := 0
	overTotal := 0
	for _, r := range Rules {
		c := counts[r.ID]
		if c == 0 {
			continue
		}
		b := base[r.ID]
		allowed := 0.0
		if r.Strength != Hard && b.DocFrac > 0 {
			allowed = 1.25*b.Rate*float64(words)/1000 + 1
		}
		excess := math.Max(0, float64(c)-allowed)
		f := Finding{
			Rule: r.ID, Name: r.Name, Strength: r.Strength.String(), Count: c,
			Allowed: round1(allowed), Excess: round1(excess), Fix: r.Fix, Examples: examples[r.ID],
		}
		if excess > 0 {
			overTotal++
			if r.Strength != Weak {
				overNonWeak++
			}
			f.Penalty = math.Min(ruleCap[r.Strength], perHit[r.Strength]*excess)
			if r.Strength == Hard {
				a.HardFail = true
			}
		}
		a.Findings = append(a.Findings, f)
	}
	for i := range a.Findings {
		f := &a.Findings[i]
		if f.Strength == Weak.String() && f.Penalty > 0 && overTotal < 2 {
			f.Penalty = 0 // weak alone
		}
		a.Penalty += f.Penalty
	}
	a.Penalty = round1(math.Min(MaxPenalty, a.Penalty))
	sort.SliceStable(a.Findings, func(i, j int) bool { return a.Findings[i].Penalty > a.Findings[j].Penalty })
	return a
}

// repeatedOpeners flags the third and later sentence in a run that all start
// with the same word.
func repeatedOpeners(text string) [][2]int {
	var spans [][2]int
	offset := 0
	for _, para := range strings.SplitAfter(text, "\n\n") {
		flat := strings.ReplaceAll(para, "\n", " ")
		prev, run := "", 0
		pos := 0
		for _, s := range stylo.SplitSentences(flat) {
			idx := strings.Index(flat[pos:], s)
			if idx < 0 {
				continue
			}
			start := pos + idx
			pos = start + len(s)
			w := stylo.FirstWord(s)
			if w != "" && w == prev {
				run++
			} else {
				run = 1
			}
			prev = w
			if run >= 3 {
				spans = append(spans, [2]int{offset + start, offset + start + len(w)})
			}
		}
		offset += len(para)
	}
	return spans
}

var (
	triadRe       = regexp.MustCompile(`(?i)\b([\p{L}'-]+(?: [\p{L}'-]+){0,2}), ([\p{L}'-]+(?: [\p{L}'-]+){0,2}),? and ([\p{L}'-]+)\b`)
	notListStart  = regexp.MustCompile(`(?i)^(?:including|such|like|especially|which|who|that|where|when|while|but|so|because|then)\b`)
	notListFinish = regexp.MustCompile(`(?i)^(?:the|a|an|then|i|we|it|he|she|they|you|this|that)$`)
)

// triads flags "A, B, and C" lists of short items, skipping a clause that
// only looks like one ("layers, including X and the …").
func triads(text string) [][2]int {
	var spans [][2]int
	for _, m := range triadRe.FindAllStringSubmatchIndex(text, -1) {
		mid, last := text[m[4]:m[5]], text[m[6]:m[7]]
		if notListStart.MatchString(mid) || notListFinish.MatchString(last) {
			continue
		}
		spans = append(spans, [2]int{m[0], m[1]})
	}
	return spans
}

var minorWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true, "of": true, "to": true,
	"in": true, "on": true, "for": true, "with": true, "at": true, "by": true, "from": true, "as": true,
}

// titleCaseHeadings flags Markdown headings that capitalise every main word.
func titleCaseHeadings(text string) [][2]int {
	var spans [][2]int
	for _, loc := range headingRe.FindAllStringSubmatchIndex(text, -1) {
		words := strings.Fields(text[loc[2]:loc[3]])
		if len(words) < 3 {
			continue
		}
		main, capped := 0, 0
		for _, w := range words[1:] {
			w = strings.Trim(w, `.,:;!?"'()`)
			if w == "" || minorWords[strings.ToLower(w)] {
				continue
			}
			main++
			if r := []rune(w); unicode.IsUpper(r[0]) && strings.ToUpper(w) != w {
				capped++
			}
		}
		if main >= 2 && capped == main {
			spans = append(spans, [2]int{loc[0], loc[1]})
		}
	}
	return spans
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
