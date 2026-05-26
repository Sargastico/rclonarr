package mongo

import (
	"testing"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDumper_BuildArgs_URI(t *testing.T) {
	t.Parallel()

	config.App = config.AppInfo{
		KomodoMongoURI: "mongodb://user:pass@mongo:27017/komodo",
	}

	d := NewDumper()
	args, err := d.BuildArgs("/tmp/dump")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"--out", "/tmp/dump",
		"--uri", "mongodb://user:pass@mongo:27017/komodo",
	}, args)
}

func TestDumper_BuildArgs_Discrete(t *testing.T) {
	t.Parallel()

	config.App = config.AppInfo{
		KomodoMongoAddress:  "mongo:27017",
		KomodoMongoUsername: "admin",
		KomodoMongoPassword: "secret",
		KomodoMongoDBName:   "komodo",
	}

	d := NewDumper()
	args, err := d.BuildArgs("/tmp/dump")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"--out", "/tmp/dump",
		"--host", "mongo:27017",
		"--username", "admin",
		"--password", "secret",
		"--db", "komodo",
	}, args)
}
