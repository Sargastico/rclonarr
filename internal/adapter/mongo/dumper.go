package mongo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

type Dumper struct {
	mongodumpPath string
}

var _ port.MongoDumper = (*Dumper)(nil)

func NewDumper() *Dumper {
	path := strings.TrimSpace(config.App.MongodumpPath)
	if path == "" {
		path = "mongodump"
	}
	return &Dumper{mongodumpPath: path}
}

func (d *Dumper) Dump(ctx context.Context, dumpDir string) error {
	args, err := d.buildArgs(dumpDir)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, d.mongodumpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongodump: %w", err)
	}

	return nil
}

func (d *Dumper) buildArgs(dumpDir string) ([]string, error) {
	args := []string{"--out", dumpDir}

	if uri := strings.TrimSpace(config.App.KomodoMongoURI); uri != "" {
		args = append(args, "--uri", uri)
	} else {
		addr := strings.TrimSpace(config.App.KomodoMongoAddress)
		if addr == "" {
			return nil, fmt.Errorf("komodo mongo address or uri required")
		}
		args = append(args, "--host", addr)
		if user := strings.TrimSpace(config.App.KomodoMongoUsername); user != "" {
			args = append(args, "--username", user)
		}
		if pass := config.App.KomodoMongoPassword; pass != "" {
			args = append(args, "--password", pass)
		}
		db := strings.TrimSpace(config.App.KomodoMongoDBName)
		if db == "" {
			db = "komodo"
		}
		args = append(args, "--db", db)
	}

	if extra := strings.TrimSpace(config.App.KomodoDumpExtraArgs); extra != "" {
		args = append(args, strings.Fields(extra)...)
	}

	return args, nil
}

// BuildArgs exposes argv construction for tests.
func (d *Dumper) BuildArgs(dumpDir string) ([]string, error) {
	return d.buildArgs(dumpDir)
}
