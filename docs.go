package main

import (
	"embed"

	"github.com/kocieusz/ghostwriter/internal/skill"
)

// docsFS is the manual in docs/. It is printed by `ghostwriter docs` and copied
// into every installed skill's references/, so an agent learns the whole tool
// from the binary alone.
//
//go:embed docs/*.md
var docsFS embed.FS

// docTopics maps each `ghostwriter docs` topic to its content.
func docTopics() map[string]string {
	read := func(name string) string {
		b, err := docsFS.ReadFile("docs/" + name)
		if err != nil {
			panic(err) // embedded at build time; missing means a broken build
		}
		return string(b)
	}
	return map[string]string{
		"cli":     read("cli.md"),
		"scoring": read("scoring.md"),
		"tells":   skill.TellsReference(),
	}
}

type docsCmd struct {
	Topic string `arg:"" optional:"" enum:"cli,scoring,tells" default:"cli" help:"cli (commands), scoring (reading a score), or tells (AI-tell rules)."`
}

func (d *docsCmd) Run() error {
	_, err := stdout.WriteString(docTopics()[d.Topic])
	return err
}
