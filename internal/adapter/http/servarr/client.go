package servarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

const (
	apiPrefix       = "/api/v3"
	commandBackup   = "Backup"
	statusCompleted = "completed"
	statusFailed    = "failed"
)

type Client struct {
	httpClient *http.Client
}

var _ port.ArrBackupTrigger = (*Client)(nil)

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{httpClient: httpClient}
}

func (c *Client) Trigger(ctx context.Context, target models.BackupTarget) (string, error) {
	if target.APIScheme != models.APISchemeServarrV3 {
		return "", fmt.Errorf("servarr client: unsupported api scheme %q", target.APIScheme)
	}

	before, err := c.listBackups(ctx, target)
	if err != nil {
		return "", err
	}

	cmd, err := c.postCommand(ctx, target, commandBackup)
	if err != nil {
		return "", err
	}

	if err := c.waitForCommand(ctx, target, cmd.ID); err != nil {
		return "", err
	}

	after, err := c.listBackups(ctx, target)
	if err != nil {
		return "", err
	}

	backup := newestBackup(after, before)
	if backup == nil {
		return "", fmt.Errorf("servarr backup not found after command completed")
	}

	return resolveLocalPath(target, backup.Path)
}

type commandRequest struct {
	Name string `json:"name"`
}

type commandResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type backupResource struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Time string `json:"time"`
}

func (c *Client) postCommand(ctx context.Context, target models.BackupTarget, name string) (*commandResponse, error) {
	body, err := json.Marshal(commandRequest{Name: name})
	if err != nil {
		return nil, err
	}

	var resp commandResponse
	if err := c.doJSON(ctx, target, http.MethodPost, apiPrefix+"/command", body, &resp); err != nil {
		return nil, fmt.Errorf("post backup command: %w", err)
	}

	return &resp, nil
}

func (c *Client) waitForCommand(ctx context.Context, target models.BackupTarget, commandID int64) error {
	poll := time.Duration(config.App.CommandPollIntervalMS) * time.Millisecond
	if poll <= 0 {
		poll = time.Second
	}

	timeout := time.Duration(config.App.CommandTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	deadline := time.Now().Add(timeout)
	uri := fmt.Sprintf("%s/command/%d", apiPrefix, commandID)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("backup command %d timed out after %s", commandID, timeout)
		}

		var resp commandResponse
		if err := c.doJSON(ctx, target, http.MethodGet, uri, nil, &resp); err != nil {
			return fmt.Errorf("poll command %d: %w", commandID, err)
		}

		switch strings.ToLower(resp.Status) {
		case statusCompleted:
			return nil
		case statusFailed:
			return fmt.Errorf("backup command %d failed", commandID)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (c *Client) listBackups(ctx context.Context, target models.BackupTarget) ([]backupResource, error) {
	var backups []backupResource
	if err := c.doJSON(ctx, target, http.MethodGet, apiPrefix+"/system/backup", nil, &backups); err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	return backups, nil
}

func newestBackup(all []backupResource, before []backupResource) *backupResource {
	seen := make(map[int]struct{}, len(before))
	for _, b := range before {
		seen[b.ID] = struct{}{}
	}

	var newest *backupResource
	for i := range all {
		b := all[i]
		if _, exists := seen[b.ID]; exists {
			continue
		}
		if newest == nil || b.Time > newest.Time {
			newest = &b
		}
	}

	if newest != nil {
		return newest
	}

	for i := range all {
		b := all[i]
		if newest == nil || b.Time > newest.Time {
			newest = &b
		}
	}
	return newest
}

func resolveLocalPath(target models.BackupTarget, apiPath string) (string, error) {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" {
		return "", fmt.Errorf("backup path empty in api response")
	}

	mount := strings.TrimSpace(target.BackupMount)
	if mount == "" {
		return "", models.ErrMissingBackupMount
	}

	local := apiPath
	if strings.HasPrefix(apiPath, "/config") {
		local = filepath.Join(mount, strings.TrimPrefix(apiPath, "/config"))
	} else if vol := strings.TrimPrefix(apiPath, "\\config"); vol != apiPath {
		local = filepath.Join(mount, vol)
	}

	local = filepath.Clean(local)
	if _, err := os.Stat(local); err != nil {
		return "", fmt.Errorf("backup file not found at %q (api path %q): %w", local, apiPath, err)
	}

	return local, nil
}

func (c *Client) doJSON(ctx context.Context, target models.BackupTarget, method, uri string, body []byte, out any) error {
	endpoint, err := joinURL(target.APIBaseURL, uri)
	if err != nil {
		return err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", target.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("http %s %s: %s", method, resp.Status, strings.TrimSpace(string(payload)))
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func joinURL(base, uri string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "", fmt.Errorf("api base url is empty")
	}

	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	u.Path = path.Join(u.Path, uri)
	return u.String(), nil
}
