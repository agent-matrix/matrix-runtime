package workflow

import "fmt"

var transitions = map[string]map[string]bool{
	"queued":        {"running": true, "cancelled": true},
	"running":       {"waiting_human": true, "paused": true, "verifying": true, "failed": true, "cancelled": true, "rolling_back": true},
	"waiting_human": {"running": true, "cancelled": true},
	"paused":        {"running": true, "cancelled": true, "rolling_back": true},
	"verifying":     {"completed": true, "running": true, "failed": true, "rolling_back": true},
	"rolling_back":  {"rolled_back": true, "failed": true},
	"completed":     {},
	"failed":        {"rolling_back": true},
	"cancelled":     {},
	"rolled_back":   {},
}

func ValidateTransition(current, target string) error {
	next, ok := transitions[current]
	if !ok {
		return fmt.Errorf("unknown workflow status %q", current)
	}
	if !next[target] {
		return fmt.Errorf("invalid workflow transition %s -> %s", current, target)
	}
	return nil
}

type EventView struct {
	Seq        int
	FromStatus string
	ToStatus   string
}

func Replay(initial string, events []EventView) (string, error) {
	state := initial
	for _, ev := range events {
		if ev.FromStatus != state {
			return "", fmt.Errorf("event %d expected from %s, replay state is %s", ev.Seq, ev.FromStatus, state)
		}
		if err := ValidateTransition(state, ev.ToStatus); err != nil {
			return "", err
		}
		state = ev.ToStatus
	}
	return state, nil
}
