package ops

import (
	"context"
	"io"
	"maps"
	"slices"
	"time"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
	"github.com/0xble/eightsleep/internal/config"
)

func registerTempModes(reg *op.Registry, b *Backend) {
	type mode struct {
		name, cli, summary, what string
		run                      func(*client.TempModes, context.Context) error
	}
	for _, m := range []mode{
		{"tempmode.nap.on", "tempmode nap on", "Activate nap mode", "activate nap mode", (*client.TempModes).NapActivate},
		{"tempmode.nap.off", "tempmode nap off", "Deactivate nap mode", "deactivate nap mode", (*client.TempModes).NapDeactivate},
		{"tempmode.nap.extend", "tempmode nap extend", "Extend nap mode", "extend nap mode", (*client.TempModes).NapExtend},
		{"tempmode.hotflash.on", "tempmode hotflash on", "Activate hot-flash mode", "activate hot-flash mode", (*client.TempModes).HotFlashActivate},
		{"tempmode.hotflash.off", "tempmode hotflash off", "Deactivate hot-flash mode", "deactivate hot-flash mode", (*client.TempModes).HotFlashDeactivate},
	} {
		action(reg, b, m.name, m.cli, m.summary, m.what, func(ctx context.Context, cl *client.Client, _ NoInput) error {
			return m.run(cl.TempModes(), ctx)
		})
	}
	status := func(name, cli, summary string, read func(*client.TempModes, context.Context, any) error) {
		op.Add(reg, op.Op[NoInput, []Row]{
			Name: name, CLI: cli, Summary: summary, Effect: op.Read, MCP: true,
			Handler: func(ctx context.Context, req op.Request, _ NoInput) ([]Row, error) {
				cl, _, err := b.client(req)
				if err != nil {
					return nil, err
				}
				var out map[string]any
				if err := read(cl.TempModes(), ctx, &out); err != nil {
					return nil, providerErr(err)
				}
				return []Row{out}, nil
			},
			// The columns are the payload's keys, sorted.
			Render: func(w io.Writer, rs []Row) error {
				return b.printRows(w, slices.Sorted(maps.Keys(rs[0])), rs)
			},
		})
	}
	status("tempmode.nap.status", "tempmode nap status", "Show nap mode status", (*client.TempModes).NapStatus)
	status("tempmode.hotflash.status", "tempmode hotflash status", "Show hot-flash mode status", (*client.TempModes).HotFlashStatus)

	raw(reg, b, "tempmode.events", "tempmode events", "List temperature events", "events",
		func(ctx context.Context, cl *client.Client, _ config.Settings, in DateRange) (any, error) {
			from, to := dayBounds(in.From, in.To)
			var out any
			err := cl.TempModes().TempEvents(ctx, from, to, &out)
			return out, err
		})
}

// dayBounds widens YYYY-MM-DD dates to the RFC 3339 start and end of the day
// in UTC. Other values pass through.
func dayBounds(from, to string) (string, string) {
	if d, err := time.Parse(dateLayout, from); err == nil {
		from = d.UTC().Format("2006-01-02T15:04:05Z")
	}
	if d, err := time.Parse(dateLayout, to); err == nil {
		to = time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.UTC).Format("2006-01-02T15:04:05Z")
	}
	return from, to
}
