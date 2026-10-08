package ops

import (
	"context"
	"fmt"
	"io"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
)

// AwayInput selects whose away mode to change or read. on and off default
// to the authenticated user's side, status to every discovered side.
type AwayInput struct {
	SideInput
	Both bool `json:"both,omitempty" help:"Apply to every household member"`
}

// AwayResult is the result of away on or off. The cloud is eventually
// consistent: away status may show the previous state for a while.
type AwayResult struct {
	Applied bool `json:"applied"`
	Away    bool `json:"away"`
	// Scope is "your side", "side left", "user <id>" or "<n> household members".
	Scope   string   `json:"scope"`
	Targets []Target `json:"targets"`
}

// AwayRow is one side's away state.
type AwayRow struct {
	Side   string `json:"side"`
	Name   string `json:"name"`
	UserID string `json:"user_id"`
	Away   bool   `json:"away"`
}

func registerAway(reg *op.Registry, b *Backend) {
	for _, on := range []bool{true, false} {
		name, summary, done := "away.off", "Deactivate away mode", "deactivated"
		if on {
			name, summary, done = "away.on", "Activate away mode", "activated"
		}
		op.Add(reg, op.Op[AwayInput, AwayResult]{
			Name: name, Summary: summary, Effect: op.Write, MCP: true, CLIImmediate: true,
			Handler: func(ctx context.Context, req op.Request, in AwayInput) (AwayResult, error) {
				return b.setAway(ctx, req, in, on)
			},
			Render: func(w io.Writer, r AwayResult) error {
				if !r.Applied {
					_, err := fmt.Fprintf(w, "dry run: would set away mode %s (%s)\n", map[bool]string{true: "on", false: "off"}[r.Away], r.Scope)
					return err
				}
				if s, err := b.load(op.SurfaceCLI); err == nil && s.AwayQuiet {
					return nil
				}
				_, err := fmt.Fprintf(w, "away mode %s (%s)\n", done, r.Scope)
				return err
			},
		})
	}
	rows(reg, b, op.Op[AwayInput, []AwayRow]{
		Name: "away.status", Summary: "Show whether away mode is active",
		Handler: func(ctx context.Context, req op.Request, in AwayInput) ([]AwayRow, error) {
			if in.Both && (in.Side != "" || in.TargetUserID != "") {
				return nil, usage("--both conflicts with --side/--target-user-id")
			}
			cl, _, err := b.client(req)
			if err != nil {
				return nil, err
			}
			target, err := selectTarget(ctx, cl, in.SideInput)
			if err != nil {
				return nil, err
			}
			var targets []client.HouseholdUserTarget
			if target != nil {
				targets = []client.HouseholdUserTarget{*target}
			} else if targets, err = cl.HouseholdUserTargets(ctx); err != nil {
				return nil, providerErr(err)
			}
			if err := checkTargets(targets); err != nil {
				return nil, err
			}
			out := make([]AwayRow, 0, len(targets))
			for _, t := range targets {
				away, err := cl.GetAwayMode(ctx, t.UserID)
				if err != nil {
					return nil, providerErr(fmt.Errorf("reading away for %s: %w", t.UserID, err))
				}
				out = append(out, AwayRow{Side: t.SideLabel(), Name: t.DisplayName(), UserID: t.UserID, Away: away})
			}
			return out, nil
		},
	}, "side", "name", "user_id", "away")
}

func checkTargets(targets []client.HouseholdUserTarget) error {
	if len(targets) == 0 {
		return op.Errorf(op.KindNotFound, "no_household_users", "no household users found")
	}
	for _, t := range targets {
		if t.UserID == "" {
			return op.Errorf(op.KindError, "invalid_household_user", "household user is missing a user ID")
		}
	}
	return nil
}

func (b *Backend) setAway(ctx context.Context, req op.Request, in AwayInput, on bool) (AwayResult, error) {
	cl, _, err := b.client(req)
	if err != nil {
		return AwayResult{}, err
	}
	target, err := selectTarget(ctx, cl, in.SideInput)
	if err != nil {
		return AwayResult{}, err
	}
	out := AwayResult{Applied: req.Apply, Away: on}
	var targets []client.HouseholdUserTarget
	switch {
	case in.Both:
		if target != nil {
			return AwayResult{}, usage("--both conflicts with --side/--target-user-id")
		}
		// The unfiltered device response can omit every user while away.
		// Household lookup explicitly requests the IDs needed for the writes.
		if targets, err = cl.HouseholdUserTargets(ctx); err != nil {
			return AwayResult{}, providerErr(fmt.Errorf("fetching household users: %w", err))
		}
		if err := checkTargets(targets); err != nil {
			return AwayResult{}, err
		}
		out.Scope = fmt.Sprintf("%d household members", len(targets))
		if len(targets) == 1 {
			out.Scope = "1 household member"
		}
	case target != nil:
		targets = []client.HouseholdUserTarget{*target}
		if out.Scope = targetScope(target); out.Scope == "" {
			out.Scope = "selected target"
		}
	default:
		out.Scope = "your side"
	}
	out.Targets = targetRefs(targets)
	if !req.Apply {
		return out, nil
	}
	if len(targets) == 0 {
		return out, providerErr(cl.SetAwayMode(ctx, "", on))
	}
	for _, t := range targets {
		if err := cl.SetAwayMode(ctx, t.UserID, on); err != nil {
			return AwayResult{}, providerErr(fmt.Errorf("setting away for %s: %w", t.UserID, err))
		}
	}
	return out, nil
}
