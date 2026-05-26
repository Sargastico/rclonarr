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

// Bazarr auth.apikey (not Sonarr/Radarr integration keys further down in the same file).
var bazarrAuthAPIKeyRE = regexp.MustCompile(`(?ms)^auth:\s*\n\s*apikey:\s*([^\s'"]+)`)

var bazarrAnyAPIKeyRE = regexp.MustCompile(`(?m)^\s*apikey:\s*['"]?([^'"\s#]+)`)

// BazarrAPIKey reads auth.apikey from config.yaml under the Bazarr config tree.
// backupMount is usually .../bazarr/backup (linuxserver stores zips there); config.yaml is
// at .../bazarr/config.yaml or .../bazarr/config/config.yaml.
func BazarrAPIKey(backupMount string) (string, error) {
	root := bazarrConfigRoot(backupMount)
	candidates := []string{
		filepath.Join(root, "config.yaml"),
		filepath.Join(root, "config", "config.yaml"),
	}
	if extra, err := findBazarrConfigYAML(root); err == nil {
		candidates = append(candidates, extra)
	}
	var readErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			readErr = err
			continue
		}
		if key := parseBazarrAPIKey(data); key != "" {
			return key, nil
		}
		return "", fmt.Errorf("apikey not found in %s", path)
	}
	return "", fmt.Errorf("read bazarr config.yaml under %s: %w", root, readErr)
}

func findBazarrConfigYAML(root string) (string, error) {
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && filepath.Base(path) == "backup" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "config.yaml" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("config.yaml not found under %s", root)
	}
	return found, nil
}

func parseBazarrAPIKey(data []byte) string {
	if m := bazarrAuthAPIKeyRE.FindSubmatch(data); len(m) >= 2 {
		return strings.TrimSpace(string(m[1]))
	}
	for _, m := range bazarrAnyAPIKeyRE.FindAllSubmatch(data, -1) {
		if len(m) < 2 {
			continue
		}
		key := strings.TrimSpace(string(m[1]))
		if key != "" && key != "''" {
			return key
		}
	}
	return ""
}

func bazarrConfigRoot(backupMount string) string {
	dir := strings.TrimSpace(backupMount)
	if strings.HasSuffix(filepath.ToSlash(dir), "/backup") {
		return filepath.Dir(dir)
	}
	return dir
}
