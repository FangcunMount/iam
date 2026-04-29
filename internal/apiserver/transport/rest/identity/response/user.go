package response

import "time"

// VerifiedContactResponse 用户联系方式展示
type VerifiedContactResponse struct {
	Type       string     `json:"type"`
	Value      string     `json:"value"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
}

// UserResponse 用户响应
type UserResponse struct {
	ID        string                    `json:"id"`
	Status    string                    `json:"status"`
	Nickname  string                    `json:"nickname,omitempty"`
	Avatar    string                    `json:"avatar,omitempty"`
	Contacts  []VerifiedContactResponse `json:"contacts,omitempty"`
	Roles     []string                  `json:"roles,omitempty"`
	CreatedAt *time.Time                `json:"createdAt,omitempty"`
	UpdatedAt *time.Time                `json:"updatedAt,omitempty"`
}
