package sync

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDirectory creates a zip file from a single directory.
// Returns the path to the temporary zip file. Caller is responsible for cleanup.
func ZipDirectory(dirPath string) (string, error) {
	return ZipDirectories([]string{dirPath})
}

// ZipDirectories creates a zip file containing multiple directories.
// Each directory is preserved as a top-level subdirectory in the archive,
// allowing a single zip to hold e.g. UCUS98751_DATA00/, UCUS98751_DATA01/, UCUS98751_INSDIR/.
// Returns the path to the temporary zip file. Caller is responsible for cleanup.
func ZipDirectories(dirPaths []string) (string, error) {
	tmpFile, err := os.CreateTemp("", "grout-save-*.zip")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	w := zip.NewWriter(tmpFile)
	defer w.Close()

	for _, dirPath := range dirPaths {
		if err := addDirToZip(w, dirPath); err != nil {
			os.Remove(tmpFile.Name())
			return "", err
		}
	}

	return tmpFile.Name(), nil
}

// addDirToZip walks dirPath and writes all its contents into w,
// preserving the directory's base name as the top-level entry in the archive.
func addDirToZip(w *zip.Writer, dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
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
