package queryprofile

const (
	defaultMaxResults         = 20
	defaultCandidateBudgetMult = 10
	defaultCandidateBudget    = defaultMaxResults * defaultCandidateBudgetMult
)

// Config 控制查询用例行为。
type Config struct {
	MaxResults         int
	CandidateBudget    int
	DisableMobileMask  bool
}

// WithDefaults 填充默认值。
func (c Config) WithDefaults() Config {
	out := c
	if out.MaxResults <= 0 {
		out.MaxResults = defaultMaxResults
	}
	if out.CandidateBudget <= 0 {
		out.CandidateBudget = out.MaxResults * defaultCandidateBudgetMult
	}
	if out.CandidateBudget < out.MaxResults {
		out.CandidateBudget = out.MaxResults
	}
	return out
}
