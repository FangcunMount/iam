package response

import "time"

// ProfileLinkResponse 档案关系响应
type ProfileLinkResponse struct {
	ID        uint64     `json:"id"`
	UserID    string     `json:"userId"`
	ProfileID string     `json:"profileId"`
	Relation  string     `json:"relation"`
	Since     time.Time  `json:"since"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

// ProfileLinkPageResponse 档案关系分页响应
type ProfileLinkPageResponse struct {
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
	Items  []ProfileLinkResponse `json:"items"`
}
