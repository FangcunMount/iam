package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	appprofile "github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	responsedto "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/response"
)

func profileResultToResponse(result *appprofile.ProfileResult) responsedto.ProfileResponse {
	if result == nil {
		return responsedto.ProfileResponse{}
	}

	resp := responsedto.ProfileResponse{
		ID:        result.ID,
		LegalName: result.Name,
		DOB:       result.Birthday,
		IDMasked:  maskIDCard(result.IDCard),
	}

	resp.Gender = &result.Gender

	if result.Height > 0 {
		h := int(result.Height)
		resp.HeightCm = &h
	}

	if result.Weight > 0 {
		kg := float64(result.Weight) / 1000.0
		w := fmt.Sprintf("%.2f", kg)
		resp.WeightKg = &w
	}

	return resp
}

func parseHeightCm(heightCm *int) *uint32 {
	if heightCm == nil || *heightCm <= 0 {
		return nil
	}
	h := uint32(*heightCm)
	return &h
}

func parseWeightKg(weightKg string) *uint32 {
	if strings.TrimSpace(weightKg) == "" {
		return nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(weightKg), 64)
	if err != nil || f <= 0 {
		return nil
	}
	w := uint32(f * 1000)
	return &w
}

func maskIDCard(idCard string) string {
	if len(idCard) < 6 {
		return idCard
	}
	return idCard[:6] + "********" + idCard[len(idCard)-4:]
}

func sliceProfiles(items []responsedto.ProfileResponse, offset, limit int) []responsedto.ProfileResponse {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []responsedto.ProfileResponse{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseOptionalTime(timeStr string) *time.Time {
	if timeStr == "" {
		return nil
	}
	parsed := parseTime(timeStr)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}
