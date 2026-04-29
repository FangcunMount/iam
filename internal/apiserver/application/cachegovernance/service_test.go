package cachegovernance

import (
	"context"
	"testing"
)

func TestReadServiceOverview(t *testing.T) {
	inspectors := make([]FamilyInspector, 0, len(Families()))
	for _, descriptor := range Families() {
		inspectors = append(inspectors, governanceInspectorStub{
			descriptor: descriptor,
			status: FamilyStatus{
				Family:          descriptor.Family,
				Configured:      true,
				Healthy:         true,
				EntryCountKnown: descriptor.Backend == BackendKindMemory,
			},
		})
	}

	overview, err := NewReadService(inspectors).Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}

	if len(overview.Families) != 10 {
		t.Fatalf("family count = %d, want 10", len(overview.Families))
	}
	if len(overview.RuntimeStatuses) != 2 {
		t.Fatalf("runtime status count = %d, want 2", len(overview.RuntimeStatuses))
	}

	for _, view := range overview.Families {
		if !view.Status.Configured {
			t.Fatalf("family %s configured = false, want true", view.Descriptor.Family)
		}
		if !view.Status.Healthy {
			t.Fatalf("family %s healthy = false, want true; notes=%v", view.Descriptor.Family, view.Status.Notes)
		}
	}

	redisRuntime := findRuntimeStatus(t, overview.RuntimeStatuses, BackendKindRedis)
	if !redisRuntime.Configured || !redisRuntime.Healthy {
		t.Fatalf("redis runtime = %#v, want configured and healthy", redisRuntime)
	}
	memoryRuntime := findRuntimeStatus(t, overview.RuntimeStatuses, BackendKindMemory)
	if !memoryRuntime.Configured || !memoryRuntime.Healthy {
		t.Fatalf("memory runtime = %#v, want configured and healthy", memoryRuntime)
	}
}

func TestReadServiceDegradesWithoutInspectors(t *testing.T) {
	service := NewReadService(nil)

	overview, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if len(overview.Families) != 10 {
		t.Fatalf("family count = %d, want 10", len(overview.Families))
	}

	for _, view := range overview.Families {
		if view.Status.Configured {
			t.Fatalf("family %s configured = true, want false when inspector missing", view.Descriptor.Family)
		}
		if view.Status.Healthy {
			t.Fatalf("family %s healthy = true, want false when inspector missing", view.Descriptor.Family)
		}
	}

	redisRuntime := findRuntimeStatus(t, overview.RuntimeStatuses, BackendKindRedis)
	if redisRuntime.Configured || redisRuntime.Healthy {
		t.Fatalf("redis runtime = %#v, want unconfigured and unhealthy", redisRuntime)
	}
}

func findRuntimeStatus(t *testing.T, statuses []RuntimeStatus, backend BackendKind) RuntimeStatus {
	t.Helper()
	for _, status := range statuses {
		if status.Backend == backend {
			return status
		}
	}
	t.Fatalf("runtime status for backend %s not found", backend)
	return RuntimeStatus{}
}

type governanceInspectorStub struct {
	descriptor FamilyDescriptor
	status     FamilyStatus
	err        error
}

func (s governanceInspectorStub) Descriptor() FamilyDescriptor {
	return s.descriptor
}

func (s governanceInspectorStub) Status(context.Context) (FamilyStatus, error) {
	return s.status, s.err
}
