package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// WalkRecords accepts a JSON array or a stream of JSON objects, including JSONL.
// It keeps only one record in memory and rejects malformed or non-object data.
func WalkRecords(path string, visit func(map[string]interface{}) error) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		b, e := r.Peek(1)
		if e != nil {
			return 0, fmt.Errorf("empty dataset")
		}
		if b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t' {
			_, _ = r.ReadByte()
			continue
		}
		break
	}
	first, _ := r.Peek(1)
	array := first[0] == '['
	d := json.NewDecoder(r)
	if array {
		if _, err = d.Token(); err != nil {
			return 0, err
		}
	}
	count := 0
	for {
		if array && !d.More() {
			break
		}
		var row map[string]interface{}
		err = d.Decode(&row)
		if err == io.EOF && !array {
			break
		}
		if err != nil {
			return count, fmt.Errorf("record %d: %w", count+1, err)
		}
		if row == nil {
			return count, fmt.Errorf("record %d must be an object", count+1)
		}
		if visit != nil {
			if err = visit(row); err != nil {
				return count, err
			}
		}
		count++
	}
	if array {
		if _, err = d.Token(); err != nil {
			return count, err
		}
		var extra any
		if err = d.Decode(&extra); err != io.EOF {
			return count, fmt.Errorf("unexpected data after JSON array")
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("dataset contains no records")
	}
	return count, nil
}

// CopyRecords converts JSON arrays/JSONL into JSONL with bounded memory and
// publishes the result only after every record passed validation.
func CopyRecords(input, output string, transform func(map[string]interface{}) error) (int, error) {
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return 0, err
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".records-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(f.Name())
	enc := json.NewEncoder(f)
	count, err := WalkRecords(input, func(row map[string]interface{}) error {
		if transform != nil {
			if err := transform(row); err != nil {
				return err
			}
		}
		return enc.Encode(row)
	})
	closeErr := f.Close()
	if err != nil {
		return count, err
	}
	if closeErr != nil {
		return count, closeErr
	}
	return count, os.Rename(f.Name(), output)
}

func PreviewRecords(path string, limit int) ([]map[string]interface{}, int, []string, error) {
	if limit < 1 || limit > 100 {
		limit = 10
	}
	rows := []map[string]interface{}{}
	fields := map[string]bool{}
	total, err := WalkRecords(path, func(row map[string]interface{}) error {
		for key := range row {
			fields[key] = true
		}
		if len(rows) < limit {
			rows = append(rows, row)
		}
		return nil
	})
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return rows, total, keys, err
}
