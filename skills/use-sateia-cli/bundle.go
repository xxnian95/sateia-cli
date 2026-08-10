package usesateiacli

import "embed"

// Files contains the exact agent skill shipped with this CLI build.
//
//go:embed SKILL.md agents/openai.yaml
var Files embed.FS

var Paths = []string{"SKILL.md", "agents/openai.yaml"}
