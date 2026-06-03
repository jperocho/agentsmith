// Package manifest reads and writes ~/.agentsmith/skills.json, the version
// manifest tracking every hub skill and its installs (plan section 4).
//
// All writes are atomic (temp file -> fsync -> rename) and the caller is
// expected to hold the hub lock during read-modify-write cycles.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SchemaVersion is the current skills.json schema version.
const SchemaVersion = 1

// Install records one skill installation into an agent.
type Install struct {
	Agent           string `json:"agent"`           // adapter name, e.g. "claude-code"
	Scope           string `json:"scope"`           // "global" | "project"
	Target          string `json:"target"`          // canonical absolute path written
	Mode            string `json:"mode"`            // "symlink" | "copy"
	InstalledCommit string `json:"installedCommit"` // for copy-mode drift detection
	InstalledAt     string `json:"installedAt"`     // RFC3339
}

// Tracking classifies how a skill follows its source.
const (
	TrackBranch = "branch" // follows a moving branch; update pulls new commits
	TrackPinned = "pinned" // pinned to a tag; update only moves if the tag moved
)

// Skill is a single tracked skill in the hub.
type Skill struct {
	Repo         string    `json:"repo"`                   // canonical clone URL
	ResolvedRef  string    `json:"resolvedRef"`            // tag/branch requested
	Tracking     string    `json:"tracking"`               // "branch" | "pinned"
	Commit       string    `json:"commit"`                 // exact pinned commit
	FetchedAt    string    `json:"fetchedAt"`              // RFC3339
	Path         string    `json:"path"`                   // relative to hub root
	Checksum     string    `json:"checksum"`               // "sha256:..." of content tree
	AllowScripts bool      `json:"allowScripts,omitempty"` // keep executable bits in hub clone
	Installs     []Install `json:"installs"`
}

// Manifest is the top-level skills.json document.
type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	Skills        map[string]*Skill `json:"skills"`

	path string // source path, set by Load; used by Save
}

// ErrSkillNotFound is returned when a named skill is absent from the manifest.
var ErrSkillNotFound = errors.New("skill not found in manifest")

// Load reads the manifest at path. A missing file yields an empty manifest
// (not an error) so first-run is seamless.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{SchemaVersion: SchemaVersion, Skills: map[string]*Skill{}, path: path}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if m.Skills == nil {
		m.Skills = map[string]*Skill{}
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = SchemaVersion
	}
	if m.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("manifest schema v%d newer than supported v%d; upgrade agentsmith", m.SchemaVersion, SchemaVersion)
	}
	m.path = path
	return &m, nil
}

// Save writes the manifest atomically (temp -> fsync -> rename) with 0600 perms.
func (m *Manifest) Save() error {
	if m.path == "" {
		return errors.New("manifest has no path; load it via Load")
	}
	m.SchemaVersion = SchemaVersion
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(m.path)
	tmp, err := os.CreateTemp(dir, ".skills-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp manifest: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp manifest: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp manifest: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fsync temp manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp manifest: %w", err)
	}
	if err := os.Rename(tmpName, m.path); err != nil {
		return fmt.Errorf("rename temp manifest: %w", err)
	}
	return nil
}

// Get returns the named skill or ErrSkillNotFound.
func (m *Manifest) Get(name string) (*Skill, error) {
	s, ok := m.Skills[name]
	if !ok {
		return nil, fmt.Errorf("%q: %w", name, ErrSkillNotFound)
	}
	return s, nil
}

// Names returns skill names sorted alphabetically.
func (m *Manifest) Names() []string {
	names := make([]string, 0, len(m.Skills))
	for n := range m.Skills {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Now returns the current time formatted RFC3339 in UTC, for manifest fields.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// FindInstall returns the index of an install matching agent+scope, or -1.
func (s *Skill) FindInstall(agent, scope string) int {
	for i, in := range s.Installs {
		if in.Agent == agent && in.Scope == scope {
			return i
		}
	}
	return -1
}
