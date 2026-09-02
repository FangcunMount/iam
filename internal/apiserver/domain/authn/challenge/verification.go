package challenge

// VerificationOutcome 描述挑战校验结果。
type VerificationOutcome int

const (
	VerificationUnknown VerificationOutcome = iota
	VerificationSuccess
	VerificationRejected
	VerificationInvalidInput
	VerificationExhausted
	VerificationInfrastructureError
)

// VerificationResult 挑战校验结果。
type VerificationResult struct {
	Outcome VerificationOutcome
}
