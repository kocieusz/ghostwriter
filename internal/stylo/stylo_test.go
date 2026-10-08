package stylo

import (
	"slices"
	"strings"
	"testing"
)

func TestSentences(t *testing.T) {
	got := Sentences("Hi there! How are\nyou? Fine... thanks.\n\nNew para (really.) End")
	want := []string{"Hi there!", "How are you?", "Fine...", "thanks.", "New para (really.)", "End"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestCountPhrasesMatchesWholeTokens(t *testing.T) {
	toks := Lower("Also, hi think so. I think so too, i think.")
	if n := CountPhrases(toks, []string{"i think"}); n != 2 {
		t.Fatalf("i think: %d", n)
	}
	if n := CountPhrases(toks, []string{"so"}); n != 2 {
		t.Fatalf("so (not inside also): %d", n)
	}
}

func TestMaskKeepsOffsets(t *testing.T) {
	in := "a `code` b\n```\nx\n```\nsee https://x.y/z ok"
	out := Mask(in)
	if len(out) != len(in) || strings.Count(out, "\n") != strings.Count(in, "\n") {
		t.Fatal("mask changed length or lines")
	}
	if strings.Contains(out, "code") || strings.Contains(out, "https") {
		t.Fatalf("not masked: %q", out)
	}
}

func TestFeaturesBasics(t *testing.T) {
	f := Features("i think it's fine. I really do.\n\nSo yes.", Options{Openers: []string{"so"}})
	if f["lowercase_i_rate"] == 0 || f["contraction_rate"] == 0 || f["pct_writer_openers"] == 0 {
		t.Fatalf("features missing: %v", f)
	}
	if f["mean_paragraph_sentences"] != 1.5 {
		t.Fatalf("paragraph sentences: %v", f["mean_paragraph_sentences"])
	}
}

func TestUnicodeWords(t *testing.T) {
	if w := Words("Zażółć gęślą jaźń"); len(w) != 3 {
		t.Fatalf("got %q", w)
	}
}
