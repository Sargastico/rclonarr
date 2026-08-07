package postgres

import (
	"testing"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDumper_BuildDumpArgs_URI(t *testing.T) {
	config.App = config.AppInfo{
		PostgresURI: "postgresql://user:pass@db:5432/app",
	}

	d := NewDumper()
	args, env, err := d.BuildDumpArgs("/tmp/app.dump")
	require.NoError(t, err)
	assert.Empty(t, env)
	assert.Equal(t, []string{
		"--format=custom",
		"--file", "/tmp/app.dump",
		"--dbname", "postgresql://user:pass@db:5432/app",
	}, args)
}

func TestDumper_BuildDumpArgs_Discrete(t *testing.T) {
	config.App = config.AppInfo{
		PostgresHost:     "db",
		PostgresPort:     "5432",
		PostgresUser:     "app",
		PostgresPassword: "secret",
		PostgresDBName:   "appdb",
	}

	d := NewDumper()
	args, env, err := d.BuildDumpArgs("/tmp/app.dump")
	require.NoError(t, err)
	assert.Equal(t, []string{"PGPASSWORD=secret"}, env)
	assert.Equal(t, []string{
		"--format=custom",
		"--file", "/tmp/app.dump",
		"--host", "db",
		"--port", "5432",
		"--username", "app",
		"--dbname", "appdb",
	}, args)
}

func TestDumper_BuildDumpArgs_RequiresHostOrURI(t *testing.T) {
	config.App = config.AppInfo{}
	d := NewDumper()
	_, _, err := d.BuildDumpArgs("/tmp/app.dump")
	require.Error(t, err)
}

func TestReplaceURIDatabase(t *testing.T) {
	t.Parallel()

	got, err := replaceURIDatabase("postgresql://admin:pass@postgres:5432/postgres", "miniflux")
	require.NoError(t, err)
	assert.Equal(t, "postgresql://admin:pass@postgres:5432/miniflux", got)
}

func TestDumper_connectionArgs_AllDatabasesOverride(t *testing.T) {
	config.App = config.AppInfo{
		PostgresHost:     "postgres",
		PostgresUser:     "admin",
		PostgresPassword: "secret",
		PostgresDBName:   "postgres",
	}

	d := NewDumper()
	args, env, err := d.connectionArgs("kan")
	require.NoError(t, err)
	assert.Equal(t, []string{"PGPASSWORD=secret"}, env)
	assert.Equal(t, []string{
		"--host", "postgres",
		"--username", "admin",
		"--dbname", "kan",
	}, args)
}
