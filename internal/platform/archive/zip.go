package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const versionedZipTimeLayout = "20060102_150405"

// VersionedZipName returns a UTC timestamped backup filename, e.g. sonarr_backup_20260527_031053.zip.
func VersionedZipName(targetID string, at time.Time) string {
	return fmt.Sprintf("%s_backup_%s.zip", targetID, at.UTC().Format(versionedZipTimeLayout))
}

// ZipDirectory writes the contents of srcDir into destZip (deflate).
func ZipDirectory(srcDir, destZip string) error {
	srcDir = filepath.Clean(srcDir)
	info, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", srcDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source %q is not a directory", srcDir)
	}

	if err := os.MkdirAll(filepath.Dir(destZip), 0o755); err != nil {
		return fmt.Errorf("create zip parent dir: %w", err)
	}

	out, err := os.Create(destZip)
	if err != nil {
		return fmt.Errorf("create zip %q: %w", destZip, err)
	}

	zw := zip.NewWriter(out)
	zipErr := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			_, err := zw.Create(rel + "/")
			return err
		}

		if !d.Type().IsRegular() {
			return nil
		}

		return addFileToZip(zw, path, rel)
	})
	closeErr := zw.Close()
	if err := out.Close(); err != nil && zipErr == nil {
		closeErr = err
	}

	if zipErr != nil {
		_ = os.Remove(destZip)
		return fmt.Errorf("zip directory %q: %w", srcDir, zipErr)
	}
	if closeErr != nil {
		_ = os.Remove(destZip)
		return fmt.Errorf("finalize zip %q: %w", destZip, closeErr)
	}

	return nil
}

// ZipDirectoryToTemp zips srcDir into a new temp file named with VersionedZipName.
func ZipDirectoryToTemp(srcDir, targetID string, at time.Time) (zipPath string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp("", "rclonarr-zip-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	zipPath = filepath.Join(tmpDir, VersionedZipName(targetID, at))
	if err := ZipDirectory(srcDir, zipPath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}

	return zipPath, func() { _ = os.RemoveAll(tmpDir) }, nil
}

func addFileToZip(zw *zip.Writer, path, name string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}

	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	_, err = io.Copy(w, in)
	return err
}

// IsZipFile reports whether path has a .zip extension (case-insensitive).
func IsZipFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".zip")
}
