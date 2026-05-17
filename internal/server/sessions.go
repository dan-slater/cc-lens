package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionsRoot returns the directory Claude Code writes per-session
// transcripts to. Override with CC_LENS_PROJECTS_DIR for tests.
func SessionsRoot() string {
	if v := os.Getenv("CC_LENS_PROJECTS_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// DiskSession is the on-disk view of a session, derived only from filesystem
// state. We never copy transcript contents into our own store.
type DiskSession struct {
	ID              string    `json:"id"`
	TranscriptPath  string    `json:"transcript_path"`
	TranscriptBytes int64     `json:"transcript_bytes"`
	ModifiedAt      time.Time `json:"modified_at"`
	CWD             string    `json:"cwd,omitempty"`
}

// DiscoverSessions walks ~/.claude/projects looking for *.jsonl files. The
// session ID is the filename without extension. CWD is derived from the
// encoded parent directory name (Claude Code replaces "/" with "-").
func DiscoverSessions() ([]DiskSession, error) {
	root := SessionsRoot()
	if root == "" {
		return nil, errors.New("could not resolve sessions root")
	}
	var out []DiskSession
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fs.SkipDir
			}
			return nil // tolerate transient errors
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		id := strings.TrimSuffix(d.Name(), ".jsonl")
		cwd := decodeProjectDir(filepath.Base(filepath.Dir(path)))
		out = append(out, DiskSession{
			ID:              id,
			TranscriptPath:  path,
			TranscriptBytes: info.Size(),
			ModifiedAt:      info.ModTime().UTC(),
			CWD:             cwd,
		})
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ModifiedAt.After(out[j].ModifiedAt)
	})
	return out, nil
}

// decodeProjectDir reverses Claude Code's path encoding. The encoding is
// lossy ("/" and "-" both become "-"), so we treat the result as a hint only.
func decodeProjectDir(name string) string {
	if name == "" {
		return ""
	}
	return strings.ReplaceAll(name, "-", "/")
}

// TranscriptLine is a permissive view of one JSONL line. Unknown fields are
// preserved in Raw so callers can introspect.
type TranscriptLine struct {
	Type      string          `json:"type,omitempty"`
	UUID      string          `json:"uuid,omitempty"`
	Parent    string          `json:"parentUuid,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Raw       json.RawMessage `json:"raw"`
}

// ReadTranscript reads up to `limit` lines from the end of the transcript
// file. If `before` is non-empty, returns lines whose UUID precedes it (for
// reverse pagination). Limit <= 0 means "all".
func ReadTranscript(path string, limit int, before string) ([]TranscriptLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var all []TranscriptLine
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var tl TranscriptLine
		_ = json.Unmarshal(line, &tl)
		tl.Raw = append(json.RawMessage(nil), line...)
		all = append(all, tl)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if before != "" {
		for i, l := range all {
			if l.UUID == before {
				all = all[:i]
				break
			}
		}
	}
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}
