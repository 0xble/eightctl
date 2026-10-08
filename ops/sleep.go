package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
)

const dateLayout = "2006-01-02"

// Zone is the timezone input of a date-sensitive read. The root --timezone
// flag fills it on the command line; empty means the configured timezone.
type Zone struct {
	Timezone string `json:"timezone,omitempty" help:"IANA timezone (e.g., America/New_York) or 'local'"`
}

// DateRange is an optional date window.
type DateRange struct {
	From string `json:"from,omitempty" help:"from date YYYY-MM-DD"`
	To   string `json:"to,omitempty" help:"to date YYYY-MM-DD"`
}

// PresenceInput is the window presence looks at.
type PresenceInput struct {
	Zone
	DateRange
}

// PresenceRow says whether the user is in bed.
type PresenceRow struct {
	Present bool `json:"present"`
}

// PresenceDetailInput tunes eightsleepctl's presence heuristic. The windows
// are pointers with the defaults applied in windows(), so an explicit 0 from
// the flag, the environment or a request survives (toolkit gap G2).
type PresenceDetailInput struct {
	Zone
	PresentWithin *int `json:"present_window_seconds,omitempty" name:"present-within" env:"EIGHTSLEEPCTL_PRESENCE_MAX_AGE_SECONDS" help:"A signal newer than this many seconds means present (default 1200)"`
	AbsentAfter   *int `json:"absent_window_seconds,omitempty" name:"absent-after" env:"EIGHTSLEEPCTL_ABSENCE_MIN_AGE_SECONDS" help:"A signal at least this many seconds old means absent (default 7200)"`
}

// Default presence windows, eightsleepctl's.
const (
	defaultPresentWithin = 1200
	defaultAbsentAfter   = 7200
)

func (in PresenceDetailInput) windows() (present, absent int) {
	present, absent = defaultPresentWithin, defaultAbsentAfter
	if in.PresentWithin != nil {
		present = *in.PresentWithin
	}
	if in.AbsentAfter != nil {
		absent = *in.AbsentAfter
	}
	return present, absent
}

// PresenceDetail is eightsleepctl's presence estimate: the freshest
// biometric signal of the latest sleep session, and the device's state.
type PresenceDetail struct {
	// Present is null when the newest signal is between the two windows.
	Present *bool `json:"present"`
	// Reason is fresh_timeseries, stale_timeseries, ambiguous_timeseries or
	// no_timeseries.
	Reason               string          `json:"reason"`
	LastSignal           *Signal         `json:"last_signal,omitempty"`
	AgeSeconds           *int            `json:"age_seconds,omitempty"`
	PresentWindowSeconds *int            `json:"present_window_seconds,omitempty"`
	AbsentWindowSeconds  *int            `json:"absent_window_seconds,omitempty"`
	Device               *PresenceDevice `json:"device,omitempty"`
}

// Signal is the newest sample of one biometric series.
type Signal struct {
	Signal    string `json:"signal"`
	Timestamp string `json:"timestamp"`
	Value     any    `json:"value"`
}

// PresenceDevice is the current device's connectivity.
type PresenceDevice struct {
	ID        string   `json:"id"`
	Online    any      `json:"online"`
	LastHeard any      `json:"lastHeard"`
	Blanket   *Blanket `json:"blanket,omitempty"`
}

// Blanket is the cover's connection state.
type Blanket struct {
	IsConnected any `json:"isConnected"`
	Connectors  any `json:"connectors"`
}

// SleepDayInput selects a day.
type SleepDayInput struct {
	Zone
	Date string `json:"date,omitempty" help:"date YYYY-MM-DD (default today)"`
}

// SleepRangeInput selects a range of days.
type SleepRangeInput struct {
	Zone
	From string `json:"from,omitempty" help:"start date YYYY-MM-DD"`
	To   string `json:"to,omitempty" help:"end date YYYY-MM-DD"`
}

// SleepDayRow is one day's sleep metrics.
type SleepDayRow struct {
	Date          string  `json:"date"`
	Score         float64 `json:"score"`
	Tnt           int     `json:"tnt"`
	RespRate      float64 `json:"resp_rate"`
	HeartRate     float64 `json:"heart_rate"`
	Duration      float64 `json:"duration"`
	LatencyAsleep float64 `json:"latency_asleep"`
	LatencyOut    float64 `json:"latency_out"`
	HRVScore      float64 `json:"hrv_score"`
}

// SleepRangeRow is one day of a range.
type SleepRangeRow struct {
	Date      string  `json:"date"`
	Score     float64 `json:"score"`
	Duration  float64 `json:"duration"`
	Tnt       int     `json:"tnt"`
	RespRate  float64 `json:"resp_rate"`
	HeartRate float64 `json:"heart_rate"`
	HRVScore  float64 `json:"hrv_score"`
}

var signalKeys = []string{"heartRate", "hrv", "respiratoryRate", "tempBedC", "tempRoomC"}

func registerSleep(reg *op.Registry, b *Backend) {
	rows(reg, b, op.Op[PresenceInput, []PresenceRow]{
		Name: "presence", CLI: "presence check", Summary: "Check if user is in bed", DefaultCommand: true,
		Handler: func(ctx context.Context, req op.Request, in PresenceInput) ([]PresenceRow, error) {
			if err := validDates(in.From, in.To); err != nil {
				return nil, err
			}
			cl, s, err := b.client(req)
			if err != nil {
				return nil, err
			}
			present, err := cl.GetPresence(ctx, in.From, in.To, b.apiZone(in.Timezone, s.Timezone))
			if err != nil {
				return nil, providerErr(err)
			}
			return []PresenceRow{{Present: present}}, nil
		},
	}, "present")

	op.Add(reg, op.Op[PresenceDetailInput, PresenceDetail]{
		Name: "presence.detail", Summary: "Estimate presence from the freshest biometric signal, with device state",
		Effect: op.Read, MCP: true,
		Handler: func(ctx context.Context, req op.Request, in PresenceDetailInput) (PresenceDetail, error) {
			cl, s, err := b.client(req)
			if err != nil {
				return PresenceDetail{}, err
			}
			return b.presenceDetail(ctx, cl, in, s.Timezone)
		},
		Render: func(w io.Writer, d PresenceDetail) error {
			state := "unknown"
			if d.Present != nil {
				state = map[bool]string{true: "yes", false: "no"}[*d.Present]
			}
			line := fmt.Sprintf("present: %s (%s", state, d.Reason)
			if d.LastSignal != nil && d.AgeSeconds != nil {
				line += fmt.Sprintf(", %s %ds ago", d.LastSignal.Signal, *d.AgeSeconds)
			}
			_, err := fmt.Fprintln(w, line+")")
			return err
		},
	})

	rows(reg, b, op.Op[SleepDayInput, []SleepDayRow]{
		Name: "sleep.day", Summary: "Fetch sleep metrics for a date (YYYY-MM-DD)",
		Handler: func(ctx context.Context, req op.Request, in SleepDayInput) ([]SleepDayRow, error) {
			cl, s, err := b.client(req)
			if err != nil {
				return nil, err
			}
			tz := b.apiZone(in.Timezone, s.Timezone)
			date := in.Date
			if date == "" {
				loc, err := time.LoadLocation(tz)
				if err != nil {
					return nil, usage("load timezone %q: %v", tz, err)
				}
				date = b.now().In(loc).Format(time.DateOnly)
			}
			day, err := cl.GetSleepDay(ctx, date, tz)
			if err != nil {
				return nil, providerErr(err)
			}
			return []SleepDayRow{{Date: day.Date, Score: day.Score, Tnt: day.Tnt, RespRate: day.Respiratory, HeartRate: day.HeartRate,
				Duration: day.Duration, LatencyAsleep: day.LatencyAsleep, LatencyOut: day.LatencyOut, HRVScore: day.SleepQuality.HRV.Score}}, nil
		},
	}, "date", "score", "duration", "latency_asleep", "latency_out", "tnt", "resp_rate", "heart_rate", "hrv_score")

	rows(reg, b, op.Op[SleepRangeInput, []SleepRangeRow]{
		Name: "sleep.range", Summary: "Fetch sleep metrics for a date range",
		Handler: func(ctx context.Context, req op.Request, in SleepRangeInput) ([]SleepRangeRow, error) {
			if in.From == "" || in.To == "" {
				return nil, usage("--from and --to are required")
			}
			start, err := time.Parse(dateLayout, in.From)
			if err != nil {
				return nil, usage("%v", err)
			}
			end, err := time.Parse(dateLayout, in.To)
			if err != nil {
				return nil, usage("%v", err)
			}
			if end.Before(start) {
				return nil, usage("to must be >= from")
			}
			cl, s, err := b.client(req)
			if err != nil {
				return nil, err
			}
			tz := b.apiZone(in.Timezone, s.Timezone)
			out := []SleepRangeRow{}
			for d := start; !d.After(end); d = d.Add(24 * time.Hour) {
				day, err := cl.GetSleepDay(ctx, d.Format(dateLayout), tz)
				if err != nil {
					return nil, providerErr(err)
				}
				out = append(out, SleepRangeRow{Date: day.Date, Score: day.Score, Duration: day.Duration, Tnt: day.Tnt,
					RespRate: day.Respiratory, HeartRate: day.HeartRate, HRVScore: day.SleepQuality.HRV.Score})
			}
			return out, nil
		},
	}, "date", "score", "duration", "tnt", "resp_rate", "heart_rate", "hrv_score")

	op.Add(reg, op.Op[NoInput, []Row]{
		Name: "schedule.list", Summary: "Show the Autopilot schedule for the current user", Effect: op.Read, MCP: true,
		Handler: func(ctx context.Context, req op.Request, _ NoInput) ([]Row, error) {
			cl, _, err := b.client(req)
			if err != nil {
				return nil, err
			}
			smart, err := cl.GetSmartSchedule(ctx)
			if errors.Is(err, client.ErrNoSmartSchedule) {
				return []Row{}, nil
			}
			if err != nil {
				return nil, providerErr(err)
			}
			return []Row{{"smart": smart}}, nil
		},
		Render: func(w io.Writer, rs []Row) error {
			if len(rs) == 0 {
				_, err := fmt.Fprintln(w, "no Autopilot schedule configured for this user")
				return err
			}
			return b.printRows(w, []string{"smart"}, rs)
		},
	})
}

func validDates(from, to string) error {
	if from != "" {
		if _, err := time.Parse(dateLayout, from); err != nil {
			return usage("invalid --from date %q: %v", from, err)
		}
	}
	if to != "" {
		if _, err := time.Parse(dateLayout, to); err != nil {
			return usage("invalid --to date %q: %v", to, err)
		}
	}
	if from != "" && to != "" && to < from {
		return usage("--to must be >= --from")
	}
	return nil
}

// presenceDetail is eightsleepctl's heuristic: the newest sample among the
// heart rate, HRV, respiratory rate and temperature series of the latest
// session in the last two UTC days. A sample newer than PresentWithin means
// present, one at least AbsentAfter old means absent, and between is unknown.
func (b *Backend) presenceDetail(ctx context.Context, cl *client.Client, in PresenceDetailInput, configured string) (PresenceDetail, error) {
	now := b.now().UTC()
	tz := in.Timezone
	if tz == "" {
		tz = configured
	}
	if tz == "" || tz == "local" {
		tz = "UTC"
	}
	cd, err := cl.CurrentDevice(ctx)
	if err != nil {
		return PresenceDetail{}, providerErr(err)
	}
	var device *PresenceDevice
	if id, _ := cd["id"].(string); id != "" {
		dev, err := cl.DeviceStatus(ctx, id)
		if err != nil {
			return PresenceDetail{}, providerErr(err)
		}
		device = &PresenceDevice{ID: id, Online: dev["online"], LastHeard: dev["lastHeard"]}
		if sensor, ok := dev["sensorInfo"].(map[string]any); ok {
			if blanket, ok := sensor["blanket"].(map[string]any); ok {
				device.Blanket = &Blanket{IsConnected: blanket["isConnected"], Connectors: blanket["connectors"]}
			}
		}
	}
	days, err := cl.TrendDays(ctx, now.AddDate(0, 0, -1).Format(dateLayout), now.Format(dateLayout), tz)
	if err != nil {
		return PresenceDetail{}, providerErr(err)
	}
	var last *Signal
	var lastAt time.Time
	for i := len(days) - 1; i >= 0 && last == nil; i-- {
		day, _ := days[i].(map[string]any)
		sessions, _ := day["sessions"].([]any)
		if len(sessions) == 0 {
			continue
		}
		session, _ := sessions[len(sessions)-1].(map[string]any)
		series, _ := session["timeseries"].(map[string]any)
		if series == nil {
			continue
		}
		last, lastAt = latestSignal(series)
	}
	if last == nil {
		f := false
		return PresenceDetail{Present: &f, Reason: "no_timeseries", Device: device}, nil
	}
	age := int(now.Sub(lastAt).Seconds())
	present, absent := in.windows()
	d := PresenceDetail{LastSignal: last, AgeSeconds: &age, PresentWindowSeconds: &present, AbsentWindowSeconds: &absent, Device: device}
	switch {
	case now.Sub(lastAt) < time.Duration(present)*time.Second:
		t := true
		d.Present, d.Reason = &t, "fresh_timeseries"
	case now.Sub(lastAt) >= time.Duration(absent)*time.Second:
		f := false
		d.Present, d.Reason = &f, "stale_timeseries"
	default:
		d.Reason = "ambiguous_timeseries"
	}
	return d, nil
}

// latestSignal returns the newest of the last samples of each series. On a
// tie the later series in signalKeys wins, as eightsleepctl's stable sort did.
func latestSignal(series map[string]any) (*Signal, time.Time) {
	var best *Signal
	var bestAt time.Time
	for _, key := range signalKeys {
		samples, _ := series[key].([]any)
		if len(samples) == 0 {
			continue
		}
		sample, _ := samples[len(samples)-1].([]any)
		if len(sample) == 0 {
			continue
		}
		ts, ok := sample[0].(string)
		if !ok {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			continue
		}
		var value any
		if len(sample) > 1 {
			value = sample[1]
		}
		if best == nil || !at.Before(bestAt) {
			best, bestAt = &Signal{Signal: key, Timestamp: ts, Value: value}, at
		}
	}
	return best, bestAt
}

// apiZone resolves the timezone for API queries: the input, else the
// configured value, with "local" read from the system. The API rejects the
// literal "Local", so UTC is the last resort.
func (b *Backend) apiZone(input, configured string) string {
	tz := strings.TrimSpace(input)
	if tz == "" {
		tz = strings.TrimSpace(configured)
	}
	if tz != "" && !strings.EqualFold(tz, "local") {
		return tz
	}
	if zone := b.localZone(); zone != "" {
		return zone
	}
	if loc := strings.TrimSpace(time.Now().Location().String()); loc != "" && !strings.EqualFold(loc, "local") {
		return loc
	}
	slog.Warn("system local timezone is not an IANA zone; falling back to UTC for API queries")
	return "UTC"
}

func (b *Backend) localZone() string {
	if b.LocalZone != nil {
		return b.LocalZone()
	}
	if tz := strings.TrimSpace(b.getenv("TZ")); tz != "" && !strings.EqualFold(tz, "local") {
		return tz
	}
	switch runtime.GOOS {
	case "darwin":
		if target, err := os.Readlink("/etc/localtime"); err == nil {
			return zoneinfoSuffix(target)
		}
	case "linux":
		if data, err := os.ReadFile("/etc/timezone"); err == nil {
			if zone := strings.TrimSpace(string(data)); zone != "" {
				return zone
			}
		}
		if target, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
			return zoneinfoSuffix(target)
		}
	}
	return ""
}

func zoneinfoSuffix(path string) string {
	const marker = "zoneinfo/"
	if i := strings.Index(path, marker); i >= 0 {
		return path[i+len(marker):]
	}
	return ""
}
