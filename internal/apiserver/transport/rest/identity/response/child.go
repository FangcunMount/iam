package response

import "time"

// ChildResponse 儿童档案响应
type ChildResponse struct {
	ID        string     `json:"id"`
	LegalName string     `json:"legalName"`
	Gender    *uint8     `json:"gender,omitempty"`
	DOB       string     `json:"dob,omitempty"`
	IDType    string     `json:"idType,omitempty"`
	IDMasked  string     `json:"idMasked,omitempty"`
	HeightCm  *int       `json:"heightCm,omitempty"`
	WeightKg  *string    `json:"weightKg,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

// ChildPageResponse 儿童档案分页响应
type ChildPageResponse struct {
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
	Items  []ChildResponse `json:"items"`
}

// ChildRegisterResponse 儿童注册响应
type ChildRegisterResponse struct {
	Child        ChildResponse        `json:"child"`
	Guardianship GuardianshipResponse `json:"guardianship"`
}
