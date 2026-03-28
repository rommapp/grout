package sync

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDirectory creates a zip file from a directory, preserving the directory structure.
// Returns the path to the temporary zip file. Caller is responsible for cleanup.
func ZipDirectory(dirPath string) (string, error) {
	tmpFile, err := os.CreateTemp("", "grout-save-*.zip")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	w := zip.NewWriter(tmpFile)
	defer w.Close()

	baseName := filepath.Base(dirPath)

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(filepath.Dir(dirPath), path)
		if err != nil {
			return err
		}

		if strings.HasPrefix(filepath.Base(rel), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			_, err := w.Create(rel + "/")
			return err
		}

		fw, err := w.Create(rel)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(fw, f)
		return err
	})

	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	_ = baseName // used implicitly via filepath.Rel

	return tmpFile.Name(), nil
}

// UnzipToDirectory extracts a zip file to a target directory.
func UnzipToDirectory(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath := filepath.Join(targetDir, f.Name)

		// Prevent zip slip
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(targetDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
