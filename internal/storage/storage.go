package storage

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

// getDataDir returns DATA_DIR env var or /data by default
func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return "/data"
}

// getUsersDir returns the users directory under DATA_DIR
func getUsersDir() string {
	return filepath.Join(getDataDir(), "users")
}

// BackupUsersDir creates a gzipped tar archive of the users directory
func BackupUsersDir(destPath string) error {
	dataDir := getUsersDir()
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
}

// RestoreUsersDir extracts a gzipped tar archive into the users directory.
// If overwrite is false, existing files are kept and conflicting entries are skipped.
func RestoreUsersDir(archivePath string, overwrite bool) error {
	dataDir := getUsersDir()
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dataDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			if !overwrite {
				if _, err := os.Stat(targetPath); err == nil {
					// skip existing file
					if _, err := io.Copy(io.Discard, tr); err != nil {
						return err
					}
					continue
				}
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
