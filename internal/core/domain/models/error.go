package models

import "errors"

var (
	ErrNoEnabledTargets = errors.New("no enabled backup targets configured")
	ErrUnknownTarget    = errors.New("unknown backup target")
	ErrMissingPath      = errors.New("config path not set for enabled target")
	ErrMissingAPI       = errors.New("api url and api key are required for this target")
	ErrMissingBackupMount = errors.New("backup mount path is required to read backup files from disk")
	ErrMissingRclone    = errors.New("rclone remote configuration is incomplete")
	ErrMissingMongo     = errors.New("komodo mongo connection is not configured")
)
