package voice

import (
	"embed"
	"math"
	"path"
	"slices"
	"strings"
)

// airef holds deliberately generic, assistant-style drafts in the common
// genres. Calibration scores them against the writer's profile to show how far
// a default LLM draft lands from their voice.
//
//go:embed airef/*.txt
var airef embed.FS

// Calibration records how well the profile separates the writer from a
// default AI draft, and the pass mark that follows from it.
type Calibration struct {
	// leave-one-out: each real sample scored against a profile built from the others
	LOOMean float64 `json:"loo_mean"`
	LOOLow  float64 `json:"loo_p25"`
	LOOMin  float64 `json:"loo_min"`
	// generic assistant drafts scored against the full profile
	AIMean float64 `json:"ai_mean"`
	AIMax  float64 `json:"ai_max"`
	// Target is the suggested pass mark; Separated reports whether real
	// samples clear it while AI drafts don't.
	Target    float64 `json:"target"`
	Separated bool    `json:"separated"`
	Samples   []Check `json:"samples"`
}

// Check is one calibration score.
type Check struct {
	Name    string  `json:"name"`
	Overall float64 `json:"overall"`
	AI      bool    `json:"ai,omitempty"`
}

// MinCalibrationDocs is the smallest corpus leave-one-out makes sense for.
const MinCalibrationDocs = 4

// Calibrate fills p.Calibration using leave-one-out over docs. It returns nil
// (and leaves p alone) when the corpus is too small.
func Calibrate(p *Profile, docs []Doc) *Calibration {
	if len(docs) < MinCalibrationDocs {
		return nil
	}
	c := &Calibration{}
	var loo []float64
	for i, d := range docs {
		rest := slices.Concat(docs[:i], docs[i+1:])
		r := Score(Build(rest), d.Text, Options{Genre: d.Genre, Target: 1})
		loo = append(loo, r.Overall)
		c.Samples = append(c.Samples, Check{Name: d.Name, Overall: r.Overall})
	}
	entries, _ := airef.ReadDir("airef")
	var ai []float64
	for _, e := range entries {
		b, err := airef.ReadFile(path.Join("airef", e.Name()))
		if err != nil {
			continue
		}
		genre := strings.TrimSuffix(e.Name(), ".txt")
		r := Score(p, string(b), Options{Genre: genre, Target: 1})
		ai = append(ai, r.Overall)
		c.Samples = append(c.Samples, Check{Name: "ai:" + genre, Overall: r.Overall, AI: true})
	}

	slices.Sort(loo)
	slices.Sort(ai)
	c.LOOMean = round(mean(loo), 1)
	c.LOOLow = round(percentile(loo, 0.25), 1)
	c.LOOMin = round(loo[0], 1)
	c.AIMean = round(mean(ai), 1)
	c.AIMax = round(ai[len(ai)-1], 1)

	// Pass mark: low enough that three in four of the writer's own pieces
	// pass, but never at or below the best generic AI draft. When those
	// conflict the profile can't tell the two apart yet, so split the
	// difference and say so.
	t := c.LOOLow
	c.Separated = t > c.AIMax+5
	if !c.Separated {
		t = (c.LOOMean + c.AIMax) / 2
	}
	c.Target = math.Round(math.Max(55, math.Min(90, t)))
	p.Calibration = c
	return c
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// percentile assumes xs is sorted.
func percentile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	pos := q * float64(len(xs)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return xs[lo] + (xs[hi]-xs[lo])*(pos-float64(lo))
}
