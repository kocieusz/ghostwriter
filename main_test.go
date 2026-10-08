package main

import (
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// TestManualCoversEveryCommandAndFlag keeps docs/cli.md in step with the CLI:
// the manual ships inside every installed skill, so an undocumented flag is
// one the agent will never know about.
func TestManualCoversEveryCommandAndFlag(t *testing.T) {
	k, err := kong.New(&cli{}, kong.Name("ghostwriter"), kong.Vars{"version": "test"})
	if err != nil {
		t.Fatal(err)
	}
	manual := docTopics()["cli"]
	for _, n := range k.Model.Leaves(true) {
		path := commandPath(n)
		if !strings.Contains(manual, "\n## "+path+"\n") {
			t.Errorf("docs/cli.md has no section %q", "## "+path)
		}
		for _, group := range n.AllFlags(true) {
			for _, f := range group {
				if f.Name == "help" {
					continue
				}
				if !strings.Contains(manual, "--"+f.Name) {
					t.Errorf("docs/cli.md never mentions --%s (%s)", f.Name, path)
				}
				if f.Short != 0 && !strings.Contains(manual, "`-"+string(f.Short)) {
					t.Errorf("docs/cli.md never mentions -%c (%s)", f.Short, path)
				}
			}
		}
	}
}

func TestDocTopics(t *testing.T) {
	for topic, body := range docTopics() {
		if len(body) < 500 {
			t.Errorf("topic %q looks empty", topic)
		}
	}
}

// commandPath is "prompt style" for a nested command, without aliases.
func commandPath(n *kong.Node) string {
	var parts []string
	for ; n != nil && n.Parent != nil; n = n.Parent {
		parts = append([]string{n.Name}, parts...)
	}
	return strings.Join(parts, " ")
}
