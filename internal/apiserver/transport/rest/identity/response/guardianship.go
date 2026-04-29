package response

import "time"

// GuardianshipResponse 监护关系响应
type GuardianshipResponse struct {
	ID        uint64     `json:"id"`
	UserID    string     `json:"userId"`
	ChildID   string     `json:"childId"`
	Relation  string     `json:"relation"`
	Since     time.Time  `json:"since"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

// GuardianshipPageResponse 监护关系分页响应
type GuardianshipPageResponse struct {
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Items  []GuardianshipResponse `json:"items"`
}
