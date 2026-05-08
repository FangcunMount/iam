package response

import "time"

// ProfileResponse 档案响应
type ProfileResponse struct {
	ID        string     `json:"id"`
	LegalName string     `json:"legalName"`
	Gender    *uint8     `json:"gender,omitempty"`
	DOB       string     `json:"dob,omitempty"`
	IDType    string     `json:"idType,omitempty"`
	IDMasked  string     `json:"idMasked,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

// ProfilePageResponse 档案分页响应
type ProfilePageResponse struct {
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
	Items  []ProfileResponse `json:"items"`
}
