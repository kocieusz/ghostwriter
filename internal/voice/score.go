package voice

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/kocieusz/ghostwriter/internal/facts"
	"github.com/kocieusz/ghostwriter/internal/stylo"
	"github.com/kocieusz/ghostwriter/internal/tells"
)

// feature describes how one feature contributes to resemblance.
type feature struct {
	weight float64
	// banded features are voice density: reaching the writer's mean earns
	// full credit up to band std-devs above it, and only beyond that does
	// overshooting cost (an LLM told to "add hedges" stuffs them in).
	banded bool
	low    string // note when the draft is under the writer
	high   string // note when it is over
}

var features = map[string]feature{
	"mean_sentence_len": {1.0, false,
		"Sentences too short (avg %.0f vs the writer's ~%.0f words); chain clauses the way they do.",
		"Sentences too long (avg %.0f vs the writer's ~%.0f words); split some to match their rhythm."},
	"std_sentence_len": {0.5, false,
		"Sentence lengths too even (spread %.0f vs ~%.0f); mix short and long ones.", ""},
	"cv_sentence_len": {0.75, false,
		"Rhythm too uniform (variation %.2f vs ~%.2f); real writing alternates short and long sentences.",
		"Rhythm more erratic than the writer's (variation %.2f vs ~%.2f)."},
	"pct_long_sentences":  {0.5, false, "", ""},
	"pct_short_sentences": {0.5, false, "", "Too many very short sentences (%.0f%% vs ~%.0f%%); fragments for effect are an AI habit."},
	"mean_paragraph_sentences": {0.5, false,
		"Paragraphs too short (%.1f vs ~%.1f sentences); merge some.",
		"Paragraphs too long (%.1f vs ~%.1f sentences); break them up like the writer does."},
	"pct_single_sentence_paragraphs": {0.5, false, "",
		"Too many one-sentence paragraphs (%.0f%% vs ~%.0f%%); fold one-liners into the paragraph they belong to."},
	"mean_word_len": {0.75, false, "",
		"Vocabulary too heavy (avg word %.1f vs ~%.1f letters); use the writer's plainer words."},
	"mattr": {0.5, false, "", ""},
	"discourse_marker_rate": {1.25, true,
		"Too few of the writer's connectives (%.0f vs ~%.0f per 1000 words); reuse the ones in STYLE.md.",
		"Too much signposting (%.0f vs ~%.0f per 1000 words); ease off."},
	"pct_para_initial_connective": {0.75, true,
		"Only %.0f%% of paragraphs open with a connective vs the writer's ~%.0f%%; match how they start paragraphs.", ""},
	"pct_sent_start_conjunction": {0.5, true,
		"Few sentences start with So/And/But/Also (%.0f%% vs ~%.0f%%).", ""},
	"pct_writer_openers": {0.75, false,
		"Sentence openings don't sound like the writer (%.0f%% vs ~%.0f%% start with their usual words).", ""},
	"comma_per_sentence": {0.75, true,
		"Few commas per sentence (%.1f vs ~%.1f); the writer's sentences carry more.", ""},
	"pct_run_on_sentences": {0.75, true,
		"Sentences too tidy (%.0f%% long multi-comma sentences vs ~%.0f%%); let clauses run together like the writer does.", ""},
	"semicolon_rate": {0.3, false, "",
		"More semicolons than the writer uses (%.1f vs ~%.1f per 1000 words)."},
	"question_rate":    {0.3, false, "", ""},
	"exclamation_rate": {0.3, false, "The writer uses more exclamation marks (%.1f vs ~%.1f per 1000 words).", ""},
	"paren_rate":       {0.3, false, "", ""},
	"first_person_rate": {1.25, true,
		"Not personal enough (%.0f vs ~%.0f first-person words per 1000); more I, my, we.", ""},
	"second_person_rate": {0.5, false, "", ""},
	"i_think_rate": {0.75, true,
		"Add the writer's opinion framing (\"I think\", \"I feel\"): now %.0f vs ~%.0f per 1000 words.", ""},
	"hedge_rate": {0.75, true,
		"Add hedging (%.0f vs ~%.0f per 1000 words): probably, maybe, kind of.",
		"Hedging overdone (%.0f vs ~%.0f per 1000 words); keep only real doubt."},
	"intensifier_rate": {0.75, true,
		"Add intensifiers (%.0f vs ~%.0f per 1000 words): really, so, super.",
		"Intensifiers overdone (%.0f vs ~%.0f per 1000 words)."},
	"signature_rate": {1.0, true,
		"Missing the writer's signature phrases (%.0f vs ~%.0f per 1000 words)", ""},
	"contraction_rate": {0.75, false,
		"Too few contractions (%.0f vs ~%.0f per 1000 words); the writer contracts (it's, don't).",
		"Too many contractions (%.0f vs ~%.0f per 1000 words); the writer spells them out."},
	"lowercase_i_rate": {0.3, true,
		"The writer often types a lowercase \"i\" (%.0f vs ~%.0f per 1000 words); keep that quirk.", ""},
	"cap_anomaly_rate": {0.2, true, "", ""},
}

// featureNames fixes the iteration order so equal-impact notes come out the
// same way every run.
var featureNames = slices.Sorted(maps.Keys(features))

const (
	fwWeight  = 0.5
	tolerance = 1.6
	band      = 2.0
)

// Options tune one scoring run.
type Options struct {
	Genre  string
	Target float64 // 0 means the calibrated target, else DefaultTarget
	Source string  // brief or original text to check facts against; empty skips
}

// DefaultTarget is used when the profile is uncalibrated.
const DefaultTarget = 75

// FeatureScore is one feature's contribution.
type FeatureScore struct {
	Draft  float64 `json:"draft"`
	Mean   float64 `json:"mean"`
	Z      float64 `json:"z"`
	Score  float64 `json:"score"`
	Points float64 `json:"points_lost"`
}

// Note is one actionable instruction, ranked by the points it would recover.
type Note struct {
	Points float64 `json:"points"`
	Kind   string  `json:"kind"` // voice, tell, fact
	Text   string  `json:"text"`
}

// Result is a scored draft.
type Result struct {
	Overall     float64                 `json:"overall"`
	Resemblance float64                 `json:"resemblance"`
	TellPenalty float64                 `json:"tell_penalty"`
	FactPenalty float64                 `json:"fact_penalty"`
	Band        string                  `json:"band"`
	Target      float64                 `json:"target"`
	Pass        bool                    `json:"pass"`
	Blockers    []string                `json:"blockers,omitempty"`
	Words       int                     `json:"words"`
	Genre       string                  `json:"genre,omitempty"`
	Notes       []Note                  `json:"notes"`
	Tells       tells.Assessment        `json:"tells"`
	Facts       *facts.Report           `json:"facts,omitempty"`
	Features    map[string]FeatureScore `json:"features"`
}

// Score grades a draft against the profile.
func Score(p *Profile, draft string, o Options) Result {
	opt := stylo.Options{Signatures: p.Signatures, Openers: p.Openers}
	df := stylo.Features(draft, opt)
	words := stylo.WordCount(draft)
	r := Result{Words: words, Features: map[string]FeatureScore{}, Target: o.Target}
	if r.Target == 0 {
		r.Target = DefaultTarget
		if p.Calibration != nil {
			r.Target = p.Calibration.Target
		}
	}

	means := map[string]float64{}
	for k, s := range p.Features {
		means[k] = s.Mean
	}
	if g, ok := p.Genres[o.Genre]; ok && o.Genre != "" {
		// shrink the genre mean toward the global one: a genre with two
		// samples is a hint, not a fingerprint
		r.Genre = o.Genre
		k := 2.0
		for f, gm := range g.Features {
			means[f] = (float64(g.Docs)*gm + k*means[f]) / (float64(g.Docs) + k)
		}
	}

	// short drafts give noisy rates, so loosen the Gaussian for them
	tol := tolerance
	if words > 0 && p.MedianWords > words {
		tol *= math.Min(2, math.Sqrt(float64(p.MedianWords)/float64(words)))
	}

	var total, got float64
	for _, f := range features {
		total += f.weight
	}
	total += fwWeight

	var notes []Note
	for _, name := range featureNames {
		f := features[name]
		st, ok := p.Features[name]
		if !ok {
			continue
		}
		mean := means[name]
		sigma := clampSigma(name, mean, st.Std)
		d := df[name]
		z := (d - mean) / sigma
		var fs float64
		switch {
		case f.banded && d >= mean && d <= mean+band*sigma:
			fs = 1
		case f.banded && d > mean:
			fs = gauss((d-mean-band*sigma)/sigma, tol)
		default:
			fs = gauss(z, tol)
		}
		got += f.weight * fs
		lost := 100 * f.weight * (1 - fs) / total
		r.Features[name] = FeatureScore{round(d, 2), round(mean, 2), round(z, 2), round(fs, 3), round(lost, 1)}
		if fs >= 0.75 {
			continue
		}
		msg := f.low
		if d > mean {
			msg = f.high
		}
		if msg == "" {
			continue
		}
		msg = fmt.Sprintf(msg, d, mean)
		if name == "signature_rate" && len(p.Signatures) > 0 {
			msg += ", e.g. " + quoteList(p.Signatures[:min(4, len(p.Signatures))])
		}
		if name == "pct_writer_openers" && len(p.Openers) > 0 {
			msg += " Their usual openers: " + quoteList(p.Openers) + "."
		}
		notes = append(notes, Note{round(lost, 1), "voice", msg})
	}

	fwc := fwCosine(df, means)
	fws := math.Max(0, math.Min(1, (fwc-0.5)/0.4))
	got += fwWeight * fws
	fwLost := 100 * fwWeight * (1 - fws) / total
	r.Features["function_word_cosine"] = FeatureScore{Draft: round(fwc, 3), Score: round(fws, 3), Points: round(fwLost, 1)}
	if fwc < 0.85 {
		notes = append(notes, Note{round(fwLost, 1), "voice",
			"The mix of small words (the, and, so, just…) drifts from the writer's; reread a sample and echo its connectors."})
	}
	r.Resemblance = round(100*got/total, 1)

	// AI tells, judged against what this writer does anyway
	r.Tells = tells.Assess(tells.Detect(draft), words, p.Tells)
	r.TellPenalty = r.Tells.Penalty
	for _, f := range r.Tells.Findings {
		if f.Penalty == 0 {
			continue
		}
		where := make([]string, 0, len(f.Examples))
		for _, h := range f.Examples {
			where = append(where, fmt.Sprintf("L%d %q", h.Line, h.Match))
		}
		notes = append(notes, Note{f.Penalty, "tell", fmt.Sprintf("AI tell, %s (%d× vs %.1f allowed for this writer): %s [%s]",
			f.Name, f.Count, f.Allowed, f.Fix, strings.Join(where, "; "))})
	}
	if r.Tells.HardFail {
		r.Blockers = append(r.Blockers, "contains a hard AI tell (chatbot markup, placeholder or AI self-reference)")
	}

	if o.Source != "" {
		fr := facts.Compare(o.Source, draft)
		r.Facts = &fr
		r.FactPenalty = fr.Penalty()
		add := func(pts float64, what string, items []string) {
			if len(items) > 0 {
				notes = append(notes, Note{pts * float64(len(items)), "fact", what + quoteList(items)})
			}
		}
		add(5, "Invented numbers not in the source; remove them or ask for the real figures: ", fr.AddedNumbers)
		add(5, "Quotation not in the source; never invent quotes: ", fr.AddedQuotes)
		add(3, "Names not in the source; check they aren't made up: ", fr.AddedNames)
		add(2, "Numbers from the source went missing: ", fr.DroppedNumbers)
		add(1, "Names from the source went missing: ", fr.DroppedNames)
		if fr.Fabricated() {
			r.Blockers = append(r.Blockers, "adds numbers or quotations that are not in the source")
		}
	}

	r.Overall = round(math.Max(0, r.Resemblance-r.TellPenalty-r.FactPenalty), 1)
	r.Band = "far"
	if r.Overall >= 80 {
		r.Band = "close"
	} else if r.Overall >= 60 {
		r.Band = "medium"
	}
	r.Pass = r.Overall >= r.Target && len(r.Blockers) == 0
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].Points > notes[j].Points })
	r.Notes = notes
	return r
}

// clampSigma keeps a feature's spread inside sensible bounds. Too narrow and a
// writer who never used a semicolon fails any draft with one; too wide and the
// feature stops discriminating.
func clampSigma(name string, mean, std float64) float64 {
	floor := 0.15 * math.Abs(mean)
	switch {
	case strings.HasSuffix(name, "_rate"):
		floor = math.Max(floor, 1.5)
	case strings.HasPrefix(name, "pct_"):
		floor = math.Max(floor, 4)
	case name == "mean_sentence_len" || name == "std_sentence_len":
		floor = math.Max(floor, 1.5)
	default:
		floor = math.Max(floor, 0.02)
	}
	ceil := math.Max(0.6*math.Abs(mean), floor)
	return math.Min(math.Max(std, floor), ceil)
}

func gauss(z, tol float64) float64 { return math.Exp(-0.5 * (z / tol) * (z / tol)) }

func fwCosine(draft, means map[string]float64) float64 {
	var dot, na, nb float64
	for _, w := range stylo.FunctionWords {
		a, b := draft["fw_"+w], means["fw_"+w]
		dot += a * b
		na += a * a
		nb += b * b
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

func round(x float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(x*p) / p
}

func quoteList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = fmt.Sprintf("%q", x)
	}
	return strings.Join(q, ", ")
}
