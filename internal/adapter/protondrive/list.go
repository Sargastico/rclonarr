package protondrive

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

func (u *Uploader) ListFiles(ctx context.Context, remoteDir string) ([]port.RemoteFile, error) {
	remoteDir = normalizeRemoteDir(remoteDir)
	if remoteDir == "" {
		return nil, fmt.Errorf("remote directory is empty")
	}

	out, err := u.exec(ctx, "filesystem", "list", "--type", "file", "--json", remoteDir)
	if err != nil {
		if looksLikeAuthRequired(out, err) {
			if authErr := u.EnsureAuth(ctx); authErr != nil {
				return nil, authErr
			}
			out, err = u.exec(ctx, "filesystem", "list", "--type", "file", "--json", remoteDir)
		}
		if err != nil {
			return nil, fmt.Errorf("proton-drive list %q: %w\n%s", remoteDir, err, strings.TrimSpace(string(out)))
		}
	}

	files, err := parseListJSON(out, remoteDir)
	if err != nil {
		return nil, fmt.Errorf("parse proton-drive list %q: %w", remoteDir, err)
	}
	return files, nil
}

func (u *Uploader) Trash(ctx context.Context, remotePaths ...string) error {
	if len(remotePaths) == 0 {
		return nil
	}

	args := append([]string{"filesystem", "trash"}, remotePaths...)
	out, err := u.exec(ctx, args...)
	if err != nil {
		if looksLikeAuthRequired(out, err) {
			if authErr := u.EnsureAuth(ctx); authErr != nil {
				return authErr
			}
			out, err = u.exec(ctx, args...)
		}
		if err != nil {
			return fmt.Errorf("proton-drive trash: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func parseListJSON(data []byte, parentDir string) ([]port.RemoteFile, error) {
	data = bytesTrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	rawEntries, err := decodeListEntries(data)
	if err != nil {
		return nil, err
	}

	files := make([]port.RemoteFile, 0, len(rawEntries))
	for _, raw := range rawEntries {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entry); err != nil {
			continue
		}

		typ := strings.ToLower(unwrapString(entry["type"]))
		if typ != "" && typ != "file" {
			continue
		}

		name := unwrapString(entry["name"])
		if name == "" {
			continue
		}

		mod := unwrapTime(entry["modificationTime"])
		files = append(files, port.RemoteFile{
			Name:    name,
			Path:    path.Join(parentDir, name),
			ModTime: mod,
		})
	}
	return files, nil
}

func decodeListEntries(data []byte) ([]json.RawMessage, error) {
	// Top-level array
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}

	// Wrapped object variants
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	for _, key := range []string{"value", "items", "nodes", "entries", "data"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		// { ok: true, value: [...] }
		var innerArr []json.RawMessage
		if err := json.Unmarshal(raw, &innerArr); err == nil {
			return innerArr, nil
		}
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapped); err == nil {
			if v, ok := wrapped["value"]; ok {
				if err := json.Unmarshal(v, &innerArr); err == nil {
					return innerArr, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("unrecognized list json shape")
}

func unwrapString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var w struct {
		OK    bool   `json:"ok"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &w); err == nil {
		return w.Value
	}
	return ""
}

func unwrapTime(raw json.RawMessage) time.Time {
	s := unwrapString(raw)
	if s == "" {
		return time.Time{}
	}
	if at, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return at.UTC()
	}
	if at, err := time.Parse(time.RFC3339, s); err == nil {
		return at.UTC()
	}
	return time.Time{}
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
