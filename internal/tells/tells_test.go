package tells

import (
	"strings"
	"testing"
)

func rulesHit(text string) map[string]int { return Counts(Detect(text)) }

// Each "before" example is a typical machine-written sentence and must trip the
// named rule.
func TestDetectBefore(t *testing.T) {
	cases := []struct{ rule, text string }{
		{"negative_parallelism", "It's not just about the beat riding under the vocals; it's part of the aggression."},
		{"negative_parallelism", "This does not mean every choice is equal. It means there is no external system."},
		{"negative_parallelism", "It is not merely a song, it is a statement."},
		{"dramatic_closer", "Caching cuts repeat work.\n\nThat's the real win."},
		{"dramatic_closer", "It had no preference for symmetry. No aesthetic prior. No nostalgia for human taste."},
		{"dramatic_closer", "I check it every. single. day."},
		{"deep_saying", "The real question is whether teams can adapt. At its core, it is about readiness."},
		{"deep_saying", "Symmetry is the language of trust."},
		{"staged_opener", "Let's dive into how caching works in Next.js. Here's what you need to know."},
		{"staged_opener", "Is it worth the price? Honestly? It depends."},
		{"strawman", "This isn't mainly about prompt length, and I'm not saying docs don't matter."},
		{"chatbot_residue", "Great question! Here is an overview. I hope this helps!"},
		{"chatbot_residue", "Would you like me to expand on any section?"},
		{"source_hedge", "Information about her early life is not publicly available, suggesting she maintains a low profile."},
		{"inflated_significance", "It was established in 1989, marking a pivotal moment in the evolution of regional statistics."},
		{"inflated_significance", "Despite these challenges, Korattur continues to thrive."},
		{"ing_rider", "The temple is blue and gold, symbolizing Texas bluebonnets."},
		{"sales_language", "Nestled within the breathtaking region of Gonder, it has stunning natural beauty."},
		{"borrowed_authority", "Experts believe it plays a crucial role in the regional ecosystem."},
		{"ai_vocabulary", "Additionally, an enduring testament to this is the vibrant tapestry of local cuisine."},
		{"signposting", "It is important to note that the policy changed."},
		{"meta_writing", "This section explores the history of the town."},
		{"em_dash", "The new policy — announced without warning — affects thousands."},
		{"em_dash", "The changes -- long overdue -- take effect now."},
		{"inline_header_list", "- **Performance:** Performance has been enhanced."},
		{"emoji_decoration", "🚀 Launch phase: the product launches in Q3"},
		{"triad", "Attendees can expect innovation, inspiration, and industry insights."},
		{"stacked_qualifiers", "It could potentially be argued that the policy might have an effect."},
		{"copula_avoidance", "Gallery 825 serves as the exhibition space and boasts 3,000 square feet."},
		{"vague_association", "He is associated with the Rajhans Orchestra."},
		{"bold_decoration", "It blends **OKRs** and **KPIs**."},
		{"curly_quotes", "He said “the project is on track”."},
		{"repeated_openers", "She noted the door. She noted the lock on it. She filed both away."},
		{"title_case_heading", "## Strategic Negotiations And Global Partnerships\n\nText."},
		{"chat_artifact", "He settled in Cheshire.:contentReference[oaicite:0]{index=0}"},
		{"chat_artifact", "See https://example.com/story?utm_source=chatgpt.com for more."},
		{"chat_artifact", "Revenue grew 10% [cite: 3, 12]."},
		{"placeholder", "I am writing about the article on [Entertainer's Name], which is wrong."},
		{"placeholder", "Accessed 2025-xx-xx."},
		{"ai_disclosure", "As of my last knowledge update in January 2022, I don't have details."},
	}
	for _, c := range cases {
		if rulesHit(c.text)[c.rule] == 0 {
			t.Errorf("%s: no hit in %q (got %v)", c.rule, c.text, rulesHit(c.text))
		}
	}
}

// The matching "after" rewrites must come out clean.
func TestDetectAfter(t *testing.T) {
	clean := []string{
		"The heavy beat adds to the aggressive tone.",
		"No external system confirms which choice is right, although the choices still have different consequences.",
		"Next.js caches data at multiple layers, including request memoization and the router cache.",
		"Whether it's worth the price depends on how often you'll use it.",
		"The French Revolution began in 1789 when a financial crisis and food shortages led to unrest.",
		"The Statistical Institute of Catalonia was established in 1989.",
		"Gallery 825 is LAAA's exhibition space for contemporary art. The gallery has four rooms.",
		"He founded and conducts the Rajhans Orchestra.",
		"She noted the door and its lock, then filed both away.",
		"## Strategic negotiations and global partnerships",
	}
	for _, text := range clean {
		if h := Detect(text); len(h) > 0 {
			t.Errorf("unexpected hits in %q: %+v", text, h)
		}
	}
}

func TestMaskingSkipsCodeQuotesAndURLs(t *testing.T) {
	text := "Run `delve --verbose` first.\n\n```\nlet's dive in — robust\n```\n\nShe said \"it is a testament to grit\" and left.\n\nSee https://example.com/pivotal-landscape."
	if h := Detect(text); len(h) > 0 {
		t.Fatalf("masked text produced hits: %+v", h)
	}
}

func TestLineNumbers(t *testing.T) {
	h := Detect("Fine line.\n\nAnother fine line.\n\nLet's dive in.")
	if len(h) != 1 || h[0].Line != 5 {
		t.Fatalf("want one hit on line 5, got %+v", h)
	}
}

func TestAssessBaseline(t *testing.T) {
	text := strings.Repeat("I went home — then I slept. ", 4) // 4 dashes, ~24 words
	hits := Detect(text)

	// a writer who never uses dashes: every one is excess
	a := Assess(hits, 100, nil)
	if f := find(a, "em_dash"); f == nil || f.Excess != 4 || f.Penalty != 12 {
		t.Fatalf("no baseline: %+v", a)
	}

	// a writer who uses them a lot: no penalty
	a = Assess(hits, 100, map[string]Baseline{"em_dash": {Rate: 40, DocFrac: 1}})
	if a.Penalty != 0 {
		t.Fatalf("dash-heavy writer was penalised: %+v", a)
	}
}

func TestAssessWeakAlone(t *testing.T) {
	hits := Detect("We sell apples, pears, and plums.")
	if a := Assess(hits, 50, nil); a.Penalty != 0 {
		t.Fatalf("a lone weak tell should cost nothing, got %+v", a)
	}
	hits = Detect("We sell apples, pears, and plums. Let's dive in.")
	a := Assess(hits, 50, nil)
	if f := find(a, "triad"); f == nil || f.Penalty == 0 {
		t.Fatalf("weak tell in company should cost points: %+v", a)
	}
}

func TestAssessHard(t *testing.T) {
	a := Assess(Detect("Dear [Recipient Name], thanks."), 50, map[string]Baseline{"placeholder": {Rate: 100, DocFrac: 1}})
	if !a.HardFail {
		t.Fatalf("hard tells ignore the baseline: %+v", a)
	}
}

func find(a Assessment, id string) *Finding {
	for i := range a.Findings {
		if a.Findings[i].Rule == id {
			return &a.Findings[i]
		}
	}
	return nil
}
