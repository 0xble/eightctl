package ops

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
	"github.com/0xble/eightsleep/internal/config"
)

// Row is a row of a raw provider payload, keyed by eightctl's column name.
type Row = map[string]any

// format is the human output format: --output, then EIGHTCTL_OUTPUT, then the
// config file, then table. Anything but json and csv prints a table, as
// eightctl did.
func (b *Backend) format() string {
	s, err := b.load(op.SurfaceCLI)
	if err != nil {
		return "table"
	}
	return s.Output
}

// printRows prints rows the way eightctl's printRows did: an indented JSON
// array, CSV, or a tab-aligned table of the header columns.
func (b *Backend) printRows(w io.Writer, headers []string, rows []map[string]any) error {
	switch b.format() {
	case "json":
		if rows == nil {
			rows = []map[string]any{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	case "csv":
		cw := csv.NewWriter(w)
		if err := cw.Write(headers); err != nil {
			return err
		}
		for _, row := range rows {
			if err := cw.Write(cells(headers, row)); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	default:
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, row := range rows {
			_, _ = fmt.Fprintln(tw, strings.Join(cells(headers, row), "\t"))
		}
		return tw.Flush()
	}
}

func cells(headers []string, row map[string]any) []string {
	out := make([]string, len(headers))
	for i, h := range headers {
		out[i] = fmt.Sprint(row[h])
	}
	return out
}

// rowMaps turns typed rows into eightctl's row maps: each json-tagged field
// keeps its Go value, so tables print what eightctl printed. Omitted empty
// fields are left out, as eightctl left those keys out. Pointers are
// dereferenced: eightctl printed their addresses.
func rowMaps[R any](rows []R) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowMap(r))
	}
	return out
}

func rowMap(r any) map[string]any {
	if m, ok := r.(map[string]any); ok {
		return m
	}
	v := reflect.ValueOf(r)
	m := map[string]any{}
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		fv := v.Field(i)
		if strings.Contains(opts, "omitempty") && fv.IsZero() {
			continue
		}
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				m[name] = nil
				continue
			}
			fv = fv.Elem()
		}
		m[name] = fv.Interface()
	}
	return m
}

// rows registers a read whose output is eightctl's printRows table. headers
// lists the columns in order.
func rows[In, R any](reg *op.Registry, b *Backend, o op.Op[In, []R], headers ...string) {
	o.Effect = op.Read
	o.MCP = true
	o.RenderWithInput = func(w io.Writer, _ In, out []R) error {
		return b.printRows(w, headers, rowMaps(out))
	}
	op.Add(reg, o)
}

// raw registers a read of one provider payload, printed as eightctl's
// single-column row: {"<column>": <payload>}.
func raw[In any](reg *op.Registry, b *Backend, name, cli, summary, column string,
	fetch func(ctx context.Context, cl *client.Client, s config.Settings, in In) (any, error)) {
	rows(reg, b, op.Op[In, []Row]{
		Name: name, CLI: cli, Summary: summary,
		Handler: func(ctx context.Context, req op.Request, in In) ([]Row, error) {
			cl, s, err := b.client(req)
			if err != nil {
				return nil, err
			}
			v, err := fetch(ctx, cl, s, in)
			if err != nil {
				return nil, providerErr(err)
			}
			return []Row{{column: v}}, nil
		},
	}, column)
}

// silent renders nothing for an applied write, as eightctl printed nothing,
// and a one-line description for a preview.
func silent(w io.Writer, d Done) error {
	if d.Applied {
		return nil
	}
	_, err := fmt.Fprintf(w, "dry run: would %s\n", d.Action)
	return err
}

// Done is the result of a write that reports nothing else.
type Done struct {
	// Applied is false for a preview.
	Applied bool `json:"applied"`
	// Action describes the change, such as "snooze alarm a1".
	Action string `json:"action"`
}
