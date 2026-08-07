package postgres

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

type Dumper struct {
	pgDumpPath string
	psqlPath   string
}

var _ port.PostgresDumper = (*Dumper)(nil)

func NewDumper() *Dumper {
	pgDump := strings.TrimSpace(config.App.PgDumpPath)
	if pgDump == "" {
		pgDump = "pg_dump"
	}
	psql := strings.TrimSpace(config.App.PsqlPath)
	if psql == "" {
		psql = "psql"
	}
	return &Dumper{pgDumpPath: pgDump, psqlPath: psql}
}

// Dump writes custom-format dump(s).
// When APP_POSTGRES_ALL_DATABASES is true, dest is a directory and each DB is written as <name>.dump.
// Otherwise dest is the output .dump file path for a single database.
func (d *Dumper) Dump(ctx context.Context, dest string) error {
	if config.App.PostgresAllDatabases {
		return d.dumpAll(ctx, dest)
	}
	return d.dumpOne(ctx, "", dest)
}

func (d *Dumper) dumpAll(ctx context.Context, dumpDir string) error {
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return fmt.Errorf("create postgres dump dir: %w", err)
	}

	dbs, err := d.listDatabases(ctx)
	if err != nil {
		return err
	}
	if len(dbs) == 0 {
		return fmt.Errorf("no databases found to dump")
	}

	for _, db := range dbs {
		path := filepath.Join(dumpDir, db+".dump")
		if err := d.dumpOne(ctx, db, path); err != nil {
			return fmt.Errorf("dump database %q: %w", db, err)
		}
	}

	return nil
}

func (d *Dumper) dumpOne(ctx context.Context, dbName, dumpFile string) error {
	args, env, err := d.buildDumpArgs(dbName, dumpFile)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, d.pgDumpPath, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	return nil
}

func (d *Dumper) listDatabases(ctx context.Context) ([]string, error) {
	args, env, err := d.buildPsqlArgs(
		"-tAc",
		"SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn ORDER BY 1",
	)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, d.psqlPath, args...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("psql list databases: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}

	var dbs []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		dbs = append(dbs, name)
	}
	return dbs, nil
}

func (d *Dumper) buildDumpArgs(dbName, dumpFile string) ([]string, []string, error) {
	args := []string{
		"--format=custom",
		"--file", dumpFile,
	}

	connArgs, env, err := d.connectionArgs(dbName)
	if err != nil {
		return nil, nil, err
	}
	args = append(args, connArgs...)

	if extra := strings.TrimSpace(config.App.PostgresExtraArgs); extra != "" {
		args = append(args, strings.Fields(extra)...)
	}
	return args, env, nil
}

func (d *Dumper) buildPsqlArgs(extra ...string) ([]string, []string, error) {
	connArgs, env, err := d.connectionArgs("")
	if err != nil {
		return nil, nil, err
	}
	return append(connArgs, extra...), env, nil
}

// connectionArgs returns pg_dump/psql connection flags.
// dbName overrides the configured database when non-empty (used for all-databases dumps).
func (d *Dumper) connectionArgs(dbName string) ([]string, []string, error) {
	if uri := strings.TrimSpace(config.App.PostgresURI); uri != "" {
		if dbName != "" {
			overridden, err := replaceURIDatabase(uri, dbName)
			if err != nil {
				return nil, nil, err
			}
			uri = overridden
		}
		return []string{"--dbname", uri}, nil, nil
	}

	host := strings.TrimSpace(config.App.PostgresHost)
	if host == "" {
		return nil, nil, fmt.Errorf("postgres host or uri required")
	}

	db := strings.TrimSpace(dbName)
	if db == "" {
		db = strings.TrimSpace(config.App.PostgresDBName)
	}
	if db == "" {
		// Maintenance connection for listing databases.
		db = "postgres"
	}

	args := []string{"--host", host}
	if port := strings.TrimSpace(config.App.PostgresPort); port != "" {
		args = append(args, "--port", port)
	}
	if user := strings.TrimSpace(config.App.PostgresUser); user != "" {
		args = append(args, "--username", user)
	}
	args = append(args, "--dbname", db)

	var env []string
	if pass := config.App.PostgresPassword; pass != "" {
		env = append(env, "PGPASSWORD="+pass)
	}
	return args, env, nil
}

func replaceURIDatabase(rawURI, dbName string) (string, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("parse postgres uri: %w", err)
	}
	u.Path = "/" + dbName
	// Clear query dbname if present (some drivers use ?dbname=).
	q := u.Query()
	if q.Has("dbname") {
		q.Set("dbname", dbName)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// BuildDumpArgs exposes argv/env construction for tests (single-DB path).
func (d *Dumper) BuildDumpArgs(dumpFile string) ([]string, []string, error) {
	return d.buildDumpArgs("", dumpFile)
}
