package voice

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// corpus loads the fictional writer in testdata: casual, uses em dashes and
// "in conclusion", signs off with "haha" and "thanks a lot".
func corpus(t *testing.T) []Doc {
	t.Helper()
	paths, err := filepath.Glob("testdata/writer/*.txt")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	var docs []Doc
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(p)
		docs = append(docs, Doc{Name: name, Genre: strings.SplitN(name, "_", 2)[0], Text: string(b)})
	}
	return docs
}

const inVoice = `So the printer on our floor is broken again — it jams on every second page and then just blinks at you. Great. I tried the trick Tomek showed me, twice actually, and it worked for like five minutes.

Honestly I think we should stop fixing it. It's the third time this month and I'm really done, so let's just ask for a new one, I'm happy to write to facilities if nobody else wants to, it's not a big deal.

Also if anyone needs to print something urgent, the one on the fourth floor works fine, I checked this morning haha.`

// inVoiceStuffed keeps the writer's markers but is otherwise default AI prose.
const inVoiceStuffed = `Honestly I think the printer situation is not just an inconvenience — it is a fundamental challenge to our productivity, collaboration, and morale haha.

At its core, a reliable printer plays a crucial role in our workflow, highlighting the profound connection between tools and performance. Also it's kind of a pivotal moment, honestly, so we must navigate the complexities of this evolving landscape.

In conclusion, a new printer stands as a testament to our commitment to excellence. It's not just about printing; it's about how we work, and I think that's really true.`

func TestCalibrationSeparatesWriterFromAI(t *testing.T) {
	docs := corpus(t)
	p := Build(docs)
	c := Calibrate(p, docs)
	if c == nil {
		t.Fatal("expected calibration with 6 docs")
	}
	if !c.Separated || c.LOOMean < c.AIMax+20 {
		t.Fatalf("writer and AI drafts should be far apart: %+v", c)
	}
}

func TestScoreRanksVoiceAboveStuffedAI(t *testing.T) {
	docs := corpus(t)
	p := Build(docs)
	Calibrate(p, docs)

	good := Score(p, inVoice, Options{Genre: "email"})
	bad := Score(p, inVoiceStuffed, Options{Genre: "email"})
	if !good.Pass {
		t.Errorf("in-voice draft should pass: overall %.1f target %.1f notes %v", good.Overall, good.Target, good.Notes)
	}
	if bad.Pass || bad.TellPenalty < 20 {
		t.Errorf("stuffed AI draft should fail on tells: overall %.1f tells %.1f", bad.Overall, bad.TellPenalty)
	}
	if good.Overall-bad.Overall < 30 {
		t.Errorf("want a wide gap, got good %.1f bad %.1f", good.Overall, bad.Overall)
	}
}

// The writer uses em dashes and "in conclusion", so those must not be
// penalised in their drafts.
func TestBaselineKeepsWritersOwnHabits(t *testing.T) {
	docs := corpus(t)
	p := Build(docs)
	r := Score(p, inVoice, Options{})
	for _, f := range r.Tells.Findings {
		if f.Rule == "em_dash" && f.Penalty > 0 {
			t.Fatalf("writer's own dash habit penalised: %+v", f)
		}
	}
}

func TestBandedFeaturesPenaliseStuffing(t *testing.T) {
	docs := corpus(t)
	p := Build(docs)
	stuffed := strings.Repeat("I think maybe I probably kind of guess honestly. ", 20)
	r := Score(p, stuffed, Options{})
	if fs := r.Features["hedge_rate"]; fs.Score > 0.5 {
		t.Fatalf("hedge stuffing should cost points: %+v", fs)
	}
}

func TestFactCheckBlocksInventedNumbers(t *testing.T) {
	docs := corpus(t)
	p := Build(docs)
	brief := "Printer on our floor broken again. Tomek's trick worked briefly. Ask facilities for a new one."
	r := Score(p, inVoice+"\n\nIt has jammed 47 times since March.", Options{Source: brief})
	if r.Pass || r.Facts == nil || !slices.Contains(r.Facts.AddedNumbers, "47") {
		t.Fatalf("invented number should block: pass=%v facts=%+v", r.Pass, r.Facts)
	}
}

func TestSignaturesMergeShiftedWindows(t *testing.T) {
	texts := []string{
		"well at the end of the day it works fine",
		"and at the end of the day we go home",
		"so at the end of the day nobody cares",
	}
	sig := Signatures(texts, 5)
	if len(sig) == 0 || sig[0] != "at the end of the day" {
		t.Fatalf("want the merged phrase first, got %q", sig)
	}
	for _, s := range sig[1:] {
		if strings.Contains("at the end of the day", s) {
			t.Fatalf("fragment %q kept beside the merged phrase: %q", s, sig)
		}
	}
}
