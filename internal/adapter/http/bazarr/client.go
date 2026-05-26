package bazarr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

const backupsPath = "/api/system/backups"

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
	if target.APIScheme != models.APISchemeBazarr {
		return "", fmt.Errorf("bazarr client: unsupported api scheme %q", target.APIScheme)
	}

	before, err := c.listBackups(ctx, target)
	if err != nil {
		return "", err
	}

	if err := c.createBackup(ctx, target); err != nil {
		return "", err
	}

	filename, err := c.waitForNewBackup(ctx, target, before)
	if err != nil {
		return "", err
	}

	local := filepath.Join(target.BackupMount, filename)
	local = filepath.Clean(local)
	if _, err := os.Stat(local); err != nil {
		return "", fmt.Errorf("backup file not found at %q: %w", local, err)
	}

	return local, nil
}

type backupEntry struct {
	Filename string `json:"filename"`
	Date     string `json:"date"`
}

type backupsResponse struct {
	Data []backupEntry `json:"data"`
}

func (c *Client) listBackups(ctx context.Context, target models.BackupTarget) (map[string]struct{}, error) {
	var resp backupsResponse
	if err := c.doJSON(ctx, target, http.MethodGet, backupsPath, &resp); err != nil {
		return nil, err
	}

	known := make(map[string]struct{}, len(resp.Data))
	for _, entry := range resp.Data {
		known[entry.Filename] = struct{}{}
	}
	return known, nil
}

func (c *Client) createBackup(ctx context.Context, target models.BackupTarget) error {
	return c.doJSON(ctx, target, http.MethodPost, backupsPath, nil)
}

func (c *Client) waitForNewBackup(ctx context.Context, target models.BackupTarget, before map[string]struct{}) (string, error) {
	poll := time.Duration(config.App.CommandPollIntervalMS) * time.Millisecond
	if poll <= 0 {
		poll = time.Second
	}

	timeout := time.Duration(config.App.CommandTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	deadline := time.Now().Add(timeout)

	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("bazarr backup timed out after %s", timeout)
		}

		var resp backupsResponse
		if err := c.doJSON(ctx, target, http.MethodGet, backupsPath, &resp); err != nil {
			return "", err
		}

		for _, entry := range resp.Data {
			if _, exists := before[entry.Filename]; !exists && entry.Filename != "" {
				return entry.Filename, nil
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (c *Client) doJSON(ctx context.Context, target models.BackupTarget, method, uri string, out any) error {
	endpoint, err := joinURL(target.APIBaseURL, uri)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Set("apikey", target.APIKey)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")

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

	u.Path = strings.TrimRight(u.Path, "/") + uri
	return u.String(), nil
}
