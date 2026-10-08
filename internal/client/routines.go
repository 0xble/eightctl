package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ActiveAlarms returns the alarms ringing now, as the app API reports them.
func (a *AlarmActions) ActiveAlarms(ctx context.Context) ([]map[string]any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	var res struct {
		Alarms []any `json:"alarms"`
	}
	path := fmt.Sprintf("/v1/users/%s/alarms/active", a.c.UserID)
	if err := a.c.doApp(ctx, http.MethodGet, path, nil, nil, &res); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, v := range res.Alarms {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// DismissAllRoute is the app API route that dismisses every active alarm.
func (a *AlarmActions) DismissAllRoute() string {
	return fmt.Sprintf("%s/users/%s/alarms/active/dismiss-all", a.c.appAPIBase(), a.c.UserID)
}

// DismissRoute is the app API route that dismisses one alarm.
func (a *AlarmActions) DismissRoute(alarmID string) string {
	return fmt.Sprintf("%s/users/%s/alarms/%s/dismiss", a.c.appAPIBase(), a.c.UserID, alarmID)
}

// DismissActive dismisses every active alarm with PUT on the app API, the
// route that advertises Allow: PUT. When that route answers 404 or 405 it
// dismisses each active alarm instead and returns their IDs; dismissed is
// nil when the bulk route worked.
func (a *AlarmActions) DismissActive(ctx context.Context) (dismissed []string, err error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	err = a.c.doURL(ctx, http.MethodPut, a.DismissAllRoute(), map[string]any{}, nil)
	var ae *APIError
	if err == nil || !errors.As(err, &ae) || (ae.Status != http.StatusNotFound && ae.Status != http.StatusMethodNotAllowed) {
		return nil, err
	}
	alarms, err := a.ActiveAlarms(ctx)
	if err != nil {
		return nil, err
	}
	dismissed = []string{}
	for _, alarm := range alarms {
		id := alarm["id"]
		if id == nil || id == "" {
			id = alarm["alarmId"]
		}
		if id == nil || id == "" {
			continue
		}
		sid := fmt.Sprint(id)
		if err := a.c.doURL(ctx, http.MethodPost, a.DismissRoute(sid), map[string]any{}, nil); err != nil {
			return dismissed, err
		}
		dismissed = append(dismissed, sid)
	}
	return dismissed, nil
}

// NextAlarmID reads the routines and returns the next alarm's ID: the
// state's nextAlarm, else the first alarm of the upcoming routine's override,
// else of the routine itself. Empty means none is scheduled.
func (a *AlarmActions) NextAlarmID(ctx context.Context) (string, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return "", err
	}
	type alarmRef struct {
		AlarmID any `json:"alarmId"`
	}
	var res struct {
		State struct {
			NextAlarm         *alarmRef `json:"nextAlarm"`
			UpcomingRoutineID any       `json:"upcomingRoutineId"`
		} `json:"state"`
		Settings struct {
			Routines []struct {
				ID       any `json:"id"`
				Override *struct {
					Alarms []alarmRef `json:"alarms"`
				} `json:"override"`
				Alarms []alarmRef `json:"alarms"`
			} `json:"routines"`
		} `json:"settings"`
	}
	path := fmt.Sprintf("/v2/users/%s/routines", a.c.UserID)
	if err := a.c.doApp(ctx, http.MethodGet, path, nil, nil, &res); err != nil {
		return "", err
	}
	id := func(v any) string {
		if v == nil || v == false || v == "" {
			return ""
		}
		return fmt.Sprint(v)
	}
	if res.State.NextAlarm != nil {
		if s := id(res.State.NextAlarm.AlarmID); s != "" {
			return s, nil
		}
	}
	upcoming := id(res.State.UpcomingRoutineID)
	if upcoming == "" {
		return "", nil
	}
	for _, r := range res.Settings.Routines {
		if id(r.ID) != upcoming {
			continue
		}
		if r.Override != nil && len(r.Override.Alarms) > 0 {
			if s := id(r.Override.Alarms[0].AlarmID); s != "" {
				return s, nil
			}
		}
		if len(r.Alarms) > 0 {
			return id(r.Alarms[0].AlarmID), nil
		}
		return "", nil
	}
	return "", nil
}
