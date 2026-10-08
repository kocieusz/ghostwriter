// Package skill renders what the drafting agent reads: the installable
// SKILL.md, the prompt that has the agent author STYLE.md, and the per-request
// context (style guide, excerpts, fingerprint).
package skill

import (
	"embed"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"text/template"

	"github.com/kocieusz/ghostwriter/internal/stylo"
	"github.com/kocieusz/ghostwriter/internal/tells"
	"github.com/kocieusz/ghostwriter/internal/voice"
)

//go:embed templates/*.tmpl
var templates embed.FS

var tmpl = template.Must(template.New("").Funcs(template.FuncMap{"yaml": yamlString}).ParseFS(templates, "templates/*.tmpl"))

// StylePrompt returns the instructions for authoring STYLE.md.
func StylePrompt(profile, stylePath string, p *voice.Profile, docs []voice.Doc) (string, error) {
	var b strings.Builder
	err := tmpl.ExecuteTemplate(&b, "style_prompt.md.tmpl", map[string]string{
		"Profile":     profile,
		"StylePath":   stylePath,
		"Fingerprint": Fingerprint(p),
		"Samples":     AllSamples(docs, 6000),
	})
	return b.String(), err
}

// Context is what the agent reads before drafting.
func Context(profile, style string, p *voice.Profile, docs []voice.Doc, genre string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Voice context: %s\n\n", profile)
	if strings.TrimSpace(style) == "" {
		fmt.Fprintf(&b, "> STYLE.md is missing. Before drafting, run `ghostwriter prompt style -p %s`,\n", profile)
		b.WriteString("> follow it to write STYLE.md, then run this command again.\n\n")
	} else {
		b.WriteString("## STYLE.md\n\n")
		b.WriteString(strings.TrimSpace(style))
		b.WriteString("\n\n")
	}
	b.WriteString("## Fingerprint\n\n")
	b.WriteString(Fingerprint(p))
	b.WriteString("\n## Excerpts to mirror\n\n")
	b.WriteString(Excerpts(p, docs, genre, 2400))
	return b.String()
}

// Fingerprint summarises the profile in prose the agent can act on.
func Fingerprint(p *voice.Profile) string {
	var b strings.Builder
	f := func(k string) float64 { return p.Features[k].Mean }
	fmt.Fprintf(&b, "- Corpus: %d samples, %d words. Genres: %s.\n", len(p.Docs), p.TotalWords, strings.Join(Genres(p), ", "))
	fmt.Fprintf(&b, "- Sentences: ~%.0f words on average, spread ±%.0f; %.0f%% over 30 words, %.0f%% six or fewer.\n",
		f("mean_sentence_len"), f("std_sentence_len"), f("pct_long_sentences"), f("pct_short_sentences"))
	fmt.Fprintf(&b, "- Paragraphs: ~%.1f sentences; %.0f%% are a single sentence.\n",
		f("mean_paragraph_sentences"), f("pct_single_sentence_paragraphs"))
	fmt.Fprintf(&b, "- Per 1000 words: %.0f first-person, %.0f hedges, %.0f intensifiers, %.0f contractions, %.1f exclamation marks, %.1f semicolons.\n",
		f("first_person_rate"), f("hedge_rate"), f("intensifier_rate"), f("contraction_rate"), f("exclamation_rate"), f("semicolon_rate"))
	if v := f("lowercase_i_rate"); v >= 1 {
		fmt.Fprintf(&b, "- Types a lowercase \"i\" about %.0f times per 1000 words.\n", v)
	}
	if len(p.Signatures) > 0 {
		fmt.Fprintf(&b, "- Signature phrases: %s.\n", quoted(p.Signatures))
	}
	if len(p.Openers) > 0 {
		fmt.Fprintf(&b, "- Usual sentence openers: %s.\n", quoted(p.Openers))
	}
	var never, uses []string
	for _, r := range tells.Rules {
		if r.Strength == tells.Hard {
			continue
		}
		bl := p.Tells[r.ID]
		if bl.DocFrac == 0 {
			never = append(never, r.Name)
		} else if bl.DocFrac >= 0.25 {
			uses = append(uses, fmt.Sprintf("%s (%.1f per 1000 words)", r.Name, bl.Rate))
		}
	}
	if len(never) > 0 {
		fmt.Fprintf(&b, "- Never uses: %s.\n", strings.Join(never, "; "))
	}
	if len(uses) > 0 {
		fmt.Fprintf(&b, "- Does use, so keep at this rate: %s.\n", strings.Join(uses, "; "))
	}
	if c := p.Calibration; c != nil {
		fmt.Fprintf(&b, "- Pass mark: %.0f (own samples score ~%.0f, generic AI drafts ~%.0f).\n", c.Target, c.LOOMean, c.AIMean)
	}
	return b.String()
}

// Genres lists the genres in the profile.
func Genres(p *voice.Profile) []string {
	return slices.Sorted(maps.Keys(p.Genres))
}

// Excerpts picks, for each genre (or just the requested one plus one
// fallback), the sample closest to that genre's average style rather than
// whichever happened to be first, and trims it at a paragraph boundary.
func Excerpts(p *voice.Profile, docs []voice.Doc, genre string, limit int) string {
	byGenre := map[string][]voice.Doc{}
	for _, d := range docs {
		byGenre[d.Genre] = append(byGenre[d.Genre], d)
	}
	genres := slices.Sorted(maps.Keys(byGenre))
	if genre != "" {
		if _, ok := byGenre[genre]; ok {
			genres = []string{genre}
		}
	}
	opt := stylo.Options{Signatures: p.Signatures, Openers: p.Openers}
	var b strings.Builder
	for _, g := range genres {
		best, bestDist := voice.Doc{}, math.Inf(1)
		for _, d := range byGenre[g] {
			if dist := distance(stylo.Features(d.Text, opt), p, g); dist < bestDist {
				best, bestDist = d, dist
			}
		}
		fmt.Fprintf(&b, "### %s (%s)\n\n%s\n\n", g, best.Name, trim(best.Text, limit))
	}
	return b.String()
}

// AllSamples concatenates every sample, each trimmed, for STYLE.md authoring.
func AllSamples(docs []voice.Doc, limit int) string {
	var b strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&b, "### %s (%s)\n\n%s\n\n", d.Name, d.Genre, trim(d.Text, limit))
	}
	return b.String()
}

var distanceFeatures = []string{
	"mean_sentence_len", "cv_sentence_len", "mean_paragraph_sentences", "mean_word_len",
	"discourse_marker_rate", "first_person_rate", "hedge_rate", "contraction_rate", "comma_per_sentence",
}

func distance(f map[string]float64, p *voice.Profile, genre string) float64 {
	var d float64
	for _, k := range distanceFeatures {
		mean := p.Features[k].Mean
		if g, ok := p.Genres[genre]; ok {
			mean = g.Features[k]
		}
		sd := math.Max(p.Features[k].Std, 1e-6+0.1*math.Abs(p.Features[k].Mean))
		z := (f[k] - mean) / sd
		d += z * z
	}
	return d
}

func trim(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	cut := text[:limit]
	if i := strings.LastIndex(cut, "\n\n"); i > limit/2 {
		cut = cut[:i]
	} else if i := strings.LastIndexAny(cut, " \n"); i > 0 {
		cut = cut[:i]
	}
	return cut + " […]"
}

func quoted(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = fmt.Sprintf("%q", x)
	}
	return strings.Join(q, ", ")
}
