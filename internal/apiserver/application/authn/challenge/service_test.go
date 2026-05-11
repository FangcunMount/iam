package challenge

import (
	"context"
	"testing"
	"time"

	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestSMSOTPChallengeCreateVerifyConsumeAndReplay(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	service := NewService(repo, SMSOTPDelivery{
		Gate:     &smsOTPGateStub{allow: true},
		SMS:      &smsSenderStub{},
		TTL:      time.Minute,
		Cooldown: 2 * time.Minute,
		CodeLen:  4,
	}, NewCreator(repo), NewVerifier(repo))
	ctx := context.Background()

	ok := service.VerifyAndConsume(ctx, SceneLoginPhoneOTP, "13800138000", "000000")
	require.False(t, ok)

	ok = service.VerifyAndConsume(ctx, SceneLoginPhoneOTP, "13800138000", "000000")
	require.False(t, ok)
}

func TestSendSMSOTPCreatesChallengeAndSendsCode(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	gate := &smsOTPGateStub{allow: true}
	sms := &smsSenderStub{code: "000000"}
	service := NewService(repo, SMSOTPDelivery{
		Gate:     gate,
		SMS:      sms,
		TTL:      time.Minute,
		Cooldown: 2 * time.Minute,
		CodeLen:  4,
	}, NewCreator(repo), NewVerifier(repo))
	ctx := context.Background()
	err := service.SendSMSOTP(ctx, SceneLoginPhoneOTP, "13800138000")
	require.NoError(t, err)
}

type challengeRepoStub struct {
	items map[string]*challengeDomain.AuthChallenge
}

func newChallengeRepoStub() *challengeRepoStub {
	return &challengeRepoStub{items: map[string]*challengeDomain.AuthChallenge{}}
}

func (s *challengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	s.items[challenge.ID] = challenge
	return nil
}

func (s *challengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *challengeRepoStub) Consume(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

func (s *challengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

type smsOTPGateStub struct {
	allow    bool
	phone    string
	scene    string
	cooldown time.Duration
}

func (s *smsOTPGateStub) TryAcquire(_ context.Context, phoneE164, scene string, cooldown time.Duration) (bool, error) {
	s.phone = phoneE164
	s.scene = scene
	s.cooldown = cooldown
	return s.allow, nil
}

type smsSenderStub struct {
	phone string
	code  string
}

func (s *smsSenderStub) SendLoginOTP(_ context.Context, phoneE164, code string) error {
	s.phone = phoneE164
	s.code = code
	return nil
}
