package client

import (
	"context"
	"fmt"
	"net/http"
)

type AudioActions struct{ c *Client }

func (c *Client) Audio() *AudioActions { return &AudioActions{c: c} }

func (a *AudioActions) Tracks(ctx context.Context) ([]AudioTrack, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/audio/tracks", a.c.UserID)
	var res struct {
		Tracks []AudioTrack `json:"tracks"`
	}
	if err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res); err != nil {
		return nil, err
	}
	return res.Tracks, nil
}

func (a *AudioActions) Categories(ctx context.Context) (any, error) {
	path := "/audio/categories"
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AudioActions) PlayerState(ctx context.Context) (any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/audio/player", a.c.UserID)
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AudioActions) Play(ctx context.Context, trackID string) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/audio/player", a.c.UserID)
	body := map[string]any{"action": "play"}
	if trackID != "" {
		body["trackId"] = trackID
	}
	return a.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (a *AudioActions) Pause(ctx context.Context) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/audio/player", a.c.UserID)
	body := map[string]any{"action": "pause"}
	return a.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (a *AudioActions) Seek(ctx context.Context, positionMs int) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/audio/player/seek", a.c.UserID)
	body := map[string]any{"position": positionMs}
	return a.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (a *AudioActions) Volume(ctx context.Context, level int) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/audio/player/volume", a.c.UserID)
	body := map[string]any{"level": level}
	return a.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (a *AudioActions) Pair(ctx context.Context) error {
	deviceID, err := a.c.EnsureDeviceID(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/devices/%s/audio/player/pair", deviceID)
	return a.c.do(ctx, http.MethodPost, path, nil, map[string]any{}, nil)
}

func (a *AudioActions) RecommendedNext(ctx context.Context) (any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/audio/tracks/recommended-next-track", a.c.UserID)
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AudioActions) Favorites(ctx context.Context) (any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/audio/tracks/favorites", a.c.UserID)
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AudioActions) AddFavorite(ctx context.Context, trackID string) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/audio/tracks/favorites", a.c.UserID)
	body := map[string]any{"trackId": trackID}
	return a.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (a *AudioActions) RemoveFavorite(ctx context.Context, trackID string) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/audio/tracks/favorites/%s", a.c.UserID, trackID)
	return a.c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}
