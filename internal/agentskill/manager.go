package agentskill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skillbundle "github.com/xxnian95/sateia-cli/skills/use-sateia-cli"
)

const (
	manifestName = ".sateia-skill-manifest.json"
	skillName    = "use-sateia-cli"
)

type State string

const (
	StateCurrent   State = "CURRENT"
	StateMissing   State = "MISSING"
	StateOutdated  State = "OUTDATED"
	StateModified  State = "MODIFIED"
	StateUnmanaged State = "UNMANAGED"
	StateInvalid   State = "INVALID"
)

type Status struct {
	State            State  `json:"state"`
	Target           string `json:"target"`
	BundleHash       string `json:"bundle_hash"`
	InstalledHash    string `json:"installed_hash,omitempty"`
	InstalledVersion string `json:"installed_cli_version,omitempty"`
	Message          string `json:"message"`
}

type manifest struct {
	SchemaVersion int               `json:"schema_version"`
	CLIversion    string            `json:"cli_version"`
	BundleHash    string            `json:"bundle_hash"`
	Files         map[string]string `json:"files"`
}

func DefaultTarget() (string, error) {
	root := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home for default skill target: %w", err)
		}
		root = filepath.Join(home, ".codex")
	}
	return filepath.Join(root, "skills", skillName), nil
}

func ResolveTarget(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return DefaultTarget()
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve skill target: %w", err)
	}
	if filepath.Base(filepath.Clean(absolute)) != skillName {
		return "", fmt.Errorf("skill target must end with %q", skillName)
	}
	return absolute, nil
}

func Check(rawTarget string) (Status, error) {
	target, err := ResolveTarget(rawTarget)
	if err != nil {
		return Status{}, err
	}
	bundle, err := bundledManifest("")
	if err != nil {
		return Status{}, err
	}
	status := Status{Target: target, BundleHash: bundle.BundleHash}
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		status.State = StateMissing
		status.Message = "The bundled Sateia skill is not installed at this target."
		return status, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("inspect skill target: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		status.State = StateInvalid
		status.Message = "The skill target is a symbolic link; automatic installation and update are blocked."
		return status, nil
	}
	if !info.IsDir() {
		status.State = StateInvalid
		status.Message = "The skill target exists but is not a directory."
		return status, nil
	}

	installed, err := readManifest(target)
	if errors.Is(err, fs.ErrNotExist) {
		status.State = StateUnmanaged
		status.Message = "The target has no Sateia installation manifest; it will not be overwritten automatically."
		return status, nil
	}
	if err != nil {
		status.State = StateInvalid
		status.Message = err.Error()
		return status, nil
	}
	status.InstalledHash = installed.BundleHash
	status.InstalledVersion = installed.CLIversion

	unchanged, err := filesMatchManifest(target, installed)
	if err != nil {
		return Status{}, err
	}
	if !unchanged {
		status.State = StateModified
		status.Message = "Installed skill files differ from their installation manifest; automatic update is blocked."
		return status, nil
	}
	if installed.BundleHash != bundle.BundleHash {
		status.State = StateOutdated
		status.Message = "The installed skill is unchanged but differs from the bundle in this CLI."
		return status, nil
	}
	status.State = StateCurrent
	status.Message = "The installed skill matches this CLI bundle."
	return status, nil
}

func Install(rawTarget, cliVersion string, force bool) (Status, error) {
	status, err := Check(rawTarget)
	if err != nil {
		return Status{}, err
	}
	if status.State != StateMissing && !force {
		return status, fmt.Errorf("skill target is %s; install requires an empty target or --force", status.State)
	}
	if err := writeBundle(status.Target, cliVersion); err != nil {
		return Status{}, err
	}
	return Check(status.Target)
}

func Update(rawTarget, cliVersion string, force bool) (Status, error) {
	status, err := Check(rawTarget)
	if err != nil {
		return Status{}, err
	}
	switch status.State {
	case StateCurrent:
		return status, nil
	case StateMissing, StateOutdated:
		// Missing files and unmodified managed installs are safe automatic update targets.
	case StateModified, StateUnmanaged, StateInvalid:
		if !force {
			return status, fmt.Errorf("skill target is %s; refusing to overwrite it without --force", status.State)
		}
	default:
		return status, fmt.Errorf("unsupported skill state %q", status.State)
	}
	if err := writeBundle(status.Target, cliVersion); err != nil {
		return Status{}, err
	}
	return Check(status.Target)
}

func bundledManifest(cliVersion string) (manifest, error) {
	files := make(map[string]string, len(skillbundle.Paths))
	for _, path := range skillbundle.Paths {
		content, err := skillbundle.Files.ReadFile(path)
		if err != nil {
			return manifest{}, fmt.Errorf("read bundled skill file %s: %w", path, err)
		}
		files[path] = hashBytes(content)
	}
	return manifest{
		SchemaVersion: 1,
		CLIversion:    cliVersion,
		BundleHash:    hashFileMap(files),
		Files:         files,
	}, nil
}

func writeBundle(target, cliVersion string) error {
	// Refuse symlinked managed directories so a local update cannot escape its reviewed target.
	for _, path := range []string{target, filepath.Join(target, "agents")} {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed skill path %s must not be a symbolic link", path)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect managed skill path %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(target, "agents"), 0o755); err != nil {
		return fmt.Errorf("create skill target: %w", err)
	}
	installed, err := bundledManifest(cliVersion)
	if err != nil {
		return err
	}
	for _, path := range skillbundle.Paths {
		content, readErr := skillbundle.Files.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read bundled skill file %s: %w", path, readErr)
		}
		if writeErr := atomicWriteFile(filepath.Join(target, filepath.FromSlash(path)), content, 0o644); writeErr != nil {
			return fmt.Errorf("install skill file %s: %w", path, writeErr)
		}
	}
	manifestData, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return fmt.Errorf("encode skill manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := atomicWriteFile(filepath.Join(target, manifestName), manifestData, 0o644); err != nil {
		return fmt.Errorf("install skill manifest: %w", err)
	}
	return nil
}

func readManifest(target string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(target, manifestName))
	if err != nil {
		return manifest{}, err
	}
	var installed manifest
	if err := json.Unmarshal(data, &installed); err != nil {
		return manifest{}, fmt.Errorf("decode skill installation manifest: %w", err)
	}
	if installed.SchemaVersion != 1 || installed.BundleHash == "" || len(installed.Files) == 0 {
		return manifest{}, errors.New("skill installation manifest is incomplete or unsupported")
	}
	if len(installed.Files) != len(skillbundle.Paths) {
		return manifest{}, errors.New("skill installation manifest contains an unexpected file set")
	}
	for _, path := range skillbundle.Paths {
		digest, exists := installed.Files[path]
		if !exists || !validSHA256(digest) {
			return manifest{}, fmt.Errorf("skill installation manifest has an invalid hash for %s", path)
		}
	}
	if !validSHA256(installed.BundleHash) || hashFileMap(installed.Files) != installed.BundleHash {
		return manifest{}, errors.New("skill installation manifest bundle hash does not match its file hashes")
	}
	return installed, nil
}

func filesMatchManifest(target string, installed manifest) (bool, error) {
	for path, expectedHash := range installed.Files {
		content, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("read installed skill file %s: %w", path, err)
		}
		if hashBytes(content) != expectedHash {
			return false, nil
		}
	}
	return true, nil
}

func hashFileMap(files map[string]string) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		_, _ = hash.Write([]byte(path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(files[path]))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func hashBytes(content []byte) string {
	value := sha256.Sum256(content)
	return hex.EncodeToString(value[:])
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func atomicWriteFile(path string, content []byte, mode fs.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".sateia-skill-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryPath, path)
}

func replaceFile(temporaryPath, targetPath string) error {
	backup, err := os.CreateTemp(filepath.Dir(targetPath), ".sateia-backup-*")
	if err != nil {
		return err
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		return err
	}
	hasBackup := false
	if err := os.Rename(targetPath, backupPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		hasBackup = true
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		if hasBackup {
			if restoreErr := os.Rename(backupPath, targetPath); restoreErr != nil {
				return fmt.Errorf("replace skill file: %w; restore original from %s: %v", err, backupPath, restoreErr)
			}
		}
		return err
	}
	if hasBackup {
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("remove replaced skill file backup: %w", err)
		}
	}
	return nil
}
