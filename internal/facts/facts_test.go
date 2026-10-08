package facts

import (
	"slices"
	"testing"
)

func TestCompare(t *testing.T) {
	src := "Meeting with Anna Nowak on 12 March. Budget is 4,500 PLN. She said \"we need it done by spring\"."
	draft := "So I met Anna on 12 March. She mentioned a budget of 5000 PLN, and Piotr Wiśniewski will join. " +
		"Her words: \"we need it done by spring\". Also \"this is the best plan we ever had\"."
	r := Compare(src, draft)
	if !slices.Equal(r.AddedNumbers, []string{"5000"}) {
		t.Errorf("added numbers %q", r.AddedNumbers)
	}
	if !slices.Equal(r.DroppedNumbers, []string{"4500"}) {
		t.Errorf("dropped numbers %q", r.DroppedNumbers)
	}
	if !slices.Contains(r.AddedNames, "Piotr Wiśniewski") || slices.Contains(r.AddedNames, "Anna") {
		t.Errorf("added names %q", r.AddedNames)
	}
	if len(r.AddedQuotes) != 1 || r.AddedQuotes[0] != "this is the best plan we ever had" {
		t.Errorf("added quotes %q", r.AddedQuotes)
	}
	if !r.Fabricated() || r.Penalty() == 0 {
		t.Error("should count as fabricated")
	}
}

func TestCompareClean(t *testing.T) {
	src := "Lunch with Marek on Friday at 1pm, his treat."
	draft := "Lunch with Marek on Friday at 1pm! He's paying, so I'm in."
	if r := Compare(src, draft); r.Fabricated() || len(r.AddedNames) > 0 || r.Penalty() != 0 {
		t.Fatalf("clean rewrite flagged: %+v", r)
	}
}
