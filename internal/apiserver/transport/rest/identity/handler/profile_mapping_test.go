package handler

import (
	"testing"
	"time"

	appprofile "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profile"
	responsedto "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/identity/response"
)

func TestProfileResultToResponse(t *testing.T) {
	result := &appprofile.ProfileResult{
		ID:       "profile-1",
		Name:     "Kid",
		IDCard:   "123456200001019999",
		Gender:   1,
		Birthday: "2020-01-01",
	}

	resp := profileResultToResponse(result)

	if resp.ID != "profile-1" ||
		resp.LegalName != "Kid" ||
		resp.IDMasked != "123456********9999" ||
		resp.DOB != "2020-01-01" ||
		resp.Gender == nil ||
		*resp.Gender != 1 {
		t.Fatalf("unexpected profile response: %#v", resp)
	}

	empty := profileResultToResponse(nil)
	if empty != (responsedto.ProfileResponse{}) {
		t.Fatalf("unexpected nil profile response: %#v", empty)
	}
}

func TestProfileMappingParsers(t *testing.T) {
	when := "2024-01-02T03:04:05Z"
	parsed := parseTime(when)
	if parsed.IsZero() || parsed.Format(time.RFC3339) != when {
		t.Fatalf("unexpected parsed time: %v", parsed)
	}

	if parseOptionalTime("bad") != nil {
		t.Fatal("expected invalid optional time to return nil")
	}
}

func TestSliceProfiles(t *testing.T) {
	items := []responsedto.ProfileResponse{
		{ID: "1"},
		{ID: "2"},
		{ID: "3"},
	}

	sliced := sliceProfiles(items, 1, 1)
	if len(sliced) != 1 || sliced[0].ID != "2" {
		t.Fatalf("unexpected slice: %#v", sliced)
	}

	if out := sliceProfiles(items, 10, 1); len(out) != 0 {
		t.Fatalf("expected empty out-of-range slice: %#v", out)
	}
}
