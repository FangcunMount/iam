package process

import "testing"

func TestPrepareRunnerDefinesStartupStagesInOrder(t *testing.T) {
	runner := newPrepareRunner(&apiServer{})

	got := make([]string, 0, len(runner.stages))
	for _, stage := range runner.stages {
		got = append(got, stage.Name())
	}

	want := []string{
		"prepare runtime",
		"prepare resources",
		"initialize container",
		"initialize transports",
		"start runtime tasks",
		"register shutdown callbacks",
	}
	if len(got) != len(want) {
		t.Fatalf("stage count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stage[%d] = %q, want %q (all stages: %v)", i, got[i], want[i], got)
		}
	}
}
