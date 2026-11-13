// Package capture copies a program's sign-in out of its live files and
// keychain entries into a Snapshot, and writes a Snapshot back.
package capture

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"
)

// Kinds of captured items.
const (
	KindAbsent = "absent"
	KindFile   = "file"
	KindDir    = "dir"
	KindJSON   = "json"
)

// Snapshot is one profile's copy of a program's sign-in.
type Snapshot struct {
	Version  int       `json:"v"`
	App      string    `json:"app"`
	Captured time.Time `json:"captured"`
	Items    []Item    `json:"items"`
	Secrets  []Secret  `json:"secrets,omitempty"`
}

// Item is the captured state of one FileSpec.
type Item struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Mode uint32 `json:"mode,omitempty"`
	// Data is a file's content.
	Data []byte `json:"data,omitempty"`
	// Entries are a folder's content, sorted by path.
	Entries []Entry `json:"entries,omitempty"`
	// Keys are the captured top-level keys of a JSON file.
	Keys map[string]json.RawMessage `json:"keys,omitempty"`
}

// Entry is one file, folder or symlink inside a captured folder.
type Entry struct {
	Path string `json:"p"` // slash-separated, relative
	Mode uint32 `json:"m"`
	Dir  bool   `json:"d,omitempty"`
	Link string `json:"l,omitempty"`
	Data []byte `json:"b,omitempty"`
}

// Secret is the captured value of one KeychainSpec.
type Secret struct {
	ID      string `json:"id"`
	Present bool   `json:"present"`
	Value   []byte `json:"value,omitempty"`
	// Unavailable is set when the system has no credential store, so
	// the entry is neither read nor written.
	Unavailable bool `json:"unavailable,omitempty"`
}

const snapshotVersion = 1

// Empty reports whether the snapshot holds no sign-in at all.
func (s *Snapshot) Empty() bool {
	for _, it := range s.Items {
		switch it.Kind {
		case KindAbsent:
		case KindJSON:
			if len(it.Keys) > 0 {
				return false
			}
		default:
			return false
		}
	}
	for _, sec := range s.Secrets {
		if sec.Present {
			return false
		}
	}
	return true
}

// Hash fingerprints the sign-in content (not the capture time), so two
// captures of an unchanged sign-in hash the same.
func (s *Snapshot) Hash() string {
	data, _ := json.Marshal(struct {
		Items   []Item
		Secrets []Secret
	}{s.Items, s.Secrets})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Size is the snapshot's content size in bytes.
func (s *Snapshot) Size() int64 {
	var n int64
	for _, it := range s.Items {
		n += int64(len(it.Data))
		for _, e := range it.Entries {
			n += int64(len(e.Data))
		}
		for _, v := range it.Keys {
			n += int64(len(v))
		}
	}
	for _, sec := range s.Secrets {
		n += int64(len(sec.Value))
	}
	return n
}

// Encode serializes and compresses the snapshot.
func (s *Snapshot) Encode() ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(s); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode reverses Encode.
func Decode(data []byte) (*Snapshot, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Blank returns a signed-out snapshot for app: restoring it removes the
// sign-in, so the user can sign in fresh into a new profile.
func Blank(app string, itemIDs []string, jsonIDs map[string]bool, secretIDs []string) *Snapshot {
	s := &Snapshot{Version: snapshotVersion, App: app, Captured: time.Now()}
	for _, id := range itemIDs {
		kind := KindAbsent
		if jsonIDs[id] {
			kind = KindJSON
		}
		s.Items = append(s.Items, Item{ID: id, Kind: kind})
	}
	for _, id := range secretIDs {
		s.Secrets = append(s.Secrets, Secret{ID: id})
	}
	return s
}
