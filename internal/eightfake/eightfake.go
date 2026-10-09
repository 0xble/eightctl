// Package eightfake is an httptest fake of the three Eight Sleep hosts (the
// client API, the app API and the token endpoint), served under the path
// prefixes /client-api, /app-api and /auth-api of one server. It records
// every request, keeps the bed's state, and answers with fixtures. Tests and
// the caller-compatibility goldens use it; nothing here reaches Eight Sleep.
package eightfake

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xble/eightsleep/internal/client"
)

// Fixture identities.
const (
	User    = "u1"
	Partner = "u2"
	Device  = "d1"
	Token   = "fixture-access-token"
)

// Request is one request as the goldens record it.
type Request struct {
	Method string `json:"method"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	Query  string `json:"query,omitempty"`
}

// Write is one applied change: a request that is not a GET.
type Write struct {
	Method string `json:"method"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	Body   any    `json:"body,omitempty"`
}

// Server is the fake.
type Server struct {
	*httptest.Server

	Mu       sync.Mutex
	Requests []Request
	Writes   []Write

	// Now is the clock trend samples are relative to. Nil means time.Now.
	Now func() time.Time
	// Solo makes the household one user on one side.
	Solo bool
	// Fail answers "METHOD host path" with a status and no fixture.
	Fail map[string]int
	// DismissAllStatus answers the bulk dismiss route, 0 meaning 200.
	DismissAllStatus int
	// NoSchedule leaves the user without an Autopilot schedule.
	NoSchedule bool
	// StaleSignals puts the newest trend sample three hours back.
	StaleSignals bool
	// WrongUser answers the partner's user record with another user's ID.
	WrongUser bool

	level map[string]int
	state map[string]string
	away  map[string]bool
}

// New starts a fake.
func New() *Server {
	s := &Server{
		level: map[string]int{User: -20, Partner: 10},
		state: map[string]string{User: "smart", Partner: "off"},
		away:  map[string]bool{User: false, Partner: true},
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// Hosts points a client at the fake.
func (s *Server) Hosts() client.Hosts {
	return client.Hosts{Client: s.URL + "/client-api", App: s.URL + "/app-api", Auth: s.URL + "/auth-api"}
}

// Snapshot is the fake's state for the conformance kit: the applied writes.
func (s *Server) Snapshot() any {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return append([]Write{}, s.Writes...)
}

// Recorded returns the requests so far.
func (s *Server) Recorded() []Request {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return append([]Request{}, s.Requests...)
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	host, path, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	path = "/" + path
	body, _ := io.ReadAll(r.Body)
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Requests = append(s.Requests, Request{Method: r.Method, Host: host, Path: path, Query: sortedQuery(r.URL.Query())})
	if status := s.Fail[r.Method+" "+host+" "+path]; status != 0 {
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"error":"fixture %d"}`, status)
		return
	}
	if host == "auth-api" {
		writeJSON(w, map[string]any{"access_token": Token, "expires_in": 3600, "userId": User})
		return
	}
	if r.Method != http.MethodGet {
		var v any
		_ = json.Unmarshal(body, &v)
		s.Writes = append(s.Writes, Write{Method: r.Method, Host: host, Path: path, Body: v})
	}
	out, status := s.route(r.Method, host, path, r.URL.Query(), body)
	if status != 0 {
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"error":"fixture %d"}`, status)
		return
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func sortedQuery(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, k+"="+v)
		}
	}
	return strings.Join(parts, "&")
}

// route answers one request. A zero status means 200 with out.
func (s *Server) route(method, host, path string, q url.Values, body []byte) (out any, status int) {
	seg := strings.Split(strings.Trim(path, "/"), "/")
	// The client API and the app API share the user routes.
	if len(seg) > 0 && (seg[0] == "v1" || seg[0] == "v2") {
		seg = seg[1:]
	}
	at := func(i int) string {
		if i < len(seg) {
			return seg[i]
		}
		return ""
	}
	switch {
	case at(0) == "users" && at(1) == "me" && len(seg) == 2:
		return map[string]any{"user": s.user(User, true)}, 0
	case at(0) == "users" && len(seg) == 2:
		if at(1) != User && at(1) != Partner {
			return nil, http.StatusNotFound
		}
		u := s.user(at(1), false)
		if s.WrongUser && at(1) == Partner {
			u["userId"] = "u9"
		}
		return map[string]any{"user": u}, 0
	case at(0) == "devices" && at(1) == Device && len(seg) == 2:
		return map[string]any{"result": s.device()}, 0
	case at(0) == "users" && at(2) == "current-device":
		return map[string]any{"id": Device, "side": "left"}, 0
	case at(0) == "users" && at(2) == "temperature" && len(seg) == 3:
		return s.temperature(method, at(1), body), 0
	case at(0) == "users" && at(2) == "away-mode":
		if method == http.MethodPut {
			var p struct {
				AwayPeriod map[string]string `json:"awayPeriod"`
			}
			_ = json.Unmarshal(body, &p)
			_, on := p.AwayPeriod["start"]
			s.away[at(1)] = on
			return map[string]any{}, 0
		}
		return map[string]any{"isAway": s.away[at(1)]}, 0
	case at(0) == "users" && at(2) == "alarms":
		return s.alarms(method, host, seg[2:], body)
	case at(0) == "users" && at(2) == "routines":
		return map[string]any{"state": map[string]any{"nextAlarm": map[string]any{"alarmId": "a1"}, "upcomingRoutineId": "r1"},
			"settings": map[string]any{"routines": []any{map[string]any{"id": "r1", "alarms": []any{map[string]any{"alarmId": "a1"}}}}}}, 0
	case at(0) == "users" && at(2) == "trends":
		return s.trends(q), 0
	case at(0) == "users" && at(2) == "audio" && at(3) == "tracks" && len(seg) == 4:
		return map[string]any{"tracks": []any{
			map[string]any{"id": "t1", "title": "Ocean", "type": "soundscape"},
			map[string]any{"id": "t2", "title": "Rain", "type": "soundscape"},
		}}, 0
	case at(0) == "release" && at(1) == "features":
		return map[string]any{"features": []any{map[string]any{"title": "Autopilot", "body": "Learns your sleep."}}}, 0
	case at(0) == "household" && at(3) == "summary":
		return map[string]any{"households": []any{map[string]any{"sets": []any{map[string]any{"devices": []any{map[string]any{"deviceId": Device}}}}}}}, 0
	case at(0) == "users" && at(2) == "temperature" && at(3) == "nap-mode" && at(4) == "status":
		return map[string]any{"active": true, "endsAt": "2026-10-05T14:00:00Z"}, 0
	case at(0) == "users" && at(2) == "temperature" && at(3) == "hot-flash-mode" && len(seg) == 4 && method == http.MethodGet:
		return map[string]any{"enabled": false, "level": 0}, 0
	}
	if method != http.MethodGet {
		return map[string]any{}, 0
	}
	return map[string]any{"fixture": host + path}, 0
}

func (s *Server) user(id string, me bool) map[string]any {
	names := map[string][2]string{User: {"Ada", "Lovelace"}, Partner: {"Grace", "Hopper"}}
	side := "left"
	if id == Partner {
		side = "right"
	}
	if s.Solo {
		side = "solo"
	}
	u := map[string]any{"userId": id, "firstName": names[id][0], "lastName": names[id][1],
		"email": strings.ToLower(names[id][0]) + "@example.invalid", "currentDevice": map[string]any{"id": Device, "side": side}}
	if me {
		u["devices"] = []any{Device}
	}
	return u
}

func (s *Server) device() map[string]any {
	right := Partner
	if s.Solo {
		right = User
	}
	return map[string]any{"deviceId": Device, "ownerId": User, "leftUserId": User, "rightUserId": right,
		"awaySides": map[string]any{}, "online": true, "lastHeard": "2026-10-05T07:00:00Z",
		"sensorInfo": map[string]any{"blanket": map[string]any{"isConnected": true, "connectors": []any{"left", "right"}}}}
}

func (s *Server) temperature(method, id string, body []byte) map[string]any {
	if method == http.MethodPut {
		var p struct {
			CurrentLevel *int `json:"currentLevel"`
			CurrentState *struct {
				Type string `json:"type"`
			} `json:"currentState"`
		}
		_ = json.Unmarshal(body, &p)
		if p.CurrentLevel != nil {
			s.level[id] = *p.CurrentLevel
		}
		if p.CurrentState != nil {
			s.state[id] = p.CurrentState.Type
		}
		return map[string]any{}
	}
	out := map[string]any{"currentLevel": s.level[id], "currentState": map[string]any{"type": s.state[id]}}
	if !s.NoSchedule {
		out["smart"] = map[string]any{"bedTimeLevel": -10, "initialSleepLevel": -20, "finalSleepLevel": 0}
	}
	return out
}

var fixtureAlarms = []any{
	map[string]any{"id": "a1", "time": "07:00", "enabled": true, "daysOfWeek": []any{1, 2, 3, 4, 5},
		"vibration": map[string]any{"enabled": true, "pattern": "rise", "powerLevel": 50}, "sound": "chime"},
	map[string]any{"id": "a2", "time": "09:30", "enabled": false, "daysOfWeek": []any{0, 6}, "vibration": false},
}

func (s *Server) alarms(method, host string, rest []string, body []byte) (any, int) {
	switch {
	case len(rest) == 1 && method == http.MethodGet:
		return map[string]any{"alarms": fixtureAlarms}, 0
	case len(rest) == 1 && method == http.MethodPost:
		var a map[string]any
		_ = json.Unmarshal(body, &a)
		if a == nil {
			a = map[string]any{}
		}
		a["id"] = "a3"
		return map[string]any{"alarm": a}, 0
	case len(rest) == 2 && rest[1] == "active" && method == http.MethodGet:
		return map[string]any{"alarms": []any{fixtureAlarms[0]}}, 0
	case len(rest) == 3 && rest[1] == "active" && rest[2] == "dismiss-all":
		if host == "app-api" && method != http.MethodPut {
			return nil, http.StatusMethodNotAllowed
		}
		return map[string]any{}, s.DismissAllStatus
	case len(rest) == 2 && method == http.MethodPatch:
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		p["id"] = rest[1]
		return map[string]any{"alarm": p}, 0
	}
	return map[string]any{}, 0
}

// Fixture days are the dates the tests name with --date, --from and --to.
// recordedDay is when the compat goldens were recorded: a day the CLI took
// from the wall clock (sleep day without --date) was that day then.
var (
	firstFixtureDay = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	recordedDay     = time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
)

// fixtureDay is the number a day's metrics vary by: its day of month from
// the first fixture day to the recording day, and the recording day's for
// any other day, which only the wall clock asks for. So today's metrics
// are the same whatever the run date.
func fixtureDay(d time.Time) int {
	if d.Before(firstFixtureDay) || d.After(recordedDay) {
		return recordedDay.Day()
	}
	return d.Day()
}

// trends answers every requested day with metrics and one session whose
// samples end a few minutes before now, or three hours before with
// StaleSignals.
func (s *Server) trends(q url.Values) map[string]any {
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = "2026-10-04"
	}
	if to == "" {
		to = from
	}
	start, err1 := time.Parse("2006-01-02", from)
	end, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil || end.Before(start) {
		return map[string]any{"days": []any{}}
	}
	last := s.now().UTC().Add(-5 * time.Minute).Truncate(time.Second)
	if s.StaleSignals {
		last = last.Add(-3 * time.Hour)
	}
	sample := func(d time.Duration, v float64) []any { return []any{last.Add(-d).Format(time.RFC3339), v} }
	days := []any{}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		n := float64(fixtureDay(d))
		days = append(days, map[string]any{
			"day": d.Format("2006-01-02"), "score": 80 + n, "tnt": 12, "respiratoryRate": 14.5, "heartRate": 58.25,
			"latencyAsleepSeconds": 600, "latencyOutSeconds": 300, "sleepDurationSeconds": 27000 + n,
			"presenceStart":     d.Format("2006-01-02") + "T06:00:00Z",
			"sleepQualityScore": map[string]any{"hrv": map[string]any{"score": 71}, "respiratoryRate": map[string]any{"score": 90}},
			"sessions": []any{map[string]any{"timeseries": map[string]any{
				"heartRate":       []any{sample(10*time.Minute, 57), sample(0, 58)},
				"hrv":             []any{sample(time.Minute, 40)},
				"respiratoryRate": []any{sample(2*time.Minute, 14)},
				"tempBedC":        []any{sample(0, 29.5)},
			}}},
		})
	}
	return map[string]any{"days": days}
}
