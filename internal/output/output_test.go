package output

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestFilterFields(t *testing.T) {
	rows := []map[string]any{{"a": 1, "b": 2}, {"a": 3, "b": 4}}
	got := FilterFields(rows, []string{"b"})
	if len(got) != 2 {
		t.Fatalf("expected 2 rows")
	}
	if _, ok := got[0]["a"]; ok {
		t.Fatalf("field a should be filtered out")
	}
	if got[0]["b"].(int) != 2 || got[1]["b"].(int) != 4 {
		t.Fatalf("unexpected values: %+v", got)
	}
}

func TestPrintFormats(t *testing.T) {
	rows := []map[string]any{{"name": "left", "level": 12}}
	headers := []string{"name", "level"}
	tests := []struct {
		name   string
		format Format
		want   string
	}{
		{name: "json", format: FormatJSON, want: `"name": "left"`},
		{name: "csv", format: FormatCSV, want: "name,level\nleft,12\n"},
		{name: "table", format: FormatTable, want: "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(t, func() {
				if err := Print(tt.format, headers, rows); err != nil {
					t.Fatalf("Print: %v", err)
				}
			})
			if !strings.Contains(got, tt.want) {
				t.Fatalf("output = %q, want substring %q", got, tt.want)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(out)
}
