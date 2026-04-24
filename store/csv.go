package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Append writes one row to the CSV file for the given dimension.
// Creates the file with a header if it does not exist.
func Append(path, dimName string, t time.Time, value float64) error {
	needHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needHeader = true
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if needHeader {
		if _, err := fmt.Fprintln(f, "dimension,timestamp,value"); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(f, "%s,%s,%.4f\n",
		dimName,
		t.Format("2006-01-02 15:04:05"),
		value,
	)
	return err
}

// RotateCSVs rotates all CSV files in the given directory.
// csv_*.csv -> csv_*.csv.1 -> csv_*.csv.2 -> csv_*.csv.3 (deleted)
func RotateCSVs(dir string) error {
	pattern := filepath.Join(dir, "csv_*.csv")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, f := range files {
		rotateFile(f, 3)
	}
	return nil
}

// rotateFile rotates a single file up to maxBackups copies.
func rotateFile(path string, maxBackups int) {
	// Remove oldest backup
	oldest := fmt.Sprintf("%s.%d", path, maxBackups)
	os.Remove(oldest)

	// Shift backups: .2 -> .3, .1 -> .2, etc.
	for i := maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", path, i)
		dst := fmt.Sprintf("%s.%d", path, i+1)
		os.Rename(src, dst)
	}

	// Rotate current file to .1
	if _, err := os.Stat(path); err == nil {
		os.Rename(path, path+".1")
	}
}
