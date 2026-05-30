package jobs

// Event status values emitted on a job's stream. These are the fine-grained
// step statuses; Job.Status reports the coarse lifecycle state.
const (
	EvQueued    = "queued"
	EvStart     = "start"
	EvOK        = "ok"
	EvError     = "error"
	EvRunning   = "running"
	EvExpired   = "expired"
	EvCancelled = "cancelled"
	EvComplete  = "complete"
)
