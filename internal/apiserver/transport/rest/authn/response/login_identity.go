package response

import "time"

type LoginIdentityResponse struct {
	ID               string     `json:"id"`
	Provider         string     `json:"provider"`
	Realm            string     `json:"realm"`
	Identifier       string     `json:"identifier"`
	GlobalIdentifier string     `json:"global_identifier,omitempty"`
	Status           string     `json:"status"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	LinkedAt         time.Time  `json:"linked_at"`
}

type LoginIdentityListResponse struct {
	Items []LoginIdentityResponse `json:"items"`
}

type LinkLoginIdentityResponse struct {
	LoginIdentity LoginIdentityResponse `json:"login_identity"`
	Reused        bool                  `json:"reused"`
}
