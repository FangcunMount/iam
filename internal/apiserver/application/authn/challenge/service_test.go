package challenge

import (
	"context"
	"testing"
	"time"

	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestLoginPhoneOTPCreatesChallengeAndConsumesOnce(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	gate := &smsOTPGateStub{allow: true}
	sms := &smsSenderStub{}
	service := NewService(repo, SMSOTPDelivery{
		Gate:     gate,
		SMS:      sms,
		TTL:      time.Minute,
		Cooldown: 2 * time.Minute,
		CodeLen:  4,
	}, NewCreator(repo), NewVerifier(repo))
	ctx := context.Background()

	err := service.SendLoginPhoneOTP(ctx, "13800138000")
	require.NoError(t, err)
	require.Equal(t, SceneLoginPhoneOTP, gate.scene)
	require.Equal(t, "+8613800138000", sms.phone)
	require.NotEmpty(t, sms.code)

	ok := service.VerifyAndConsumeLoginPhoneOTP(ctx, "13800138000", sms.code)
	require.True(t, ok)

	ok = service.VerifyAndConsumeLoginPhoneOTP(ctx, "13800138000", sms.code)
	require.False(t, ok)
}

func TestPhoneLinkOTPCreatesChallengeAndConsumesOnce(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	gate := &smsOTPGateStub{allow: true}
	sms := &smsSenderStub{}
	service := NewService(repo, SMSOTPDelivery{
		Gate:     gate,
		SMS:      sms,
		TTL:      time.Minute,
		Cooldown: 2 * time.Minute,
		CodeLen:  4,
	}, NewCreator(repo), NewVerifier(repo))
	ctx := context.Background()

	err := service.SendPhoneLinkOTP(ctx, "13800138000")
	require.NoError(t, err)
	require.Equal(t, SceneLinkPhoneOTP, gate.scene)
	require.Equal(t, "+8613800138000", sms.phone)
	require.NotEmpty(t, sms.code)

	ok := service.VerifyAndConsumePhoneLinkOTP(ctx, "13800138000", sms.code)
	require.True(t, ok)

	ok = service.VerifyAndConsumePhoneLinkOTP(ctx, "13800138000", sms.code)
	require.False(t, ok)
}

func TestPhoneOTPVerifiersDoNotConsumeOtherScenes(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	sms := &smsSenderStub{}
	service := NewService(repo, SMSOTPDelivery{
		Gate:     &smsOTPGateStub{allow: true},
		SMS:      sms,
		TTL:      time.Minute,
		Cooldown: 2 * time.Minute,
		CodeLen:  4,
	}, NewCreator(repo), NewVerifier(repo))
	ctx := context.Background()

	require.NoError(t, service.SendLoginPhoneOTP(ctx, "13800138000"))
	loginCode := sms.code
	require.False(t, service.VerifyAndConsumePhoneLinkOTP(ctx, "13800138000", loginCode))
	require.True(t, service.VerifyAndConsumeLoginPhoneOTP(ctx, "13800138000", loginCode))

	require.NoError(t, service.SendPhoneLinkOTP(ctx, "13800138000"))
	linkCode := sms.code
	require.False(t, service.VerifyAndConsumeLoginPhoneOTP(ctx, "13800138000", linkCode))
	require.True(t, service.VerifyAndConsumePhoneLinkOTP(ctx, "13800138000", linkCode))
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
