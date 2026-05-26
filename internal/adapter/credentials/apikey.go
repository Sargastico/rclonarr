package credentials

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ServarrAPIKey reads <ApiKey> from config.xml under the *arr config directory.
func ServarrAPIKey(configDir string) (string, error) {
	path := filepath.Join(configDir, "config.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read servarr config.xml: %w", err)
	}

	var doc struct {
		APIKey string `xml:"ApiKey"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse servarr config.xml: %w", err)
	}

	key := strings.TrimSpace(doc.APIKey)
	if key == "" {
		return "", fmt.Errorf("empty ApiKey in %s", path)
	}
	return key, nil
}

var bazarrAPIKeyRE = regexp.MustCompile(`(?m)^\s*apikey:\s*['"]?([^'"\s#]+)`)

// BazarrAPIKey reads auth.apikey from config.yaml next to the Bazarr config tree.
// backupMount is usually .../bazarr/backup; config.yaml lives in the parent directory.
func BazarrAPIKey(backupMount string) (string, error) {
	dir := strings.TrimSpace(backupMount)
	if strings.HasSuffix(filepath.ToSlash(dir), "/backup") {
		dir = filepath.Dir(dir)
	}

	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read bazarr config.yaml: %w", err)
	}

	m := bazarrAPIKeyRE.FindSubmatch(data)
	if len(m) < 2 {
		return "", fmt.Errorf("apikey not found in %s", path)
	}
	return strings.TrimSpace(string(m[1])), nil
}
