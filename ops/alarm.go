package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
)

// AlarmRow is one alarm as alarm list prints it.
type AlarmRow struct {
	ID        string                `json:"id"`
	Time      string                `json:"time"`
	Enabled   bool                  `json:"enabled"`
	Days      []int                 `json:"days"`
	Vibration client.AlarmVibration `json:"vibration"`
	Sound     *string               `json:"sound"`
}

// ActiveAlarms are the alarms ringing now, as the app API returns them.
type ActiveAlarms struct {
	Alarms []map[string]any `json:"alarms"`
}

// AlarmCreateInput describes a new alarm.
type AlarmCreateInput struct {
	Time        string `json:"time,omitempty" help:"HH:MM time"`
	Days        []int  `json:"days,omitempty" help:"Comma-separated days 0=Sun..6=Sat"`
	Disabled    bool   `json:"disabled,omitempty" help:"Create disabled"`
	NoVibration bool   `json:"no_vibration,omitempty" name:"no-vibration" help:"Disable vibration"`
	Sound       string `json:"sound,omitempty" help:"Sound id"`
}

// AlarmUpdateInput changes the given fields of an alarm.
type AlarmUpdateInput struct {
	ID          string `json:"id" arg:"" help:"Alarm ID"`
	Time        string `json:"time,omitempty" help:"HH:MM time"`
	Days        []int  `json:"days,omitempty" help:"Comma-separated days 0=Sun..6=Sat"`
	Enabled     *bool  `json:"enabled,omitempty" help:"Set enabled true/false"`
	NoVibration *bool  `json:"no_vibration,omitempty" name:"no-vibration" help:"Disable vibration"`
	Sound       string `json:"sound,omitempty" help:"Sound id"`
}

// AlarmIDInput names one alarm.
type AlarmIDInput struct {
	ID string `json:"id" arg:"" help:"Alarm ID"`
}

// AlarmCreated is the alarm created, or the alarm a preview would create.
type AlarmCreated struct {
	Applied bool         `json:"applied"`
	Alarm   client.Alarm `json:"alarm"`
}

// AlarmChange is the result of an update or delete.
type AlarmChange struct {
	Applied bool           `json:"applied"`
	ID      string         `json:"id"`
	Patch   map[string]any `json:"patch,omitempty"`
}

// DismissAll is eightsleepctl's dismiss-all result: what was done, or with
// --dry-run the routes it would use and the next scheduled alarm.
type DismissAll struct {
	OK     bool   `json:"ok,omitempty"`
	Method string `json:"method,omitempty"`
	// DismissedAlarmIDs is present when each active alarm was dismissed in
	// turn, because the bulk route was missing.
	DismissedAlarmIDs []string `json:"dismissed_alarm_ids,omitzero"`
	DryRun            bool     `json:"dry_run,omitempty"`
	Primary           string   `json:"primary,omitempty"`
	Fallback          string   `json:"fallback,omitempty"`
	NextAlarmID       *string  `json:"next_alarm_id,omitempty"`
}

// MarshalJSON prints a dry run with next_alarm_id null when no alarm is
// scheduled, as eightsleepctl did.
func (d DismissAll) MarshalJSON() ([]byte, error) {
	type plain DismissAll
	if !d.DryRun {
		return json.Marshal(plain(d))
	}
	return json.Marshal(struct {
		DryRun      bool    `json:"dry_run"`
		Primary     string  `json:"primary"`
		Fallback    string  `json:"fallback"`
		NextAlarmID *string `json:"next_alarm_id"`
	}{d.DryRun, d.Primary, d.Fallback, d.NextAlarmID})
}

func registerAlarms(reg *op.Registry, b *Backend) {
	rows(reg, b, op.Op[NoInput, []AlarmRow]{
		Name: "alarm.list", Summary: "List alarms",
		Handler: func(ctx context.Context, req op.Request, _ NoInput) ([]AlarmRow, error) {
			cl, _, err := b.client(req)
			if err != nil {
				return nil, err
			}
			alarms, err := cl.ListAlarms(ctx)
			if err != nil {
				return nil, providerErr(err)
			}
			out := make([]AlarmRow, 0, len(alarms))
			for _, a := range alarms {
				out = append(out, AlarmRow{ID: a.ID, Time: a.Time, Enabled: a.Enabled, Days: a.DaysOfWeek, Vibration: a.Vibration, Sound: a.Sound})
			}
			return out, nil
		},
	}, "id", "time", "enabled", "days", "vibration", "sound")

	op.Add(reg, op.Op[NoInput, ActiveAlarms]{
		Name: "alarm.active", Summary: "List the alarms ringing now", Effect: op.Read, MCP: true,
		Handler: func(ctx context.Context, req op.Request, _ NoInput) (ActiveAlarms, error) {
			cl, _, err := b.client(req)
			if err != nil {
				return ActiveAlarms{}, err
			}
			alarms, err := cl.Alarms().ActiveAlarms(ctx)
			return ActiveAlarms{Alarms: alarms}, providerErr(err)
		},
		Render: func(w io.Writer, a ActiveAlarms) error {
			return b.printRows(w, []string{"id", "time", "enabled"}, a.Alarms)
		},
	})

	op.Add(reg, op.Op[AlarmCreateInput, AlarmCreated]{
		Name: "alarm.create", Summary: "Create an alarm", Effect: op.Write, MCP: true, CLIImmediate: true,
		Handler: func(ctx context.Context, req op.Request, in AlarmCreateInput) (AlarmCreated, error) {
			if in.Time == "" {
				return AlarmCreated{}, usage("--time required")
			}
			if len(in.Days) == 0 {
				return AlarmCreated{}, usage("--days required (comma separated 0=Sun..6=Sat)")
			}
			alarm := client.Alarm{Enabled: !in.Disabled, Time: in.Time, DaysOfWeek: in.Days,
				Vibration: client.AlarmVibration{Enabled: !in.NoVibration}}
			if in.Sound != "" {
				alarm.Sound = &in.Sound
			}
			cl, _, err := b.client(req)
			if err != nil || !req.Apply {
				return AlarmCreated{Alarm: alarm}, err
			}
			created, err := cl.CreateAlarm(ctx, alarm)
			if err != nil {
				return AlarmCreated{}, providerErr(err)
			}
			return AlarmCreated{Applied: true, Alarm: *created}, nil
		},
		Render: func(w io.Writer, r AlarmCreated) error {
			if !r.Applied {
				_, err := fmt.Fprintf(w, "dry run: would create alarm at %s on days %v\n", r.Alarm.Time, r.Alarm.DaysOfWeek)
				return err
			}
			_, err := fmt.Fprintf(w, "created alarm %s\n", r.Alarm.ID)
			return err
		},
	})

	op.Add(reg, op.Op[AlarmUpdateInput, AlarmChange]{
		Name: "alarm.update", Summary: "Update an alarm", Effect: op.Write, MCP: true, CLIImmediate: true,
		Handler: func(ctx context.Context, req op.Request, in AlarmUpdateInput) (AlarmChange, error) {
			patch := map[string]any{}
			if in.Time != "" {
				patch["time"] = in.Time
			}
			if len(in.Days) > 0 {
				patch["daysOfWeek"] = in.Days
			}
			if in.Enabled != nil {
				patch["enabled"] = *in.Enabled
			}
			if in.NoVibration != nil {
				patch["vibration"] = !*in.NoVibration
			}
			if in.Sound != "" {
				patch["sound"] = in.Sound
			}
			if len(patch) == 0 {
				return AlarmChange{}, usage("no fields to update")
			}
			out := AlarmChange{ID: in.ID, Patch: patch}
			cl, _, err := b.client(req)
			if err != nil || !req.Apply {
				return out, err
			}
			if _, err := cl.UpdateAlarm(ctx, in.ID, patch); err != nil {
				return AlarmChange{}, providerErr(err)
			}
			out.Applied = true
			return out, nil
		},
		Render: changeLine("updated", "update alarm"),
	})

	op.Add(reg, op.Op[AlarmIDInput, AlarmChange]{
		// eightctl deleted without a prompt, so typing the command confirms
		// it. HTTP and MCP callers must confirm.
		Name: "alarm.delete", Summary: "Delete an alarm", Effect: op.Destructive, MCP: true,
		CLIImmediate: true, CLIConfirmed: true,
		Handler: func(ctx context.Context, req op.Request, in AlarmIDInput) (AlarmChange, error) {
			out := AlarmChange{ID: in.ID}
			cl, _, err := b.client(req)
			if err != nil || !req.Apply {
				return out, err
			}
			if err := cl.DeleteAlarm(ctx, in.ID); err != nil {
				return AlarmChange{}, providerErr(err)
			}
			out.Applied = true
			return out, nil
		},
		Render: changeLine("deleted", "delete alarm"),
	})

	alarmAction := func(name, cli, summary, verb string, run func(context.Context, *client.Client, string) error) {
		op.Add(reg, op.Op[AlarmIDInput, Done]{
			Name: name, CLI: cli, Summary: summary, Effect: op.Write, MCP: true, CLIImmediate: true,
			Handler: func(ctx context.Context, req op.Request, in AlarmIDInput) (Done, error) {
				return b.act(ctx, req, verb+" alarm "+in.ID, func(ctx context.Context, cl *client.Client) error { return run(ctx, cl, in.ID) })
			},
			Render: silent,
		})
	}
	alarmAction("alarm.snooze", "", "Snooze an alarm", "snooze", func(ctx context.Context, cl *client.Client, id string) error {
		return cl.Alarms().Snooze(ctx, id)
	})
	alarmAction("alarm.dismiss", "", "Dismiss an alarm", "dismiss", func(ctx context.Context, cl *client.Client, id string) error {
		return cl.Alarms().Dismiss(ctx, id)
	})
	action(reg, b, "alarm.vibration_test", "alarm vibration-test", "Run an alarm vibration test", "run an alarm vibration test",
		func(ctx context.Context, cl *client.Client, _ NoInput) error { return cl.Alarms().VibrationTest(ctx) })

	op.Add(reg, op.Op[NoInput, DismissAll]{
		Name: "alarm.dismiss_all", CLI: "alarm dismiss-all", Summary: "Dismiss every active alarm", Effect: op.Write, MCP: true,
		CLIImmediate: true,
		Handler: func(ctx context.Context, req op.Request, _ NoInput) (DismissAll, error) {
			cl, _, err := b.client(req)
			if err != nil {
				return DismissAll{}, err
			}
			alarms := cl.Alarms()
			if !req.Apply {
				next, err := alarms.NextAlarmID(ctx)
				if err != nil {
					return DismissAll{}, providerErr(err)
				}
				d := DismissAll{DryRun: true, Primary: alarms.DismissAllRoute() + " (PUT)",
					Fallback: alarms.DismissRoute("{id}") + " (POST for each active alarm)"}
				if next != "" {
					d.NextAlarmID = &next
				}
				return d, nil
			}
			dismissed, err := alarms.DismissActive(ctx)
			if err != nil {
				return DismissAll{}, providerErr(err)
			}
			if dismissed == nil {
				return DismissAll{OK: true, Method: "alarms/active/dismiss-all (PUT)"}, nil
			}
			return DismissAll{OK: true, Method: "alarms/{id}/dismiss (POST)", DismissedAlarmIDs: dismissed}, nil
		},
		Render: func(w io.Writer, d DismissAll) error {
			if !d.DryRun {
				return nil
			}
			next := "none"
			if d.NextAlarmID != nil {
				next = *d.NextAlarmID
			}
			_, err := fmt.Fprintf(w, "dry run: would dismiss every active alarm with %s\nfallback: %s\nnext alarm: %s\n", d.Primary, d.Fallback, next)
			return err
		},
	})
}

func changeLine(done, verb string) func(io.Writer, AlarmChange) error {
	return func(w io.Writer, c AlarmChange) error {
		if !c.Applied {
			_, err := fmt.Fprintf(w, "dry run: would %s %s\n", verb, c.ID)
			return err
		}
		_, err := fmt.Fprintln(w, done)
		return err
	}
}

// act runs a write that reports nothing: a preview resolves no target and
// makes no request.
func (b *Backend) act(ctx context.Context, req op.Request, what string, run func(context.Context, *client.Client) error) (Done, error) {
	cl, _, err := b.client(req)
	if err != nil {
		return Done{}, err
	}
	if !req.Apply {
		return Done{Action: what}, nil
	}
	if err := run(ctx, cl); err != nil {
		return Done{}, providerErr(err)
	}
	return Done{Applied: true, Action: what}, nil
}

// action registers an immediate write that printed nothing in eightctl.
func action[In any](reg *op.Registry, b *Backend, name, cli, summary, what string, run func(context.Context, *client.Client, In) error) {
	op.Add(reg, op.Op[In, Done]{
		Name: name, CLI: cli, Summary: summary, Effect: op.Write, MCP: true, CLIImmediate: true,
		Handler: func(ctx context.Context, req op.Request, in In) (Done, error) {
			return b.act(ctx, req, what, func(ctx context.Context, cl *client.Client) error { return run(ctx, cl, in) })
		},
		Render: silent,
	})
}
