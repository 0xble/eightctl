package ops

import (
	"context"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
	"github.com/0xble/eightsleep/internal/config"
)

// TrackRow is one audio track.
type TrackRow struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// FeatureRow is one release feature.
type FeatureRow struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// TrendsInput is the window of metrics trends.
type TrendsInput struct {
	Zone
	DateRange
}

// SessionInput names a sleep session.
type SessionInput struct {
	ID string `json:"id,omitempty" help:"session id"`
}

// TripInput names a trip.
type TripInput struct {
	Trip string `json:"trip,omitempty" help:"trip id"`
}

// PlanInput names a travel plan.
type PlanInput struct {
	Plan string `json:"plan,omitempty" help:"plan id"`
}

// AirportInput is an airport search.
type AirportInput struct {
	Query string `json:"query,omitempty" help:"airport query"`
}

// FlightInput names a flight.
type FlightInput struct {
	Flight string `json:"flight,omitempty" help:"flight number"`
}

type fetchFunc = func(ctx context.Context, cl *client.Client, s config.Settings, in NoInput) (any, error)

// simple adapts a client call without parameters.
func simple(get func(context.Context, *client.Client) (any, error)) fetchFunc {
	return func(ctx context.Context, cl *client.Client, _ config.Settings, _ NoInput) (any, error) { return get(ctx, cl) }
}

// into adapts a client call that decodes into a value.
func into(get func(context.Context, *client.Client, any) error) fetchFunc {
	return func(ctx context.Context, cl *client.Client, _ config.Settings, _ NoInput) (any, error) {
		var out any
		err := get(ctx, cl, &out)
		return out, err
	}
}

func registerReads(reg *op.Registry, b *Backend) {
	tracks := func(name, cli, summary string) {
		rows(reg, b, op.Op[NoInput, []TrackRow]{
			Name: name, CLI: cli, Summary: summary,
			Handler: func(ctx context.Context, req op.Request, _ NoInput) ([]TrackRow, error) {
				cl, _, err := b.client(req)
				if err != nil {
					return nil, err
				}
				ts, err := cl.Audio().Tracks(ctx)
				if err != nil {
					return nil, providerErr(err)
				}
				out := make([]TrackRow, 0, len(ts))
				for _, t := range ts {
					out = append(out, TrackRow(t))
				}
				return out, nil
			},
		}, "id", "title", "type")
	}
	tracks("tracks", "", "List audio tracks")
	tracks("audio.tracks", "", "List audio tracks")

	rows(reg, b, op.Op[NoInput, []FeatureRow]{
		Name: "feats", Summary: "List release features",
		Handler: func(ctx context.Context, req op.Request, _ NoInput) ([]FeatureRow, error) {
			cl, _, err := b.client(req)
			if err != nil {
				return nil, err
			}
			fs, err := cl.ReleaseFeatures(ctx)
			if err != nil {
				return nil, providerErr(err)
			}
			out := make([]FeatureRow, 0, len(fs))
			for _, f := range fs {
				out = append(out, FeatureRow(f))
			}
			return out, nil
		},
	}, "title", "body")

	type read struct {
		name, cli, summary, column string
		fetch                      fetchFunc
	}
	for _, r := range []read{
		{"audio.categories", "", "List audio categories", "data", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Audio().Categories(ctx) })},
		{"audio.state", "", "Show the audio player state", "state", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Audio().PlayerState(ctx) })},
		{"audio.next", "", "Show the recommended next track", "next", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Audio().RecommendedNext(ctx) })},
		{"audio.favorites.list", "", "List favorite tracks", "favorites", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Audio().Favorites(ctx) })},
		{"base.info", "", "Show the adjustable base", "info", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Base().Info(ctx) })},
		{"base.presets", "", "List base presets", "presets", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Base().Presets(ctx) })},
		{"device.info", "", "Show device info", "info", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().Info(ctx) })},
		{"device.peripherals", "", "List device peripherals", "peripherals", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().Peripherals(ctx) })},
		{"device.owner", "", "Show the device owner", "owner", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().Owner(ctx) })},
		{"device.warranty", "", "Show the device warranty", "warranty", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().Warranty(ctx) })},
		{"device.online", "", "Show whether the device is online", "online", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().Online(ctx) })},
		{"device.priming_tasks", "device priming-tasks", "List priming tasks", "priming-tasks", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().PrimingTasks(ctx) })},
		{"device.priming_schedule", "device priming-schedule", "Show the priming schedule", "priming-schedule", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Device().PrimingSchedule(ctx) })},
		{"metrics.summary", "", "Show the metrics summary", "summary", into(func(ctx context.Context, cl *client.Client, out any) error { return cl.Metrics().Summary(ctx, out) })},
		{"metrics.aggregate", "", "Show aggregate metrics", "aggregate", into(func(ctx context.Context, cl *client.Client, out any) error { return cl.Metrics().Aggregate(ctx, out) })},
		{"metrics.insights", "", "Show sleep insights", "insights", into(func(ctx context.Context, cl *client.Client, out any) error { return cl.Metrics().Insights(ctx, out) })},
		{"autopilot.details", "", "Show Autopilot details", "details", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Autopilot().Details(ctx) })},
		{"autopilot.history", "", "Show Autopilot history", "history", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Autopilot().History(ctx) })},
		{"autopilot.recap", "", "Show the Autopilot recap", "recap", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Autopilot().Recap(ctx) })},
		{"travel.trips", "", "List trips", "trips", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Travel().Trips(ctx) })},
		{"household.summary", "", "Show the household summary", "summary", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().Summary(ctx) })},
		{"household.schedule", "", "Show the household schedule", "schedule", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().Schedule(ctx) })},
		{"household.current_set", "household current-set", "Show the household's current set", "current-set", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().CurrentSet(ctx) })},
		{"household.invitations", "", "List household invitations", "invitations", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().Invitations(ctx) })},
		{"household.devices", "", "List household devices", "devices", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().Devices(ctx) })},
		{"household.users", "", "List household users", "users", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().Users(ctx) })},
		{"household.guests", "", "List household guests", "guests", simple(func(ctx context.Context, cl *client.Client) (any, error) { return cl.Household().Guests(ctx) })},
	} {
		raw(reg, b, r.name, r.cli, r.summary, r.column, r.fetch)
	}

	raw(reg, b, "metrics.trends", "", "Show sleep trends", "trends",
		func(ctx context.Context, cl *client.Client, s config.Settings, in TrendsInput) (any, error) {
			var out any
			err := cl.Metrics().Trends(ctx, in.From, in.To, b.apiZone(in.Timezone, s.Timezone), &out)
			return out, err
		})
	raw(reg, b, "metrics.intervals", "", "Show a sleep session's intervals", "interval",
		func(ctx context.Context, cl *client.Client, _ config.Settings, in SessionInput) (any, error) {
			var out any
			err := cl.Metrics().Intervals(ctx, in.ID, &out)
			return out, err
		})
	raw(reg, b, "travel.plans", "", "List a trip's plans", "plans",
		func(ctx context.Context, cl *client.Client, _ config.Settings, in TripInput) (any, error) {
			return cl.Travel().Plans(ctx, in.Trip)
		})
	raw(reg, b, "travel.tasks", "", "List a plan's tasks", "tasks",
		func(ctx context.Context, cl *client.Client, _ config.Settings, in PlanInput) (any, error) {
			return cl.Travel().PlanTasks(ctx, in.Plan)
		})
	raw(reg, b, "travel.airport_search", "travel airport-search", "Search airports", "airports",
		func(ctx context.Context, cl *client.Client, _ config.Settings, in AirportInput) (any, error) {
			return cl.Travel().AirportSearch(ctx, in.Query)
		})
	raw(reg, b, "travel.flight_status", "travel flight-status", "Show a flight's status", "flight",
		func(ctx context.Context, cl *client.Client, _ config.Settings, in FlightInput) (any, error) {
			return cl.Travel().FlightStatus(ctx, in.Flight)
		})
}
