package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxnian95/sateia-cli/internal/agentskill"
	"github.com/xxnian95/sateia-cli/internal/updatecheck"
)

func TestSkillInstallAndCheckCommandsUseEmbeddedBundle(t *testing.T) {
	codexRoot := t.TempDir()
	t.Setenv("CODEX_HOME", codexRoot)
	t.Setenv("SATEIA_NO_UPDATE_NOTIFIER", "1")
	checker := stubUpdateChecker{check: func(context.Context, string) (*updatecheck.Available, error) {
		return nil, nil
	}}

	var installOutput bytes.Buffer
	install := newWithDependencies("v1.2.3", strings.NewReader(""), &installOutput, &installOutput, stubCredentialStore{}, checker)
	install.SetArgs([]string{"skill", "install", "--json"})
	if err := install.Execute(); err != nil {
		t.Fatal(err)
	}
	var installed struct {
		Skill agentskill.Status `json:"skill"`
	}
	if err := json.Unmarshal(installOutput.Bytes(), &installed); err != nil {
		t.Fatal(err)
	}
	if installed.Skill.State != agentskill.StateCurrent || installed.Skill.InstalledVersion != "v1.2.3" {
		t.Fatalf("unexpected installed skill: %#v", installed.Skill)
	}
	for _, path := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml"), ".sateia-skill-manifest.json"} {
		if _, err := os.Stat(filepath.Join(codexRoot, "skills", "use-sateia-cli", path)); err != nil {
			t.Errorf("missing installed file %s: %v", path, err)
		}
	}

	var checkOutput bytes.Buffer
	check := newWithDependencies("v1.2.3", strings.NewReader(""), &checkOutput, &checkOutput, stubCredentialStore{}, checker)
	check.SetArgs([]string{"skill", "check", "--json"})
	if err := check.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(checkOutput.String(), `"state": "CURRENT"`) {
		t.Fatalf("unexpected check output: %s", checkOutput.String())
	}
}
