package challenge

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
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

func TestPhoneOTPConsumeRepositoryErrorFailsClosed(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	sms := &smsSenderStub{}
	service := NewService(repo, SMSOTPDelivery{
		Gate: &smsOTPGateStub{allow: true},
		SMS:  sms,
	}, NewCreator(repo), NewVerifier(repo))
	ctx := context.Background()
	require.NoError(t, service.SendLoginPhoneOTP(ctx, "13800138000"))
	repo.consumeErr = errors.New("redis unavailable")

	require.False(t, service.VerifyAndConsumeLoginPhoneOTP(ctx, "13800138000", sms.code))
	require.NotEmpty(t, repo.items)
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

func TestSendLoginPhoneOTPConsumesQuotaAndSendsInOrder(t *testing.T) {
	t.Parallel()

	var events []string
	repo := newChallengeRepoStub()
	repo.events = &events
	gate := &smsOTPGateStub{allow: true, events: &events}
	quota := &smsOTPQuotaStub{events: &events}
	sms := &smsSenderStub{events: &events}
	service := NewService(repo, SMSOTPDelivery{
		Gate:  gate,
		Quota: quota,
		SMS:   sms,
	}, NewCreator(repo), NewVerifier(repo))

	err := service.SendLoginPhoneOTP(context.Background(), "13800138000")

	require.NoError(t, err)
	require.Equal(t, []string{"gate", "quota:hourly", "quota:daily", "create", "sms"}, events)
	require.Empty(t, quota.rollbacks)
}

func TestSendLoginPhoneOTPRollsBackQuotaWhenChallengeCreationFails(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	repo.createErr = errors.New("redis down")
	quota := &smsOTPQuotaStub{}
	service := NewService(repo, SMSOTPDelivery{
		Gate:  &smsOTPGateStub{allow: true},
		Quota: quota,
		SMS:   &smsSenderStub{},
	}, NewCreator(repo), NewVerifier(repo))

	err := service.SendLoginPhoneOTP(context.Background(), "13800138000")

	require.Error(t, err)
	require.Equal(t, []string{"hourly", "daily"}, quota.rollbackDimensions())
}

func TestSendLoginPhoneOTPRollsBackQuotaAndDeletesChallengeWhenSMSFails(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	quota := &smsOTPQuotaStub{}
	sms := &smsSenderStub{err: errors.New("sms down")}
	service := NewService(repo, SMSOTPDelivery{
		Gate:  &smsOTPGateStub{allow: true},
		Quota: quota,
		SMS:   sms,
	}, NewCreator(repo), NewVerifier(repo))

	err := service.SendLoginPhoneOTP(context.Background(), "13800138000")

	require.Error(t, err)
	require.Equal(t, []string{"hourly", "daily"}, quota.rollbackDimensions())
	require.Equal(t, []string{smsOTPChallengeID(SceneLoginPhoneOTP, "+8613800138000")}, repo.deleted)
	require.Empty(t, repo.items)
}

func TestSendLoginPhoneOTPRollsBackHourlyWhenDailyQuotaExceeded(t *testing.T) {
	t.Parallel()

	quota := &smsOTPQuotaStub{denyDimensions: map[string]bool{"daily": true}}
	service := NewService(newChallengeRepoStub(), SMSOTPDelivery{
		Gate:  &smsOTPGateStub{allow: true},
		Quota: quota,
		SMS:   &smsSenderStub{},
	}, NewCreator(newChallengeRepoStub()), NewVerifier(newChallengeRepoStub()))

	err := service.SendLoginPhoneOTP(context.Background(), "13800138000")

	require.Error(t, err)
	require.Equal(t, []string{"hourly", "daily"}, quota.calls)
	require.Equal(t, []string{"hourly"}, quota.rollbackDimensions())
}

func TestSendLoginPhoneOTPDoesNotConsumeQuotaWhenHourlyQuotaExceeded(t *testing.T) {
	t.Parallel()

	quota := &smsOTPQuotaStub{denyDimensions: map[string]bool{"hourly": true}}
	service := NewService(newChallengeRepoStub(), SMSOTPDelivery{
		Gate:  &smsOTPGateStub{allow: true},
		Quota: quota,
		SMS:   &smsSenderStub{},
	}, NewCreator(newChallengeRepoStub()), NewVerifier(newChallengeRepoStub()))

	err := service.SendLoginPhoneOTP(context.Background(), "13800138000")

	require.Error(t, err)
	require.Equal(t, []string{"hourly"}, quota.calls)
	require.Empty(t, quota.rollbacks)
}

func TestSendLoginPhoneOTPDoesNotConsumeQuotaWhenGateRejects(t *testing.T) {
	t.Parallel()

	quota := &smsOTPQuotaStub{}
	service := NewService(newChallengeRepoStub(), SMSOTPDelivery{
		Gate:  &smsOTPGateStub{allow: false},
		Quota: quota,
		SMS:   &smsSenderStub{},
	}, NewCreator(newChallengeRepoStub()), NewVerifier(newChallengeRepoStub()))

	err := service.SendLoginPhoneOTP(context.Background(), "13800138000")

	require.Error(t, err)
	require.Empty(t, quota.calls)
}

type challengeRepoStub struct {
	items      map[string]*challengeDomain.AuthChallenge
	attempts   map[string]int
	createErr  error
	consumeErr error
	deleted    []string
	events     *[]string
}

func newChallengeRepoStub() *challengeRepoStub {
	return &challengeRepoStub{
		items:    map[string]*challengeDomain.AuthChallenge{},
		attempts: map[string]int{},
	}
}

func (s *challengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	if s.events != nil {
		*s.events = append(*s.events, "create")
	}
	if s.createErr != nil {
		return s.createErr
	}
	s.items[challenge.ID] = challenge
	return nil
}

func (s *challengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *challengeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, expectedHash []byte) (bool, error) {
	if s.consumeErr != nil {
		return false, s.consumeErr
	}
	item := s.items[id]
	if item == nil || !bytes.Equal(item.SecretHash, expectedHash) {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *challengeRepoStub) RecordFailedAttemptIfCurrent(
	_ context.Context,
	id string,
	currentSecretHash []byte,
	maxAttempts int,
) (bool, bool, error) {
	item := s.items[id]
	if item == nil || !bytes.Equal(item.SecretHash, currentSecretHash) {
		return false, false, nil
	}
	s.attempts[id]++
	if s.attempts[id] >= maxAttempts {
		delete(s.items, id)
		delete(s.attempts, id)
		return true, true, nil
	}
	return true, false, nil
}

func (s *challengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	s.deleted = append(s.deleted, id)
	return nil
}

type smsOTPGateStub struct {
	allow    bool
	phone    string
	scene    string
	cooldown time.Duration
	events   *[]string
}

func (s *smsOTPGateStub) TryAcquire(_ context.Context, phoneE164, scene string, cooldown time.Duration) (bool, error) {
	if s.events != nil {
		*s.events = append(*s.events, "gate")
	}
	s.phone = phoneE164
	s.scene = scene
	s.cooldown = cooldown
	return s.allow, nil
}

type smsSenderStub struct {
	phone  string
	code   string
	err    error
	events *[]string
}

func (s *smsSenderStub) SendLoginOTP(_ context.Context, phoneE164, code string) error {
	if s.events != nil {
		*s.events = append(*s.events, "sms")
	}
	s.phone = phoneE164
	s.code = code
	return s.err
}

type smsOTPQuotaStub struct {
	calls          []string
	rollbacks      []authentication.OTPSendQuotaLease
	denyDimensions map[string]bool
	events         *[]string
}

func (s *smsOTPQuotaStub) TryConsume(_ context.Context, phoneE164, scene, dimension string, limit int, window time.Duration) (authentication.OTPSendQuotaLease, bool, error) {
	if s.events != nil {
		*s.events = append(*s.events, "quota:"+dimension)
	}
	s.calls = append(s.calls, dimension)
	if s.denyDimensions[dimension] {
		return authentication.OTPSendQuotaLease{}, false, nil
	}
	return authentication.OTPSendQuotaLease{
		PhoneE164: phoneE164,
		Scene:     scene,
		Dimension: dimension,
		Member:    dimension + "-member",
		Window:    window,
	}, true, nil
}

func (s *smsOTPQuotaStub) Rollback(_ context.Context, lease authentication.OTPSendQuotaLease) error {
	s.rollbacks = append(s.rollbacks, lease)
	return nil
}

func (s *smsOTPQuotaStub) rollbackDimensions() []string {
	dimensions := make([]string, 0, len(s.rollbacks))
	for _, lease := range s.rollbacks {
		dimensions = append(dimensions, lease.Dimension)
	}
	return dimensions
}
