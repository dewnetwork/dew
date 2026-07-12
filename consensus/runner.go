package consensus

import "time"

const defaultRoundTimeout = 3 * time.Second

// Runner drives consensus rounds: starts the first round, restarts after each
// commit, and optionally times out stuck propose steps for liveness.
type Runner struct {
	Engine       *Engine
	OnCommit     func(CommitEvent) error
	RoundTimeout time.Duration // default 3s if zero
	stop         chan struct{}
}

// Start wires the engine commit hook, begins the timeout loop, and enters the
// first round for the current height.
func (r *Runner) Start() error {
	if r.stop == nil {
		r.stop = make(chan struct{})
	}
	r.Engine.OnCommit = func(ev CommitEvent) {
		if r.OnCommit != nil {
			_ = r.OnCommit(ev)
		}
		_ = r.Engine.StartRound()
	}
	go r.timeoutLoop()
	return r.Engine.StartRound()
}

// Stop ends the round-timeout goroutine.
func (r *Runner) Stop() {
	if r.stop == nil {
		return
	}
	select {
	case <-r.stop:
		return
	default:
		close(r.stop)
	}
}

func (r *Runner) timeoutLoop() {
	timeout := r.RoundTimeout
	if timeout <= 0 {
		timeout = defaultRoundTimeout
	}
	tick := time.NewTicker(timeout)
	defer tick.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-tick.C:
			switch r.Engine.Step() {
			case StepNewRound:
				_ = r.Engine.StartRound()
			case StepPropose, StepPrevote, StepPrecommit:
				_ = r.Engine.ForceTimeoutRound()
				_ = r.Engine.StartRound()
			}
		}
	}
}