package loginidentity

// Status describes the lifecycle of a login identity binding.
type Status string

const (
	StatusDisabled Status = "disabled"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
	StatusDeleted  Status = "deleted"
)

func (s Status) Validate() bool {
	switch s {
	case StatusDisabled, StatusActive, StatusArchived, StatusDeleted:
		return true
	default:
		return false
	}
}

func (s Status) IsActive() bool { return s == StatusActive }
