package stylo

import (
	"math"
	"regexp"
	"slices"
	"strings"
)

// FunctionWords are the classic stylometric function words. Their rates form a
// vector compared by cosine similarity rather than one by one.
var FunctionWords = []string{
	"the", "a", "an", "and", "but", "or", "so", "because", "if", "when",
	"of", "to", "in", "on", "for", "with", "as", "at", "by", "from",
	"that", "this", "it", "is", "are", "was", "were", "be", "will", "would",
	"can", "could", "not", "also", "however", "then", "very", "really", "just",
}

// DiscourseMarkers are signposting connectives, counted anywhere and at
// paragraph starts.
var DiscourseMarkers = []string{
	"firstly", "secondly", "thirdly", "lastly", "finally",
	"however", "also", "besides", "moreover", "furthermore",
	"luckily", "unfortunately", "lately", "nowadays", "these days",
	"in conclusion", "on the other hand", "to sum up", "all in all",
	"so", "and also", "for example", "in my opinion", "honestly",
	"basically", "actually", "okay so", "first of all", "anyway", "but",
}

// Hedges soften a claim. Writers hedge at a stable personal rate.
var Hedges = []string{
	"probably", "maybe", "perhaps", "for sure", "surely", "somehow",
	"i think", "i believe", "kind of", "sort of", "might", "may", "i guess",
	"to be fair", "i feel like", "i suppose", "apparently", "seems",
}

// Intensifiers amplify a claim.
var Intensifiers = []string{
	"really", "very", "extremely", "strongly", "totally", "absolutely",
	"completely", "super", "quite", "a lot", "tons of", "so much", "way too",
	"insanely", "pretty much", "literally",
}

var (
	contractionRe = regexp.MustCompile(`\b\p{L}+'(?:t|s|re|ve|ll|d|m)\b`)
	lowerIRe      = regexp.MustCompile(`(?:^|[^\p{L}'])i(?:[^\p{L}']|$)`)
	capWordRe     = regexp.MustCompile(`^[\p{Lu}][\p{Ll}]+$`)
	startsIRe     = regexp.MustCompile(`^\W*I\b`)
)

// Options carry the writer-specific lists that make some features comparable
// between the corpus and a draft.
type Options struct {
	Signatures []string // the writer's recurring phrases
	Openers    []string // the writer's most frequent sentence-opening words
}

// Features extracts the flat feature vector. Rates are per 1000 words and
// percentages are 0–100 unless the name says otherwise, so documents of
// different lengths compare directly.
func Features(text string, opt Options) map[string]float64 {
	text = Mask(Normalize(text))
	raw := Words(text)
	toks := Lower(text)
	n := float64(max(len(raw), 1))
	sents := Sentences(text)
	ns := float64(max(len(sents), 1))
	paras := Paragraphs(text)
	np := float64(max(len(paras), 1))
	per1k := func(c int) float64 { return 1000 * float64(c) / n }
	pct := func(c int, of float64) float64 { return 100 * float64(c) / of }

	f := map[string]float64{}

	// --- structure ------------------------------------------------------
	lens := make([]float64, len(sents))
	long, short := 0, 0
	for i, s := range sents {
		lens[i] = float64(len(Words(s)))
		if lens[i] > 30 {
			long++
		}
		if lens[i] <= 6 {
			short++
		}
	}
	mean, std := MeanStd(lens)
	f["mean_sentence_len"] = mean
	f["std_sentence_len"] = std
	// coefficient of variation: burstiness independent of average length
	if mean > 0 {
		f["cv_sentence_len"] = std / mean
	}
	f["pct_long_sentences"] = pct(long, ns)
	f["pct_short_sentences"] = pct(short, ns)
	f["mean_paragraph_sentences"] = ns / np
	single := 0
	for _, p := range paras {
		if len(SplitSentences(softBreak.ReplaceAllString(p, " "))) == 1 {
			single++
		}
	}
	f["pct_single_sentence_paragraphs"] = pct(single, np)
	wl := 0
	for _, w := range raw {
		wl += len([]rune(w))
	}
	f["mean_word_len"] = float64(wl) / n
	f["mattr"] = mattr(toks, 50)

	// --- discourse and openers -----------------------------------------
	f["discourse_marker_rate"] = per1k(CountPhrases(toks, DiscourseMarkers))
	paraConn := 0
	for _, p := range paras {
		if StartsWithAny(p, DiscourseMarkers) {
			paraConn++
		}
	}
	f["pct_para_initial_connective"] = pct(paraConn, np)
	soAlsoAnd, startI, opener := 0, 0, 0
	for _, s := range sents {
		if StartsWithAny(s, []string{"so", "also", "and", "but"}) {
			soAlsoAnd++
		}
		if startsIRe.MatchString(s) {
			startI++
		}
		if fw := FirstWord(s); fw != "" && slices.Contains(opt.Openers, fw) {
			opener++
		}
	}
	f["pct_sent_start_conjunction"] = pct(soAlsoAnd, ns)
	f["pct_sent_start_i"] = pct(startI, ns)
	f["pct_writer_openers"] = pct(opener, ns)

	// --- punctuation ----------------------------------------------------
	f["comma_per_sentence"] = float64(strings.Count(text, ",")) / ns
	multiComma := 0
	for i, s := range sents {
		if strings.Count(s, ",") >= 2 && lens[i] > 20 {
			multiComma++
		}
	}
	f["pct_run_on_sentences"] = pct(multiComma, ns)
	f["semicolon_rate"] = per1k(strings.Count(text, ";"))
	f["question_rate"] = per1k(strings.Count(text, "?"))
	f["exclamation_rate"] = per1k(strings.Count(text, "!"))
	f["paren_rate"] = per1k(strings.Count(text, "("))
	f["ellipsis_rate"] = per1k(strings.Count(text, "...") + strings.Count(text, "…"))

	// --- voice ----------------------------------------------------------
	f["first_person_rate"] = per1k(CountPhrases(toks, []string{"i", "me", "my", "mine", "we", "our", "us", "i'm", "i've", "i'd", "i'll"}))
	f["second_person_rate"] = per1k(CountPhrases(toks, []string{"you", "your", "yours", "you're", "you've", "you'll"}))
	f["i_think_rate"] = per1k(CountPhrases(toks, []string{"i think", "i would", "i feel", "i guess", "i believe"}))
	f["hedge_rate"] = per1k(CountPhrases(toks, Hedges))
	f["intensifier_rate"] = per1k(CountPhrases(toks, Intensifiers))
	f["signature_rate"] = per1k(CountPhrases(toks, opt.Signatures))
	f["contraction_rate"] = per1k(len(contractionRe.FindAllString(strings.ToLower(text), -1)))

	// --- quirks ---------------------------------------------------------
	f["lowercase_i_rate"] = per1k(len(lowerIRe.FindAllString(text, -1)))
	capAnomaly := 0
	for _, s := range sents {
		fields := strings.Fields(s)
		for _, w := range fields[min(1, len(fields)):] {
			if capWordRe.MatchString(strings.Trim(w, `.,;:!?"'()`)) {
				capAnomaly++
			}
		}
	}
	f["cap_anomaly_rate"] = per1k(capAnomaly)

	// --- function-word vector ------------------------------------------
	counts := map[string]int{}
	for _, t := range toks {
		counts[t]++
	}
	for _, w := range FunctionWords {
		f["fw_"+w] = per1k(counts[w])
	}
	return f
}

// WordCount counts prose words, ignoring code and URLs.
func WordCount(text string) int {
	return len(Words(Mask(Normalize(text))))
}

// MeanStd returns the population mean and standard deviation.
func MeanStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var v float64
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(v / float64(len(xs)))
}

// mattr is the moving-average type-token ratio, a length-robust measure of
// vocabulary richness.
func mattr(toks []string, window int) float64 {
	if len(toks) == 0 {
		return 0
	}
	if len(toks) <= window {
		return float64(len(uniq(toks))) / float64(len(toks))
	}
	counts := map[string]int{}
	for _, t := range toks[:window] {
		counts[t]++
	}
	sum := float64(len(counts))
	for i := window; i < len(toks); i++ {
		out := toks[i-window]
		if counts[out]--; counts[out] == 0 {
			delete(counts, out)
		}
		counts[toks[i]]++
		sum += float64(len(counts))
	}
	return sum / float64(len(toks)-window+1) / float64(window)
}

func uniq(xs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}
