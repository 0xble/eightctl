package ops

import (
	"context"
	"fmt"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
)

// PlayInput starts playback.
type PlayInput struct {
	Track string `json:"track,omitempty" help:"track ID to play"`
}

// SeekInput moves the playhead.
type SeekInput struct {
	Position int `json:"position,omitempty" help:"position milliseconds"`
}

// VolumeInput sets the volume.
type VolumeInput struct {
	Level *int `json:"level,omitempty" help:"volume level 0-100 (default 50)"`
}

func (v VolumeInput) level() int {
	if v.Level == nil {
		return 50
	}
	return *v.Level
}

// TrackInput names a track.
type TrackInput struct {
	Track string `json:"track,omitempty" help:"track id"`
}

// AngleInput sets the base angles.
type AngleInput struct {
	Head int `json:"head,omitempty" help:"head angle"`
	Foot int `json:"foot,omitempty" help:"foot angle"`
}

// PresetInput names a base preset.
type PresetInput struct {
	Name string `json:"name,omitempty" help:"preset name"`
}

// EnabledInput turns a setting on or off. Without the flag it turns it on.
type EnabledInput struct {
	Enabled *bool `json:"enabled,omitempty" help:"enable or disable (default true)"`
}

func (e EnabledInput) on() bool { return e.Enabled == nil || *e.Enabled }

// CreateTripInput describes a trip.
type CreateTripInput struct {
	Destination string `json:"destination,omitempty" help:"destination"`
	StartDate   string `json:"start_date,omitempty" name:"start-date" help:"start date"`
	EndDate     string `json:"end_date,omitempty" name:"end-date" help:"end date"`
	// Named --trip-timezone so the root --timezone keeps its meaning.
	TripTimezone string `json:"trip_timezone,omitempty" name:"trip-timezone" help:"IANA timezone for the trip destination"`
}

// PlanEdit describes a travel plan.
type PlanEdit struct {
	Name string `json:"name,omitempty" help:"plan name"`
	Date string `json:"date,omitempty" help:"plan date"`
}

// CreatePlanInput adds a plan to a trip.
type CreatePlanInput struct {
	Trip string `json:"trip,omitempty" help:"trip id"`
	PlanEdit
}

// UpdatePlanInput changes a plan.
type UpdatePlanInput struct {
	Plan string `json:"plan,omitempty" help:"plan id"`
	PlanEdit
}

func (p PlanEdit) body() map[string]any {
	body := map[string]any{}
	if p.Name != "" {
		body["name"] = p.Name
	}
	if p.Date != "" {
		body["date"] = p.Date
	}
	return body
}

// write registers an immediate write that printed nothing in eightctl. check
// validates the input before any credential or request; describe names the
// change for the preview.
func write[In any](reg *op.Registry, b *Backend, name, cli, summary string, effect op.Effect,
	check func(In) error, describe func(In) string, run func(context.Context, *client.Client, In) error) {
	op.Add(reg, op.Op[In, Done]{
		Name: name, CLI: cli, Summary: summary, Effect: effect, MCP: true, CLIImmediate: true,
		// eightctl deleted trips without a prompt, so the command line
		// confirms them. HTTP and MCP callers must confirm.
		CLIConfirmed: effect == op.Destructive,
		Handler: func(ctx context.Context, req op.Request, in In) (Done, error) {
			if check != nil {
				if err := check(in); err != nil {
					return Done{}, err
				}
			}
			return b.act(ctx, req, describe(in), func(ctx context.Context, cl *client.Client) error { return run(ctx, cl, in) })
		},
		Render: silent,
	})
}

func required(flag string) func(string) error {
	return func(v string) error {
		if v == "" {
			return usage("--%s required", flag)
		}
		return nil
	}
}

func registerActions(reg *op.Registry, b *Backend) {
	write(reg, b, "audio.play", "", "Play audio", op.Write, nil,
		func(in PlayInput) string { return "play audio " + in.Track },
		func(ctx context.Context, cl *client.Client, in PlayInput) error {
			return cl.Audio().Play(ctx, in.Track)
		})
	action(reg, b, "audio.pause", "", "Pause audio", "pause audio",
		func(ctx context.Context, cl *client.Client, _ NoInput) error { return cl.Audio().Pause(ctx) })
	write(reg, b, "audio.seek", "", "Seek the audio player", op.Write, nil,
		func(in SeekInput) string { return fmt.Sprintf("seek audio to %d ms", in.Position) },
		func(ctx context.Context, cl *client.Client, in SeekInput) error {
			return cl.Audio().Seek(ctx, in.Position)
		})
	write(reg, b, "audio.volume", "", "Set the audio volume", op.Write, nil,
		func(in VolumeInput) string { return fmt.Sprintf("set audio volume to %d", in.level()) },
		func(ctx context.Context, cl *client.Client, in VolumeInput) error {
			return cl.Audio().Volume(ctx, in.level())
		})
	action(reg, b, "audio.pair", "", "Pair the audio player", "pair the audio player",
		func(ctx context.Context, cl *client.Client, _ NoInput) error { return cl.Audio().Pair(ctx) })
	write(reg, b, "audio.favorites.add", "", "Add a favorite track", op.Write,
		func(in TrackInput) error { return required("track")(in.Track) },
		func(in TrackInput) string { return "add favorite track " + in.Track },
		func(ctx context.Context, cl *client.Client, in TrackInput) error {
			return cl.Audio().AddFavorite(ctx, in.Track)
		})
	write(reg, b, "audio.favorites.remove", "", "Remove a favorite track", op.Write,
		func(in TrackInput) error { return required("track")(in.Track) },
		func(in TrackInput) string { return "remove favorite track " + in.Track },
		func(ctx context.Context, cl *client.Client, in TrackInput) error {
			return cl.Audio().RemoveFavorite(ctx, in.Track)
		})

	write(reg, b, "base.angle", "", "Set the base angles", op.Write, nil,
		func(in AngleInput) string { return fmt.Sprintf("set base angles head %d, foot %d", in.Head, in.Foot) },
		func(ctx context.Context, cl *client.Client, in AngleInput) error {
			return cl.Base().SetAngle(ctx, in.Head, in.Foot)
		})
	write(reg, b, "base.preset_run", "base preset-run", "Run a base preset", op.Write, nil,
		func(in PresetInput) string { return "run base preset " + in.Name },
		func(ctx context.Context, cl *client.Client, in PresetInput) error {
			return cl.Base().RunPreset(ctx, in.Name)
		})
	action(reg, b, "base.test", "", "Run a base vibration test", "run a base vibration test",
		func(ctx context.Context, cl *client.Client, _ NoInput) error { return cl.Base().VibrationTest(ctx) })

	write(reg, b, "autopilot.level_suggestions", "autopilot level-suggestions", "Turn level suggestions on or off", op.Write, nil,
		func(in EnabledInput) string { return fmt.Sprintf("set level suggestions enabled=%t", in.on()) },
		func(ctx context.Context, cl *client.Client, in EnabledInput) error {
			return cl.Autopilot().SetLevelSuggestions(ctx, in.on())
		})
	write(reg, b, "autopilot.snore_mitigation", "autopilot snore-mitigation", "Turn snore mitigation on or off", op.Write, nil,
		func(in EnabledInput) string { return fmt.Sprintf("set snore mitigation enabled=%t", in.on()) },
		func(ctx context.Context, cl *client.Client, in EnabledInput) error {
			return cl.Autopilot().SetSnoreMitigation(ctx, in.on())
		})

	write(reg, b, "travel.create_trip", "travel create-trip", "Create a trip", op.Write,
		func(in CreateTripInput) error {
			if len(in.body()) == 0 {
				return usage("provide at least --destination or --start-date/--end-date")
			}
			return nil
		},
		func(in CreateTripInput) string { return "create a trip to " + in.Destination },
		func(ctx context.Context, cl *client.Client, in CreateTripInput) error {
			return cl.Travel().CreateTrip(ctx, in.body())
		})
	write(reg, b, "travel.delete_trip", "travel delete-trip", "Delete a trip", op.Destructive,
		func(in TripInput) error { return required("trip")(in.Trip) },
		func(in TripInput) string { return "delete trip " + in.Trip },
		func(ctx context.Context, cl *client.Client, in TripInput) error {
			return cl.Travel().DeleteTrip(ctx, in.Trip)
		})
	write(reg, b, "travel.create_plan", "travel create-plan", "Add a plan to a trip", op.Write,
		func(in CreatePlanInput) error { return required("trip")(in.Trip) },
		func(in CreatePlanInput) string { return "add a plan to trip " + in.Trip },
		func(ctx context.Context, cl *client.Client, in CreatePlanInput) error {
			return cl.Travel().CreatePlan(ctx, in.Trip, in.body())
		})
	write(reg, b, "travel.update_plan", "travel update-plan", "Update a travel plan", op.Write,
		func(in UpdatePlanInput) error {
			if err := required("plan")(in.Plan); err != nil {
				return err
			}
			if len(in.body()) == 0 {
				return usage("no fields to update")
			}
			return nil
		},
		func(in UpdatePlanInput) string { return "update plan " + in.Plan },
		func(ctx context.Context, cl *client.Client, in UpdatePlanInput) error {
			return cl.Travel().UpdatePlan(ctx, in.Plan, in.body())
		})
}

func (t CreateTripInput) body() map[string]any {
	body := map[string]any{}
	for k, v := range map[string]string{"destination": t.Destination, "startDate": t.StartDate, "endDate": t.EndDate, "timezone": t.TripTimezone} {
		if v != "" {
			body[k] = v
		}
	}
	return body
}
