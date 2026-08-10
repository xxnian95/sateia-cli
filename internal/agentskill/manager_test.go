package agentskill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCheckAndProtectModifiedSkill(t *testing.T) {
	target := filepath.Join(t.TempDir(), skillName)

	missing, err := Check(target)
	if err != nil {
		t.Fatal(err)
	}
	if missing.State != StateMissing {
		t.Fatalf("unexpected missing state: %#v", missing)
	}

	installed, err := Install(target, "v1.2.3", false)
	if err != nil {
		t.Fatal(err)
	}
	if installed.State != StateCurrent || installed.InstalledVersion != "v1.2.3" {
		t.Fatalf("unexpected installed state: %#v", installed)
	}

	skillPath := filepath.Join(target, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("user-modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modified, err := Check(target)
	if err != nil {
		t.Fatal(err)
	}
	if modified.State != StateModified {
		t.Fatalf("unexpected modified state: %#v", modified)
	}
	if _, err := Update(target, "v1.2.4", false); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("unexpected protected update error: %v", err)
	}

	restored, err := Update(target, "v1.2.4", true)
	if err != nil {
		t.Fatal(err)
	}
	if restored.State != StateCurrent || restored.InstalledVersion != "v1.2.4" {
		t.Fatalf("unexpected forced update state: %#v", restored)
	}
}

func TestUpdateAcceptsUnmodifiedOutdatedManifest(t *testing.T) {
	target := filepath.Join(t.TempDir(), skillName)
	if _, err := Install(target, "v1.0.0", false); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(target, manifestName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var installed manifest
	if err := json.Unmarshal(data, &installed); err != nil {
		t.Fatal(err)
	}
	for path := range installed.Files {
		content := []byte("older bundled content for " + path + "\n")
		if err := os.WriteFile(filepath.Join(target, filepath.FromSlash(path)), content, 0o644); err != nil {
			t.Fatal(err)
		}
		installed.Files[path] = hashBytes(content)
	}
	installed.BundleHash = hashFileMap(installed.Files)
	data, err = json.MarshalIndent(installed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Check(target)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateOutdated {
		t.Fatalf("unexpected outdated state: %#v", status)
	}
	updated, err := Update(target, "v2.0.0", false)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != StateCurrent || updated.InstalledVersion != "v2.0.0" {
		t.Fatalf("unexpected updated state: %#v", updated)
	}
}

func TestResolveTargetRequiresSkillDirectoryName(t *testing.T) {
	if _, err := ResolveTarget(t.TempDir()); err == nil || !strings.Contains(err.Error(), skillName) {
		t.Fatalf("unexpected target error: %v", err)
	}
}

func TestInstallRejectsSymlinkedTarget(t *testing.T) {
	root := t.TempDir()
	realTarget := filepath.Join(root, "real")
	if err := os.Mkdir(realTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, skillName)
	if err := os.Symlink(realTarget, target); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	status, err := Check(target)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateInvalid {
		t.Fatalf("unexpected symlink state: %#v", status)
	}
	if _, err := Install(target, "v1.0.0", true); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("unexpected symlink install error: %v", err)
	}
}
