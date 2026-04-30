package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
