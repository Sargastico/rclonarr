package protondrive

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

type streamRunner func(ctx context.Context, name string, args []string, onLine func(string)) error

var authURLPattern = regexp.MustCompile(`https://account\.proton\.me/[^\s]+`)

// Uploader uploads local files with the Proton Drive CLI binary.
type Uploader struct {
	run       commandRunner
	runStream streamRunner
	authMu    sync.Mutex
}

var _ port.RemoteUploader = (*Uploader)(nil)

func NewUploader() *Uploader {
	return &Uploader{
		run:       runCommand,
		runStream: runStreamCommand,
	}
}

func (u *Uploader) EnsureAuth(ctx context.Context) error {
	u.authMu.Lock()
	defer u.authMu.Unlock()

	if ok, err := u.checkAuthenticated(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}

	otelzap.L().WarnContext(ctx, "proton drive authentication required; starting auth login — watch logs for the sign-in URL")
	if err := u.runLogin(ctx); err != nil {
		return err
	}

	ok, err := u.checkAuthenticated(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("proton-drive auth login finished but session is still unauthenticated")
	}

	otelzap.L().InfoContext(ctx, "proton drive authentication completed")
	return nil
}

func (u *Uploader) Upload(ctx context.Context, localPath, remoteDir string) error {
	remoteDir = normalizeRemoteDir(remoteDir)
	if remoteDir == "" {
		return fmt.Errorf("remote directory is empty")
	}

	if err := u.EnsureAuth(ctx); err != nil {
		return err
	}

	if err := u.ensureDir(ctx, remoteDir); err != nil {
		if looksLikeAuthRequired(nil, err) {
			if authErr := u.EnsureAuth(ctx); authErr != nil {
				return authErr
			}
			if retryErr := u.ensureDir(ctx, remoteDir); retryErr != nil {
				return retryErr
			}
		} else {
			return err
		}
	}

	otelzap.L().InfoContext(ctx, "starting proton-drive upload",
		zap.String("source", localPath),
		zap.String("destination", remoteDir),
	)

	out, err := u.exec(ctx, "filesystem", "upload",
		"--conflict-strategy", "skip",
		"--json",
		localPath,
		remoteDir,
	)
	if err != nil {
		if looksLikeAuthRequired(out, err) {
			if authErr := u.EnsureAuth(ctx); authErr != nil {
				return authErr
			}
			out, err = u.exec(ctx, "filesystem", "upload",
				"--conflict-strategy", "skip",
				"--json",
				localPath,
				remoteDir,
			)
		}
		if err != nil {
			return fmt.Errorf("proton-drive upload %q -> %q: %w\n%s", localPath, remoteDir, err, strings.TrimSpace(string(out)))
		}
	}

	otelzap.L().InfoContext(ctx, "proton-drive upload completed",
		zap.String("source", localPath),
		zap.String("destination", remoteDir),
	)
	return nil
}

func (u *Uploader) RemotePath(subdir string) string {
	prefix := strings.TrimSuffix(normalizeRemoteDir(config.App.RemotePrefix), "/")
	subdir = strings.Trim(subdir, "/")
	if prefix == "" {
		return "/" + subdir
	}
	if subdir == "" {
		return prefix
	}
	return prefix + "/" + subdir
}

func (u *Uploader) checkAuthenticated(ctx context.Context) (bool, error) {
	out, err := u.exec(ctx, "filesystem", "list", "/my-files")
	if looksLikeAuthRequired(out, err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("proton-drive auth check failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func (u *Uploader) runLogin(ctx context.Context) error {
	name, args := u.cliArgs("auth", "login")
	runStream := u.runStream
	if runStream == nil {
		runStream = runStreamCommand
	}

	var lastURL string
	err := runStream(ctx, name, args, func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		otelzap.L().InfoContext(ctx, "proton-drive auth", zap.String("line", line))
		if url := extractAuthURL(line); url != "" && url != lastURL {
			lastURL = url
			otelzap.L().WarnContext(ctx, "PROTON DRIVE AUTH REQUIRED — open this URL in a browser to sign in",
				zap.String("auth_url", url),
			)
		}
	})
	if err != nil {
		if lastURL != "" {
			return fmt.Errorf("proton-drive auth login failed (auth URL was logged): %w", err)
		}
		return fmt.Errorf("proton-drive auth login failed: %w", err)
	}
	return nil
}

func (u *Uploader) ensureDir(ctx context.Context, remoteDir string) error {
	parts := strings.Split(strings.Trim(remoteDir, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("invalid remote directory %q", remoteDir)
	}

	current := "/" + parts[0]
	for i := 1; i < len(parts); i++ {
		name := parts[i]
		next := current + "/" + name
		if u.pathExists(ctx, next) {
			current = next
			continue
		}

		out, err := u.exec(ctx, "filesystem", "create-folder", current, name)
		if err != nil {
			if u.pathExists(ctx, next) {
				current = next
				continue
			}
			err = fmt.Errorf("proton-drive create-folder %q under %q: %w\n%s", name, current, err, strings.TrimSpace(string(out)))
			if looksLikeAuthRequired(out, err) {
				return err
			}
			return err
		}
		current = next
	}

	return nil
}

func (u *Uploader) pathExists(ctx context.Context, remotePath string) bool {
	out, err := u.exec(ctx, "filesystem", "info", remotePath)
	if looksLikeAuthRequired(out, err) {
		return false
	}
	return err == nil
}

func (u *Uploader) exec(ctx context.Context, args ...string) ([]byte, error) {
	name, cliArgs := u.cliArgs(args...)
	run := u.run
	if run == nil {
		run = runCommand
	}
	return run(ctx, name, cliArgs...)
}

func (u *Uploader) cliArgs(args ...string) (name string, cliArgs []string) {
	bin := strings.TrimSpace(config.App.ProtonDriveBin)
	if bin == "" {
		bin = "proton-drive"
	}

	if config.App.ProtonDriveDBus {
		return "dbus-run-session", append([]string{"--", bin}, args...)
	}
	return bin, args
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := append(stdout.Bytes(), stderr.Bytes()...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func runStreamCommand(ctx context.Context, name string, args []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			onLine(sc.Text())
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(stderr)
	wg.Wait()

	return cmd.Wait()
}

func looksLikeAuthRequired(out []byte, err error) bool {
	var b strings.Builder
	if len(out) > 0 {
		b.Write(out)
		b.WriteByte(' ')
	}
	if err != nil {
		b.WriteString(err.Error())
	}
	s := strings.ToLower(b.String())
	return strings.Contains(s, "need to login") ||
		strings.Contains(s, "login first") ||
		strings.Contains(s, "not authenticated") ||
		strings.Contains(s, "unauthenticated")
}

func extractAuthURL(line string) string {
	return authURLPattern.FindString(line)
}

func normalizeRemoteDir(remoteDir string) string {
	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		return ""
	}
	remoteDir = path.Clean("/" + strings.TrimPrefix(remoteDir, "/"))
	if remoteDir == "." {
		return ""
	}
	return remoteDir
}
