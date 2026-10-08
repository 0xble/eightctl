package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
	"github.com/0xble/eightsleep/internal/daemon"
)

// SideInput selects one household member. Without either field a command acts as
// it documents: on, off and temp act on every discovered member.
type SideInput struct {
	Side         string `json:"side,omitempty" help:"target household side: left|right|solo"`
	TargetUserID string `json:"target_user_id,omitempty" name:"target-user-id" help:"set or query a specific household user ID"`
}

// Target is a household member a command acted on.
type Target struct {
	Side   string `json:"side,omitempty"`
	Name   string `json:"name,omitempty"`
	UserID string `json:"user_id,omitempty"`
}

// PowerResult is the result of on or off.
type PowerResult struct {
	Applied bool   `json:"applied"`
	State   string `json:"state"`
	// Targets are the household members written. Empty means the
	// authenticated user, when discovery found nobody.
	Targets []Target `json:"targets"`

	suffix string
}

// TempInput sets the temperature of one or every side.
type TempInput struct {
	SideInput
	Value string `json:"value" arg:"" help:"Temperature: 68F, 20C, or a heating level -100..100"`
}

// TempResult is the result of temp.
type TempResult struct {
	Applied bool     `json:"applied"`
	Level   int      `json:"level"`
	Targets []Target `json:"targets"`

	suffix string
}

// StatusInput selects the sides status reads.
type StatusInput struct {
	SideInput
	AllSides bool `json:"all_sides,omitempty" name:"all-sides" help:"show status for all discovered household sides"`
}

// StatusRow is one side's heating state. Side, name and user ID are absent
// when the account has no household.
type StatusRow struct {
	Side   string `json:"side,omitempty"`
	Name   string `json:"name,omitempty"`
	UserID string `json:"user_id,omitempty"`
	Mode   string `json:"mode"`
	Level  int    `json:"level"`
}

func registerPower(reg *op.Registry, b *Backend) {
	for _, on := range []bool{true, false} {
		state, verb := "off", "Turn pod off"
		if on {
			state, verb = "on", "Turn pod on"
		}
		op.Add(reg, op.Op[SideInput, PowerResult]{
			Name: state, Summary: verb, Effect: op.Write, MCP: true, CLIImmediate: true,
			Handler: func(ctx context.Context, req op.Request, in SideInput) (PowerResult, error) {
				cl, _, err := b.client(req)
				if err != nil {
					return PowerResult{}, err
				}
				targets, targeted, err := resolveTargets(ctx, cl, in)
				if err != nil {
					return PowerResult{}, err
				}
				out := PowerResult{State: state, Targets: targetRefs(targets), Applied: req.Apply}
				if targeted {
					out.suffix = targetListSuffix(targets)
				}
				if !req.Apply {
					return out, nil
				}
				set := cl.TurnOffForUser
				if on {
					set = cl.TurnOnForUser
				}
				if !targeted {
					return out, providerErr(set(ctx, ""))
				}
				for _, t := range targets {
					if err := set(ctx, t.UserID); err != nil {
						return PowerResult{}, providerErr(err)
					}
				}
				return out, nil
			},
			Render: func(w io.Writer, r PowerResult) error {
				if !r.Applied {
					_, err := fmt.Fprintf(w, "dry run: would turn pod %s%s\n", r.State, r.suffix)
					return err
				}
				_, err := fmt.Fprintf(w, "pod turned %s%s\n", r.State, r.suffix)
				return err
			},
		})
	}
	op.Add(reg, op.Op[TempInput, TempResult]{
		Name: "temp", Summary: "Set pod temperature (e.g., 68F, 20C, or heating level -100..100)", Effect: op.Write, MCP: true,
		CLIImmediate: true,
		Handler: func(ctx context.Context, req op.Request, in TempInput) (TempResult, error) {
			lvl, err := daemon.ParseTemp(in.Value)
			if err != nil {
				return TempResult{}, usage("%v", err)
			}
			cl, _, err := b.client(req)
			if err != nil {
				return TempResult{}, err
			}
			targets, targeted, err := resolveTargets(ctx, cl, in.SideInput)
			if err != nil {
				return TempResult{}, err
			}
			out := TempResult{Applied: req.Apply, Level: lvl, Targets: targetRefs(targets)}
			if targeted {
				out.suffix = targetListSuffix(targets)
			}
			if !req.Apply {
				return out, nil
			}
			if !targeted {
				return out, providerErr(cl.SetTemperatureForUser(ctx, "", lvl))
			}
			for _, t := range targets {
				if err := cl.SetTemperatureForUser(ctx, t.UserID, lvl); err != nil {
					return TempResult{}, providerErr(err)
				}
			}
			return out, nil
		},
		Render: func(w io.Writer, r TempResult) error {
			if !r.Applied {
				_, err := fmt.Fprintf(w, "dry run: would set temperature (level %d)%s\n", r.Level, r.suffix)
				return err
			}
			_, err := fmt.Fprintf(w, "temperature set (level %d)%s\n", r.Level, r.suffix)
			return err
		},
	})
	op.Add(reg, op.Op[StatusInput, []StatusRow]{
		Name: "status", Summary: "Show device status", Effect: op.Read, MCP: true,
		Handler: func(ctx context.Context, req op.Request, in StatusInput) ([]StatusRow, error) {
			cl, _, err := b.client(req)
			if err != nil {
				return nil, err
			}
			target, err := selectTarget(ctx, cl, in.SideInput)
			if err != nil {
				return nil, err
			}
			if in.AllSides && target != nil {
				return nil, usage("use --all-sides by itself, not with --side or --target-user-id")
			}
			switch {
			case in.AllSides:
				targets, err := cl.HouseholdUserTargets(ctx)
				if err != nil {
					return nil, providerErr(err)
				}
				return statusRows(ctx, cl, targets)
			case target != nil:
				return statusRows(ctx, cl, []client.HouseholdUserTarget{*target})
			}
			targets, err := cl.HouseholdUserTargets(ctx)
			if errors.Is(err, client.ErrInvalidHouseholdUser) {
				return nil, providerErr(err)
			}
			if err == nil && len(targets) > 0 {
				return statusRows(ctx, cl, targets)
			}
			st, err := cl.GetStatusForUser(ctx, "")
			if err != nil {
				return nil, providerErr(err)
			}
			return []StatusRow{{Mode: st.CurrentState.Type, Level: st.CurrentLevel}}, nil
		},
		RenderWithInput: func(w io.Writer, _ StatusInput, rs []StatusRow) error {
			headers := []string{"side", "name", "user_id", "mode", "level"}
			if len(rs) == 1 && rs[0].UserID == "" {
				headers = []string{"mode", "level"}
			}
			return b.printRows(w, headers, rowMaps(rs))
		},
	})
}

func statusRows(ctx context.Context, cl *client.Client, targets []client.HouseholdUserTarget) ([]StatusRow, error) {
	out := make([]StatusRow, 0, len(targets))
	for _, t := range targets {
		st, err := cl.GetStatusForUser(ctx, t.UserID)
		if err != nil {
			return nil, providerErr(err)
		}
		out = append(out, StatusRow{Side: t.SideLabel(), Name: t.DisplayName(), UserID: t.UserID,
			Mode: st.CurrentState.Type, Level: st.CurrentLevel})
	}
	return out, nil
}

// selectTarget resolves --side or --target-user-id to one household member,
// or nil when neither is given. An unknown --target-user-id is used as given.
func selectTarget(ctx context.Context, cl *client.Client, in SideInput) (*client.HouseholdUserTarget, error) {
	if in.TargetUserID != "" && in.Side != "" {
		return nil, usage("use either --target-user-id or --side, not both")
	}
	if in.Side != "" {
		targets, err := cl.HouseholdUserTargets(ctx)
		if err != nil {
			return nil, providerErr(err)
		}
		t, err := client.ResolveHouseholdSide(targets, in.Side)
		if err != nil {
			return nil, usage("%v", err)
		}
		return t, nil
	}
	if in.TargetUserID == "" {
		return nil, nil
	}
	targets, err := cl.HouseholdUserTargets(ctx)
	if err == nil {
		for _, t := range targets {
			if t.UserID == in.TargetUserID {
				return &t, nil
			}
		}
	}
	return &client.HouseholdUserTarget{UserID: in.TargetUserID}, nil
}

// resolveTargets is selectTarget for on, off and temp: without a selection
// it returns every discovered member, and targeted is false only when
// discovery fails or finds nobody, so the authenticated user is written.
func resolveTargets(ctx context.Context, cl *client.Client, in SideInput) (targets []client.HouseholdUserTarget, targeted bool, err error) {
	if in.TargetUserID != "" || in.Side != "" {
		t, err := selectTarget(ctx, cl, in)
		if err != nil || t == nil {
			return nil, false, err
		}
		return []client.HouseholdUserTarget{*t}, true, nil
	}
	targets, err = cl.HouseholdUserTargets(ctx)
	if errors.Is(err, client.ErrInvalidHouseholdUser) {
		return nil, false, providerErr(err)
	}
	if err != nil || len(targets) == 0 {
		return nil, false, nil
	}
	return targets, true, nil
}

func targetRefs(targets []client.HouseholdUserTarget) []Target {
	out := make([]Target, 0, len(targets))
	for _, t := range targets {
		out = append(out, Target{Side: strings.TrimSpace(t.Side), Name: t.DisplayName(), UserID: t.UserID})
	}
	return out
}

// targetScope labels a resolved target: "side left", "user abc", or "".
func targetScope(t *client.HouseholdUserTarget) string {
	if t == nil {
		return ""
	}
	if side := strings.TrimSpace(t.Side); side != "" {
		return "side " + side
	}
	if t.UserID != "" {
		return "user " + t.UserID
	}
	return ""
}

func targetListSuffix(targets []client.HouseholdUserTarget) string {
	switch len(targets) {
	case 0:
		return ""
	case 1:
		if scope := targetScope(&targets[0]); scope != "" {
			return " for " + scope
		}
		return ""
	}
	sides := []string{}
	for _, t := range targets {
		side := strings.TrimSpace(t.Side)
		if side == "" {
			return " for all discovered users"
		}
		sides = append(sides, side)
	}
	return " for sides " + strings.Join(sides, ", ")
}
