package store

import (
	"fmt"
	"os"
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
