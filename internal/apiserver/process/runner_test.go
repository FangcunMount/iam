package process

import (
	"errors"
	"reflect"
	"testing"
)

type runnerTestState struct {
	visited []string
}

type runnerTestStage struct {
	name string
	run  func(*runnerTestState) error
}

func (s runnerTestStage) Name() string { return s.name }

func (s runnerTestStage) Run(state *runnerTestState) error {
	if s.run != nil {
		return s.run(state)
	}
	state.visited = append(state.visited, s.name)
	return nil
}

func TestRunnerRunsStagesInOrderAndBuildsPreparedOutput(t *testing.T) {
	state := &runnerTestState{}

	prepared, failedStage, err := Runner[runnerTestState, []string]{
		State: state,
		Stages: []Stage[runnerTestState]{
			runnerTestStage{name: "prepare runtime"},
			runnerTestStage{name: "prepare resources"},
			nil,
			runnerTestStage{name: "initialize container"},
		},
		BuildPrepared: func(state *runnerTestState) []string {
			return append([]string(nil), state.visited...)
		},
	}.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if failedStage != "" {
		t.Fatalf("failedStage = %q, want empty", failedStage)
	}

	want := []string{"prepare runtime", "prepare resources", "initialize container"}
	if !reflect.DeepEqual(prepared, want) {
		t.Fatalf("prepared = %#v, want %#v", prepared, want)
	}
}

func TestRunnerStopsAtFirstFailedStage(t *testing.T) {
	wantErr := errors.New("resource unavailable")
	state := &runnerTestState{}

	prepared, failedStage, err := Runner[runnerTestState, []string]{
		State: state,
		Stages: []Stage[runnerTestState]{
			runnerTestStage{name: "prepare runtime"},
			runnerTestStage{name: "prepare resources", run: func(*runnerTestState) error {
				return wantErr
			}},
			runnerTestStage{name: "initialize container"},
		},
		BuildPrepared: func(state *runnerTestState) []string {
			return append([]string(nil), state.visited...)
		},
	}.Run()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
	if failedStage != "prepare resources" {
		t.Fatalf("failedStage = %q, want prepare resources", failedStage)
	}
	if prepared != nil {
		t.Fatalf("prepared = %#v, want nil", prepared)
	}
	if want := []string{"prepare runtime"}; !reflect.DeepEqual(state.visited, want) {
		t.Fatalf("visited = %#v, want %#v", state.visited, want)
	}
}
