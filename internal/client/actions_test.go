package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

func newRecordingClient(t *testing.T) (*Client, *[]recordedRequest, func()) {
	t.Helper()
	records := []recordedRequest{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		records = append(records, recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.Query(),
			Body:   string(body),
		})
		w.Header().Set("Content-Type", "application/json")
		writeActionResponse(t, w, r)
	})
	srv := httptest.NewServer(mux)

	oldAppAPIBaseURL := appAPIBaseURL
	appAPIBaseURL = srv.URL

	c := New("email", "pass", "uid", "", "")
	c.DeviceID = "dev"
	c.BaseURL = srv.URL
	c.AppURL = srv.URL
	c.token = "tok"
	c.tokenExp = time.Now().Add(time.Hour)
	c.HTTP = srv.Client()

	cleanup := func() {
		appAPIBaseURL = oldAppAPIBaseURL
		srv.Close()
	}
	return c, &records, cleanup
}

func writeActionResponse(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	switch {
	case r.URL.Path == "/users/uid/alarms" && r.Method == http.MethodGet:
		io.WriteString(w, `{"alarms":[{"id":"alarm-1","time":"07:00","enabled":true,"daysOfWeek":[1],"vibration":true}]}`)
	case strings.HasPrefix(r.URL.Path, "/users/uid/alarms") && (r.Method == http.MethodPost || r.Method == http.MethodPatch):
		io.WriteString(w, `{"alarm":{"id":"alarm-1","time":"07:00","enabled":true}}`)
	case r.URL.Path == "/users/uid/audio/tracks" || r.URL.Path == "/audio/tracks":
		io.WriteString(w, `{"tracks":[{"id":"track-1","title":"Rain","type":"sound"}]}`)
	case r.URL.Path == "/users/uid/temperature":
		io.WriteString(w, `{"smart":{"enabled":true}}`)
	case r.URL.Path == "/v1/users/uid/temperature":
		io.WriteString(w, `{"currentLevel":12,"currentState":{"type":"smart"}}`)
	case r.URL.Path == "/users/uid/trends":
		io.WriteString(w, `{"days":[{"day":"2026-04-22","score":88}]}`)
	default:
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
}

func TestClientActionEndpoints(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		call      func(context.Context, *Client) error
		want      recordedRequest
		wantQuery map[string]string
		bodyHas   string
	}{
		{
			name: "ListAlarms",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ListAlarms(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/alarms"},
		},
		{
			name: "CreateAlarm",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateAlarm(ctx, Alarm{Time: "07:00", Enabled: true})
				return err
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/alarms"},
			bodyHas: `"time":"07:00"`,
		},
		{
			name: "UpdateAlarm",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateAlarm(ctx, "alarm-1", map[string]any{"enabled": false})
				return err
			},
			want:    recordedRequest{Method: http.MethodPatch, Path: "/users/uid/alarms/alarm-1"},
			bodyHas: `"enabled":false`,
		},
		{
			name: "DeleteAlarm",
			call: func(ctx context.Context, c *Client) error {
				return c.DeleteAlarm(ctx, "alarm-1")
			},
			want: recordedRequest{Method: http.MethodDelete, Path: "/users/uid/alarms/alarm-1"},
		},
		{
			name: "AlarmSnooze",
			call: func(ctx context.Context, c *Client) error {
				return c.Alarms().Snooze(ctx, "alarm-1")
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/alarms/alarm-1/snooze"},
		},
		{
			name: "AlarmDismiss",
			call: func(ctx context.Context, c *Client) error {
				return c.Alarms().Dismiss(ctx, "alarm-1")
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/alarms/alarm-1/dismiss"},
		},
		{
			name: "AlarmDismissAll",
			call: func(ctx context.Context, c *Client) error {
				return c.Alarms().DismissAll(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/alarms/active/dismiss-all"},
		},
		{
			name: "AlarmVibrationTest",
			call: func(ctx context.Context, c *Client) error {
				return c.Alarms().VibrationTest(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/vibration-test"},
		},
		{
			name: "AudioTracks",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Audio().Tracks(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/audio/tracks"},
		},
		{
			name: "AudioCategories",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Audio().Categories(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/audio/categories"},
		},
		{
			name: "AudioPlayerState",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Audio().PlayerState(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/audio/player/state"},
		},
		{
			name: "AudioPlay",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().Play(ctx, "track-1")
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/audio/player"},
			bodyHas: `"trackId":"track-1"`,
		},
		{
			name: "AudioPause",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().Pause(ctx)
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/audio/player"},
			bodyHas: `"action":"pause"`,
		},
		{
			name: "AudioSeek",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().Seek(ctx, 1200)
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/audio/player/seek"},
			bodyHas: `"position":1200`,
		},
		{
			name: "AudioVolume",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().Volume(ctx, 44)
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/audio/player/volume"},
			bodyHas: `"level":44`,
		},
		{
			name: "AudioPair",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().Pair(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/devices/dev/audio/player/pair"},
		},
		{
			name: "AudioRecommendedNext",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Audio().RecommendedNext(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/audio/tracks/recommended-next-track"},
		},
		{
			name: "AudioFavorites",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Audio().Favorites(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/audio/tracks/favorites"},
		},
		{
			name: "AudioAddFavorite",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().AddFavorite(ctx, "track-1")
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/audio/tracks/favorites"},
			bodyHas: `"trackId":"track-1"`,
		},
		{
			name: "AudioRemoveFavorite",
			call: func(ctx context.Context, c *Client) error {
				return c.Audio().RemoveFavorite(ctx, "track-1")
			},
			want: recordedRequest{Method: http.MethodDelete, Path: "/users/uid/audio/tracks/favorites/track-1"},
		},
		{
			name: "AutopilotDetails",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Autopilot().Details(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/autopilotDetails"},
		},
		{
			name: "AutopilotHistory",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Autopilot().History(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/autopilot-history"},
		},
		{
			name: "AutopilotRecap",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Autopilot().Recap(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/autopilotDetails/autopilotRecap"},
		},
		{
			name: "AutopilotSetLevelSuggestions",
			call: func(ctx context.Context, c *Client) error {
				return c.Autopilot().SetLevelSuggestions(ctx, true)
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/level-suggestions-mode"},
			bodyHas: `"enabled":true`,
		},
		{
			name: "AutopilotSetSnoreMitigation",
			call: func(ctx context.Context, c *Client) error {
				return c.Autopilot().SetSnoreMitigation(ctx, false)
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/autopilotDetails/snoringMitigation"},
			bodyHas: `"enabled":false`,
		},
		{
			name: "BaseInfo",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Base().Info(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/base"},
		},
		{
			name: "BaseSetAngle",
			call: func(ctx context.Context, c *Client) error {
				return c.Base().SetAngle(ctx, 3, 4)
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/base/angle"},
			bodyHas: `"foot":4`,
		},
		{
			name: "BasePresets",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Base().Presets(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/base/presets"},
		},
		{
			name: "BaseRunPreset",
			call: func(ctx context.Context, c *Client) error {
				return c.Base().RunPreset(ctx, "flat")
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/base/presets"},
			bodyHas: `"name":"flat"`,
		},
		{
			name: "BaseVibrationTest",
			call: func(ctx context.Context, c *Client) error {
				return c.Base().VibrationTest(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/devices/dev/vibration-test"},
		},
		{
			name: "DeviceInfo",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().Info(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev"},
		},
		{
			name: "DevicePeripherals",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().Peripherals(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev/peripherals"},
		},
		{
			name: "DeviceOwner",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().Owner(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev/owner"},
		},
		{
			name: "DeviceWarranty",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().Warranty(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev/warranty"},
		},
		{
			name: "DeviceOnline",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().Online(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev/online"},
		},
		{
			name: "DevicePrimingTasks",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().PrimingTasks(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev/priming/tasks"},
		},
		{
			name: "DevicePrimingSchedule",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Device().PrimingSchedule(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/devices/dev/priming/schedule"},
		},
		{
			name: "HouseholdSummary",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Household().Summary(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/household/users/uid/summary"},
		},
		{
			name: "HouseholdSchedule",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Household().Schedule(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/household/users/uid/schedule"},
		},
		{
			name: "HouseholdCurrentSet",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Household().CurrentSet(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/household/users/uid/current-set"},
		},
		{
			name: "HouseholdInvitations",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Household().Invitations(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/household/users/uid/invitations"},
		},
		{
			name: "HouseholdDevices",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Household().Devices(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/household/users/uid/summary"},
		},
		{
			name: "HouseholdGuests",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Household().Guests(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/household/users/uid/guests"},
		},
		{
			name: "MetricsTrends",
			call: func(ctx context.Context, c *Client) error {
				var out any
				return c.Metrics().Trends(ctx, "2026-04-01", "2026-04-02", "UTC", &out)
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/trends"},
			wantQuery: map[string]string{
				"from":                 "2026-04-01",
				"to":                   "2026-04-02",
				"tz":                   "UTC",
				"include-main":         "false",
				"include-all-sessions": "true",
				"model-version":        "v2",
			},
		},
		{
			name: "MetricsIntervals",
			call: func(ctx context.Context, c *Client) error {
				var out any
				return c.Metrics().Intervals(ctx, "session-1", &out)
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/intervals/session-1"},
		},
		{
			name: "MetricsInsights",
			call: func(ctx context.Context, c *Client) error {
				var out any
				return c.Metrics().Insights(ctx, &out)
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/insights"},
		},
		{
			name: "GetSmartSchedule",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetSmartSchedule(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/temperature"},
		},
		{
			name: "TempModeNapActivate",
			call: func(ctx context.Context, c *Client) error {
				return c.TempModes().NapActivate(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/temperature/nap-mode/activate"},
		},
		{
			name: "TempModeNapDeactivate",
			call: func(ctx context.Context, c *Client) error {
				return c.TempModes().NapDeactivate(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/temperature/nap-mode/deactivate"},
		},
		{
			name: "TempModeNapExtend",
			call: func(ctx context.Context, c *Client) error {
				return c.TempModes().NapExtend(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/temperature/nap-mode/extend"},
		},
		{
			name: "TempModeNapStatus",
			call: func(ctx context.Context, c *Client) error {
				var out any
				return c.TempModes().NapStatus(ctx, &out)
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/temperature/nap-mode/status"},
		},
		{
			name: "TempModeHotFlashActivate",
			call: func(ctx context.Context, c *Client) error {
				return c.TempModes().HotFlashActivate(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/temperature/hot-flash-mode/activate"},
		},
		{
			name: "TempModeHotFlashDeactivate",
			call: func(ctx context.Context, c *Client) error {
				return c.TempModes().HotFlashDeactivate(ctx)
			},
			want: recordedRequest{Method: http.MethodPost, Path: "/users/uid/temperature/hot-flash-mode/deactivate"},
		},
		{
			name: "TempModeHotFlashStatus",
			call: func(ctx context.Context, c *Client) error {
				var out any
				return c.TempModes().HotFlashStatus(ctx, &out)
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/temperature/hot-flash-mode"},
		},
		{
			name: "TempEvents",
			call: func(ctx context.Context, c *Client) error {
				var out any
				return c.TempModes().TempEvents(ctx, "2026-04-01", "2026-04-02", &out)
			},
			want:      recordedRequest{Method: http.MethodGet, Path: "/users/uid/temp-events"},
			wantQuery: map[string]string{"from": "2026-04-01", "to": "2026-04-02"},
		},
		{
			name: "TravelTrips",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Travel().Trips(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/travel/trips"},
		},
		{
			name: "TravelCreateTrip",
			call: func(ctx context.Context, c *Client) error {
				return c.Travel().CreateTrip(ctx, map[string]any{"destination": "Vienna"})
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/travel/trips"},
			bodyHas: `"destination":"Vienna"`,
		},
		{
			name: "TravelCreatePlan",
			call: func(ctx context.Context, c *Client) error {
				return c.Travel().CreatePlan(ctx, "trip-1", map[string]any{"name": "Plan"})
			},
			want:    recordedRequest{Method: http.MethodPost, Path: "/users/uid/travel/trips/trip-1/plans"},
			bodyHas: `"name":"Plan"`,
		},
		{
			name: "TravelUpdatePlan",
			call: func(ctx context.Context, c *Client) error {
				return c.Travel().UpdatePlan(ctx, "plan-1", map[string]any{"name": "Updated"})
			},
			want:    recordedRequest{Method: http.MethodPatch, Path: "/users/uid/travel/plans/plan-1"},
			bodyHas: `"name":"Updated"`,
		},
		{
			name: "TravelDeleteTrip",
			call: func(ctx context.Context, c *Client) error {
				return c.Travel().DeleteTrip(ctx, "trip-1")
			},
			want: recordedRequest{Method: http.MethodDelete, Path: "/users/uid/travel/trips/trip-1"},
		},
		{
			name: "TravelPlans",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Travel().Plans(ctx, "trip-1")
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/travel/trips/trip-1/plans"},
		},
		{
			name: "TravelPlanTasks",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Travel().PlanTasks(ctx, "plan-1")
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/travel/plans/plan-1/tasks"},
		},
		{
			name: "TravelAirportSearch",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Travel().AirportSearch(ctx, "VIE")
				return err
			},
			want:      recordedRequest{Method: http.MethodGet, Path: "/travel/airport-search"},
			wantQuery: map[string]string{"query": "VIE"},
		},
		{
			name: "TravelFlightStatus",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Travel().FlightStatus(ctx, "OS88")
				return err
			},
			want:      recordedRequest{Method: http.MethodGet, Path: "/travel/flight-status"},
			wantQuery: map[string]string{"flightNumber": "OS88"},
		},
		{
			name: "TurnOffForUser",
			call: func(ctx context.Context, c *Client) error {
				return c.TurnOffForUser(ctx, "uid")
			},
			want:    recordedRequest{Method: http.MethodPut, Path: "/v1/users/uid/temperature"},
			bodyHas: `"type":"off"`,
		},
		{
			name: "GetStatusForUser",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetStatusForUser(ctx, "uid")
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/v1/users/uid/temperature"},
		},
		{
			name: "GetSleepDay",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetSleepDay(ctx, "2026-04-22", "UTC")
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/users/uid/trends"},
			wantQuery: map[string]string{
				"from":                 "2026-04-22",
				"to":                   "2026-04-22",
				"tz":                   "UTC",
				"include-main":         "false",
				"include-all-sessions": "true",
				"model-version":        "v2",
			},
		},
		{
			name: "ListTracks",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ListTracks(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/audio/tracks"},
		},
		{
			name: "ReleaseFeatures",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReleaseFeatures(ctx)
				return err
			},
			want: recordedRequest{Method: http.MethodGet, Path: "/release/features"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, records, cleanup := newRecordingClient(t)
			defer cleanup()
			if err := tt.call(ctx, c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if len(*records) == 0 {
				t.Fatalf("no requests recorded")
			}
			got := (*records)[len(*records)-1]
			if got.Method != tt.want.Method || got.Path != tt.want.Path {
				t.Fatalf("request = %s %s, want %s %s", got.Method, got.Path, tt.want.Method, tt.want.Path)
			}
			for key, want := range tt.wantQuery {
				if got.Query.Get(key) != want {
					t.Fatalf("query %s = %q, want %q; full query %s", key, got.Query.Get(key), want, got.Query.Encode())
				}
			}
			if tt.bodyHas != "" && !strings.Contains(got.Body, tt.bodyHas) {
				t.Fatalf("body = %q, want substring %q", got.Body, tt.bodyHas)
			}
		})
	}
}

func TestGetSmartScheduleMissing(t *testing.T) {
	c, _, cleanup := newRecordingClient(t)
	defer cleanup()

	old := appAPIBaseURL
	appAPIBaseURL = c.AppURL + "/missing"
	defer func() { appAPIBaseURL = old }()

	got, err := c.GetSmartSchedule(context.Background())
	if err != ErrNoSmartSchedule {
		t.Fatalf("err = %v, want ErrNoSmartSchedule", err)
	}
	if got != nil {
		t.Fatalf("schedule = %#v, want nil", got)
	}
}

func TestClientConvenienceAndErrorBranches(t *testing.T) {
	ctx := context.Background()
	c, _, cleanup := newRecordingClient(t)
	defer cleanup()

	if err := c.TurnOn(ctx); err != nil {
		t.Fatalf("TurnOn: %v", err)
	}
	if err := c.TurnOff(ctx); err != nil {
		t.Fatalf("TurnOff: %v", err)
	}
	if err := c.SetTemperature(ctx, 7); err != nil {
		t.Fatalf("SetTemperature: %v", err)
	}
	if err := c.SetTemperature(ctx, 101); err == nil {
		t.Fatalf("expected invalid temperature error")
	}
	if _, err := c.GetStatus(ctx); err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
}

func TestResolveTZ(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.FixedZone("Europe/Vienna", 3600)
	t.Cleanup(func() { time.Local = oldLocal })

	if got := resolveTZ("America/New_York"); got != "America/New_York" {
		t.Fatalf("explicit timezone = %q", got)
	}
	if got := resolveTZ(""); got != "Europe/Vienna" {
		t.Fatalf("empty timezone = %q", got)
	}
	time.Local = time.FixedZone("Local", 0)
	if got := resolveTZ("local"); got != "UTC" {
		t.Fatalf("unresolved local timezone = %q", got)
	}
}

func TestIdentityDisplayHelpers(t *testing.T) {
	target := HouseholdUserTarget{UserID: "uid", FirstName: "Ada", LastName: "Lovelace", Side: "left"}
	if got := target.DisplayName(); got != "Ada Lovelace" {
		t.Fatalf("DisplayName = %q", got)
	}
	if got := target.SideLabel(); got != "left" {
		t.Fatalf("SideLabel = %q", got)
	}
	if got := (HouseholdUserTarget{UserID: "uid"}).DisplayName(); got != "uid" {
		t.Fatalf("fallback DisplayName = %q", got)
	}
	if got := (HouseholdUserTarget{}).SideLabel(); got != "unknown" {
		t.Fatalf("fallback SideLabel = %q", got)
	}
}

func TestEnsureUserAndDeviceErrorBranches(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"user":{}}`)
	}))
	defer srv.Close()

	c := New("email", "pass", "", "", "")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	c.token = "tok"
	c.tokenExp = time.Now().Add(time.Hour)

	if err := c.EnsureUserID(ctx); err == nil {
		t.Fatalf("expected missing userId error")
	}
	if _, err := c.EnsureDeviceID(ctx); err == nil {
		t.Fatalf("expected missing device error")
	}
}

func TestEnsureDeviceIDFallsBackToDevicesList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"user":{"devices":["dev-from-list"]}}`)
	}))
	defer srv.Close()

	c := New("email", "pass", "uid", "", "")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	c.token = "tok"
	c.tokenExp = time.Now().Add(time.Hour)

	got, err := c.EnsureDeviceID(context.Background())
	if err != nil {
		t.Fatalf("EnsureDeviceID: %v", err)
	}
	if got != "dev-from-list" {
		t.Fatalf("device id = %q", got)
	}
}

func TestDeviceOwnerFallback(t *testing.T) {
	var ownerCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/devices/dev/owner":
			ownerCalls++
			http.Error(w, "missing", http.StatusNotFound)
		case "/devices/dev":
			io.WriteString(w, `{"result":{"ownerId":"owner-1"}}`)
		default:
			io.WriteString(w, `{}`)
		}
	}))
	defer srv.Close()

	c := New("email", "pass", "uid", "", "")
	c.DeviceID = "dev"
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	c.token = "tok"
	c.tokenExp = time.Now().Add(time.Hour)

	got, err := c.Device().Owner(context.Background())
	if err != nil {
		t.Fatalf("Owner: %v", err)
	}
	if ownerCalls != 1 {
		t.Fatalf("owner calls = %d", ownerCalls)
	}
	owner := got.(map[string]any)["ownerId"]
	if owner != "owner-1" {
		t.Fatalf("owner = %v", owner)
	}
}

func TestGetSleepDayNoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"days":[]}`)
	}))
	defer srv.Close()

	c := New("email", "pass", "uid", "", "")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	c.token = "tok"
	c.tokenExp = time.Now().Add(time.Hour)

	if _, err := c.GetSleepDay(context.Background(), "2026-04-22", "UTC"); err == nil {
		t.Fatalf("expected no sleep data error")
	}
}

func TestAuthTokenEndpointInvalidResponses(t *testing.T) {
	tests := map[string]string{
		"invalid json": "{",
		"empty token":  `{"access_token":""}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, body)
			}))
			defer srv.Close()

			old := authURL
			authURL = srv.URL
			defer func() { authURL = old }()

			c := New("email", "pass", "", "", "")
			c.HTTP = srv.Client()
			if err := c.Authenticate(context.Background()); err == nil {
				t.Fatalf("expected auth error")
			}
		})
	}
}

func TestClientActionsPropagateRequireUserError(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"user":{}}`)
	}))
	defer srv.Close()

	tests := map[string]func(context.Context, *Client) error{
		"ListAlarms": func(ctx context.Context, c *Client) error {
			_, err := c.ListAlarms(ctx)
			return err
		},
		"CreateAlarm": func(ctx context.Context, c *Client) error {
			_, err := c.CreateAlarm(ctx, Alarm{})
			return err
		},
		"UpdateAlarm": func(ctx context.Context, c *Client) error {
			_, err := c.UpdateAlarm(ctx, "alarm", nil)
			return err
		},
		"DeleteAlarm": func(ctx context.Context, c *Client) error {
			return c.DeleteAlarm(ctx, "alarm")
		},
		"AlarmSnooze": func(ctx context.Context, c *Client) error {
			return c.Alarms().Snooze(ctx, "alarm")
		},
		"AudioTracks": func(ctx context.Context, c *Client) error {
			_, err := c.Audio().Tracks(ctx)
			return err
		},
		"AudioPlay": func(ctx context.Context, c *Client) error {
			return c.Audio().Play(ctx, "")
		},
		"AutopilotDetails": func(ctx context.Context, c *Client) error {
			_, err := c.Autopilot().Details(ctx)
			return err
		},
		"BaseInfo": func(ctx context.Context, c *Client) error {
			_, err := c.Base().Info(ctx)
			return err
		},
		"HouseholdSummary": func(ctx context.Context, c *Client) error {
			_, err := c.Household().Summary(ctx)
			return err
		},
		"MetricsTrends": func(ctx context.Context, c *Client) error {
			var out any
			return c.Metrics().Trends(ctx, "", "", "", &out)
		},
		"TempEvents": func(ctx context.Context, c *Client) error {
			var out any
			return c.TempModes().TempEvents(ctx, "", "", &out)
		},
		"TravelTrips": func(ctx context.Context, c *Client) error {
			_, err := c.Travel().Trips(ctx)
			return err
		},
		"SetAwayMode": func(ctx context.Context, c *Client) error {
			return c.SetAwayMode(ctx, "", true)
		},
		"GetSleepDay": func(ctx context.Context, c *Client) error {
			_, err := c.GetSleepDay(ctx, "2026-04-22", "UTC")
			return err
		},
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			c := New("email", "pass", "", "", "")
			c.BaseURL = srv.URL
			c.AppURL = srv.URL
			c.HTTP = srv.Client()
			c.token = "tok"
			c.tokenExp = time.Now().Add(time.Hour)
			if err := call(ctx, c); err == nil {
				t.Fatalf("expected requireUser error")
			}
		})
	}
}

func TestHouseholdUsersAndSideResolution(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/devices/dev":
			io.WriteString(w, `{"result":{"leftUserId":"left-user","rightUserId":"right-user"}}`)
		case "/users/left-user":
			io.WriteString(w, `{"user":{"userId":"left-user","firstName":"Left","lastName":"Side","email":"left@example.com","currentDevice":{"side":"away"}}}`)
		case "/users/right-user":
			io.WriteString(w, `{"user":{"userId":"right-user","firstName":"Right","lastName":"Side","email":"right@example.com","currentDevice":{"side":"right"}}}`)
		default:
			io.WriteString(w, `{}`)
		}
	}))
	defer srv.Close()

	c := New("email", "pass", "uid", "", "")
	c.DeviceID = "dev"
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	c.token = "tok"
	c.tokenExp = time.Now().Add(time.Hour)

	users, err := c.Household().Users(context.Background())
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	rows := users.([]map[string]any)
	if len(rows) != 2 || rows[0]["side"] != "left" || rows[1]["side"] != "right" {
		t.Fatalf("users = %#v", rows)
	}
	if len(paths) != 3 {
		t.Fatalf("paths = %#v", paths)
	}
	if got := resolveTargetSide("", "RIGHT"); got != "right" {
		t.Fatalf("resolveTargetSide = %q", got)
	}
	if got := resolveTargetSide("", "away"); got != "" {
		t.Fatalf("away sentinel side = %q", got)
	}
}
