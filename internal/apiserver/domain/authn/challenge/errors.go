package challenge

import "errors"

var (
	ErrRepositoryNotConfigured = errors.New("challenge repository is not configured")
	ErrChallengeSceneRequired  = errors.New("challenge scene is required")
	ErrPhoneE164Required       = errors.New("phone e164 is required")
	ErrOAuthSceneRequired      = errors.New("oauth scene is required")
	ErrAppIDRequired           = errors.New("app_id is required")
	ErrRedirectURIRequired     = errors.New("redirect_uri is required")
	ErrOAuthStateRequired      = errors.New("oauth state is required")
	ErrOAuthStateNotFound      = errors.New("oauth state not found")
	ErrOAuthStateInvalid       = errors.New("invalid oauth state challenge")
	ErrOAuthStateAlreadyUsed   = errors.New("oauth state already used")
	ErrOAuthStateExpired       = errors.New("oauth state expired")
	ErrOAuthStateMismatch      = errors.New("oauth state mismatch")
)
