// Package voice builds a writer's stylometric profile from their corpus and
// scores drafts against it.
package voice

import (
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/kocieusz/ghostwriter/internal/stylo"
	"github.com/kocieusz/ghostwriter/internal/tells"
)

// Doc is one writing sample in the corpus.
type Doc struct {
	Name  string `json:"name"`
	Genre string `json:"genre"`
	Words int    `json:"words"`
	Hash  string `json:"hash"`
	Text  string `json:"text"`
}

// Stat summarises one feature across the corpus.
type Stat struct {
	Mean float64 `json:"mean"`
	Std  float64 `json:"std"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// Genre holds per-genre feature means.
type Genre struct {
	Docs     int                `json:"docs"`
	Features map[string]float64 `json:"features"`
}

// DocInfo is the corpus index stored in the profile.
type DocInfo struct {
	Name  string `json:"name"`
	Genre string `json:"genre"`
	Words int    `json:"words"`
}

// Profile is the writer's fingerprint.
type Profile struct {
	Version     int                       `json:"version"`
	Docs        []DocInfo                 `json:"docs"`
	TotalWords  int                       `json:"total_words"`
	MedianWords int                       `json:"median_words"`
	Signatures  []string                  `json:"signatures"`
	Openers     []string                  `json:"openers"`
	Features    map[string]Stat           `json:"features"`
	Genres      map[string]Genre          `json:"genres"`
	Tells       map[string]tells.Baseline `json:"tells"`
	Calibration *Calibration              `json:"calibration,omitempty"`
}

// ProfileVersion is bumped when the feature set changes, so old profiles get
// rebuilt instead of compared against features they don't have.
const ProfileVersion = 1

// Build computes a profile from the corpus. It does not calibrate.
func Build(docs []Doc) *Profile {
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Text
	}
	p := &Profile{
		Version:    ProfileVersion,
		Signatures: Signatures(texts, 12),
		Openers:    Openers(texts, 8),
		Features:   map[string]Stat{},
		Genres:     map[string]Genre{},
		Tells:      map[string]tells.Baseline{},
	}
	opt := stylo.Options{Signatures: p.Signatures, Openers: p.Openers}
	feats := make([]map[string]float64, len(docs))
	words := make([]int, len(docs))
	for i, d := range docs {
		feats[i] = stylo.Features(d.Text, opt)
		words[i] = stylo.WordCount(d.Text)
		p.Docs = append(p.Docs, DocInfo{d.Name, d.Genre, words[i]})
		p.TotalWords += words[i]
	}
	sorted := slices.Clone(words)
	slices.Sort(sorted)
	if len(sorted) > 0 {
		p.MedianWords = sorted[len(sorted)/2]
	}
	p.Features = aggregate(feats)

	byGenre := map[string][]map[string]float64{}
	for i, d := range docs {
		byGenre[d.Genre] = append(byGenre[d.Genre], feats[i])
	}
	for g, fs := range byGenre {
		means := map[string]float64{}
		for k, s := range aggregate(fs) {
			means[k] = s.Mean
		}
		p.Genres[g] = Genre{Docs: len(fs), Features: means}
	}

	// tell baseline: how much of each "AI" pattern this writer produces
	for _, r := range tells.Rules {
		p.Tells[r.ID] = tells.Baseline{}
	}
	for _, d := range docs {
		for id, rate := range tells.Rates(d.Text) {
			b := p.Tells[id]
			b.Rate += rate / float64(len(docs))
			if rate > 0 {
				b.DocFrac += 1 / float64(len(docs))
			}
			p.Tells[id] = b
		}
	}
	return p
}

func aggregate(fs []map[string]float64) map[string]Stat {
	out := map[string]Stat{}
	if len(fs) == 0 {
		return out
	}
	for k := range fs[0] {
		vals := make([]float64, len(fs))
		for i, f := range fs {
			vals[i] = f[k]
		}
		mean, std := stylo.MeanStd(vals)
		out[k] = Stat{Mean: mean, Std: std, Min: slices.Min(vals), Max: slices.Max(vals)}
	}
	return out
}

// stoppy words never make a phrase distinctive on their own.
var stoppy = map[string]bool{}

func init() {
	for w := range strings.FieldsSeq(`the a an of to in on for and or is are was were it that this with as at by from
		be will would can has have had i we you my our your me us they their there he she his her its not but so if
		about also first then just like all one some what when which been do did does get got more out up`) {
		stoppy[w] = true
	}
}

// Signatures finds 2–4 word phrases that recur across several of the writer's
// documents. A phrase has to appear in at least a quarter of the documents
// (and at least two), must contain a content word, and is dropped when a
// longer kept phrase already contains it.
func Signatures(texts []string, topK int) []string {
	if len(texts) < 2 {
		return nil
	}
	minDocs := max(2, int(math.Round(0.25*float64(len(texts)))))
	df := map[string]int{}
	tf := map[string]int{}
	docStrings := make([]string, len(texts))
	for i, t := range texts {
		toks := stylo.Lower(stylo.Mask(stylo.Normalize(t)))
		docStrings[i] = " " + strings.Join(toks, " ") + " "
		seen := map[string]bool{}
		for n := 2; n <= 4; n++ {
			for i := 0; i+n <= len(toks); i++ {
				g := strings.Join(toks[i:i+n], " ")
				tf[g]++
				if !seen[g] {
					seen[g] = true
					df[g]++
				}
			}
		}
	}
	var cands []string
	for g, c := range df {
		if c < minDocs {
			continue
		}
		content := false
		for w := range strings.FieldsSeq(g) {
			if !stoppy[w] {
				content = true
			}
		}
		if content {
			cands = append(cands, g)
		}
	}
	byRank := func(a, b string) int {
		if df[a] != df[b] {
			return df[b] - df[a]
		}
		if tf[a] != tf[b] {
			return tf[b] - tf[a]
		}
		return strings.Compare(a, b)
	}
	slices.SortFunc(cands, byRank)
	if len(cands) > topK*4 {
		cands = cands[:topK*4]
	}
	cands = mergeOverlaps(cands, docStrings, minDocs, df, tf)
	slices.SortFunc(cands, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})
	var kept []string
	for _, g := range cands {
		if !slices.ContainsFunc(kept, func(k string) bool { return strings.Contains(" "+k+" ", " "+g+" ") }) {
			kept = append(kept, g)
		}
	}
	slices.SortFunc(kept, byRank)
	if len(kept) > topK {
		kept = kept[:topK]
	}
	return kept
}

// mergeOverlaps joins phrases that are shifted windows of one longer phrase
// ("at the end of" + "end of the day" → "at the end of the day") whenever the
// joined phrase itself recurs in enough documents. Without this one habit
// fills half the signature list.
func mergeOverlaps(cands, docs []string, minDocs int, df, tf map[string]int) []string {
	count := func(g string) (docsWith, total int) {
		for _, d := range docs {
			if c := strings.Count(d, " "+g+" "); c > 0 {
				docsWith++
				total += c
			}
		}
		return
	}
	for merged := true; merged; {
		merged = false
	outer:
		for i, a := range cands {
			at := strings.Fields(a)
			for j, b := range cands {
				if i == j {
					continue
				}
				bt := strings.Fields(b)
				for ov := min(len(at), len(bt)) - 1; ov >= 1; ov-- {
					if !slices.Equal(at[len(at)-ov:], bt[:ov]) {
						continue
					}
					g := strings.Join(append(slices.Clone(at), bt[ov:]...), " ")
					if n, total := count(g); n >= minDocs {
						df[g], tf[g] = n, total
						cands = slices.DeleteFunc(cands, func(c string) bool { return c == a || c == b })
						cands = append(cands, g)
						merged = true
						break outer
					}
				}
			}
		}
	}
	return cands
}

// Openers returns the words that most often open the writer's sentences.
func Openers(texts []string, topK int) []string {
	counts := map[string]int{}
	for _, t := range texts {
		for _, s := range stylo.Sentences(stylo.Mask(stylo.Normalize(t))) {
			if w := stylo.FirstWord(s); w != "" {
				counts[w]++
			}
		}
	}
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range counts {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	var out []string
	for _, e := range all[:min(topK, len(all))] {
		if e.v >= 2 {
			out = append(out, e.k)
		}
	}
	return out
}
