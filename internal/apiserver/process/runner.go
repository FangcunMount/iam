package process

// Stage describes one named preparation step in the apiserver startup path.
type Stage[S any] interface {
	Name() string
	Run(*S) error
}

// Runner executes preparation stages in order and builds the prepared server
// only after every stage succeeds.
type Runner[S any, P any] struct {
	State         *S
	Stages        []Stage[S]
	BuildPrepared func(*S) P
}

// Run executes all stages, returning the first failed stage name with its
// original error so callers can preserve existing error semantics.
func (r Runner[S, P]) Run() (P, string, error) {
	var zero P

	state := r.State
	if state == nil {
		state = new(S)
	}

	for _, stage := range r.Stages {
		if stage == nil {
			continue
		}
		if err := stage.Run(state); err != nil {
			return zero, stage.Name(), err
		}
	}

	if r.BuildPrepared == nil {
		return zero, "", nil
	}
	return r.BuildPrepared(state), "", nil
}
