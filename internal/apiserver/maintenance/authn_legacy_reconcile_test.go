package maintenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAuthNLegacyPlanTreatsCanonicalMutableFactsAsAuthoritative(t *testing.T) {
	account := legacyAuthNAccount{
		ID:         10,
		UserID:     100,
		Type:       "opera",
		ExternalID: "operator",
		Status:     1,
	}
	credential := legacyAuthNCredential{
		ID:        20,
		AccountID: 10,
		Type:      "password",
		Material:  []byte("legacy-password-hash"),
		Algo:      "bcrypt",
		Status:    1,
	}
	key := authNProviderKey{Provider: "username", Realm: "default", Identifier: "operator"}
	state := authNCanonicalState{
		IdentitiesByKey: authNIdentityMap{
			key: {ID: 110, UserID: 100, Key: key},
		},
		IdentityIDs:     map[uint64]struct{}{110: {}},
		PasswordByLogin: map[uint64]uint64{110: 220},
		CredentialIDs:   map[uint64]struct{}{220: {}},
	}

	plan := buildAuthNReconcilePlan(
		[]legacyAuthNAccount{account},
		[]legacyAuthNCredential{credential},
		state,
		newAuthNLegacySummary(false),
	)

	if !plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want retirement eligible", plan.Summary)
	}
	if plan.Summary.AccountPresent != 1 || plan.Summary.PasswordPresent != 1 {
		t.Fatalf("summary = %+v, want existing canonical facts", plan.Summary)
	}
	if len(plan.Identities) != 0 || len(plan.Passwords) != 0 {
		t.Fatalf("plan unexpectedly overwrites canonical facts: %+v", plan)
	}
}

func TestAuthNLegacyPlanInsertsOnlyMissingCanonicalFacts(t *testing.T) {
	account := legacyAuthNAccount{
		ID:         10,
		UserID:     100,
		Type:       "opera",
		ExternalID: "operator",
		Status:     1,
	}
	credentials := []legacyAuthNCredential{
		{
			ID:        20,
			AccountID: 10,
			Type:      "password",
			Material:  []byte("legacy-password-hash"),
			Algo:      "argon2id",
			Status:    1,
		},
		{
			ID:            21,
			AccountID:     10,
			Type:          "phone_otp",
			IDPIdentifier: "+8613800000000",
			Status:        1,
		},
	}
	state := emptyCanonicalAuthNState()

	plan := buildAuthNReconcilePlan(
		[]legacyAuthNAccount{account},
		credentials,
		state,
		newAuthNLegacySummary(false),
	)

	if plan.Summary.HardConflicts != 0 {
		t.Fatalf("summary = %+v, want no hard conflict", plan.Summary)
	}
	if plan.Summary.RetirementEligible {
		t.Fatal("missing canonical facts must not be retirement eligible")
	}
	if len(plan.Identities) != 2 || len(plan.Passwords) != 1 {
		t.Fatalf("planned identities=%d passwords=%d, want 2 and 1", len(plan.Identities), len(plan.Passwords))
	}
	if plan.Summary.PlannedLoginIdentityInserts != 2 || plan.Summary.PlannedPasswordInserts != 1 {
		t.Fatalf("summary = %+v, want missing-only insert counts", plan.Summary)
	}
}

func TestAuthNLegacyPlanFailsClosedOnOwnershipAndUnknownFacts(t *testing.T) {
	key := authNProviderKey{Provider: "username", Realm: "default", Identifier: "operator"}
	state := authNCanonicalState{
		IdentitiesByKey: authNIdentityMap{
			key: {ID: 110, UserID: 999, Key: key},
		},
		IdentityIDs:     map[uint64]struct{}{110: {}},
		PasswordByLogin: make(map[uint64]uint64),
		CredentialIDs:   make(map[uint64]struct{}),
	}
	accounts := []legacyAuthNAccount{
		{ID: 10, UserID: 100, Type: "opera", ExternalID: "operator"},
		{ID: 11, UserID: 101, Type: "wc-offi", AppID: "wx", ExternalID: "openid"},
	}
	credentials := []legacyAuthNCredential{
		{ID: 20, AccountID: 10, Type: "password"},
		{ID: 21, AccountID: 10, Type: "unrecognized", IDP: "custom"},
	}

	plan := buildAuthNReconcilePlan(accounts, credentials, state, newAuthNLegacySummary(false))

	if plan.Summary.AccountOwnerConflicts != 1 || plan.Summary.AccountUnsupported != 1 {
		t.Fatalf("summary = %+v, want account conflicts", plan.Summary)
	}
	if plan.Summary.PasswordInvalid != 1 || plan.Summary.UnknownCredentials != 1 {
		t.Fatalf("summary = %+v, want credential conflicts", plan.Summary)
	}
	if plan.Summary.HardConflicts < 4 || plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want fail-closed result", plan.Summary)
	}
	if len(plan.Identities) != 0 || len(plan.Passwords) != 0 {
		t.Fatalf("conflicting plan must not schedule writes: %+v", plan)
	}
}

func TestAuthNLegacyPlanReportsOAuthMismatchUsingControlledAggregates(t *testing.T) {
	accounts := []legacyAuthNAccount{
		{ID: 10, UserID: 100, Type: "mock-consumer", ExternalID: "consumer"},
		{ID: 11, UserID: 101, Type: "wc-minip", AppID: "wx-app", ExternalID: "openid"},
	}
	credentials := []legacyAuthNCredential{
		{ID: 20, AccountID: 10, Type: "oauth_wx_minip"},
		{ID: 21, AccountID: 11, Type: "oauth_wx_minip"},
		{ID: 22, AccountID: 999, Type: "oauth_wx_scan"},
	}
	usernameKey := authNProviderKey{Provider: "username", Realm: "default", Identifier: "consumer"}
	wechatKey := authNProviderKey{Provider: "wechat_minip", Realm: "wx-app", Identifier: "openid"}
	state := authNCanonicalState{
		IdentitiesByKey: authNIdentityMap{
			usernameKey: {ID: 110, UserID: 100, Key: usernameKey},
			wechatKey:   {ID: 111, UserID: 101, Key: wechatKey},
		},
		IdentityIDs:     map[uint64]struct{}{110: {}, 111: {}},
		PasswordByLogin: make(map[uint64]uint64),
		CredentialIDs:   make(map[uint64]struct{}),
	}

	plan := buildAuthNReconcilePlan(accounts, credentials, state, newAuthNLegacySummary(false))

	if plan.Summary.OAuthPresent != 1 || plan.Summary.OAuthMissing != 2 {
		t.Fatalf("summary = %+v, want one present and two missing OAuth artifacts", plan.Summary)
	}
	if plan.Summary.OAuthWechatMinipRows != 2 || plan.Summary.OAuthWechatScanRows != 1 {
		t.Fatalf("summary = %+v, want controlled credential type counts", plan.Summary)
	}
	if plan.Summary.OAuthUsernameAccountRows != 1 || plan.Summary.OAuthWechatMinipAccountRows != 1 {
		t.Fatalf("summary = %+v, want controlled account type counts", plan.Summary)
	}
	if plan.Summary.OAuthProviderMismatches != 1 || plan.Summary.OAuthAccountOrphans != 1 {
		t.Fatalf("summary = %+v, want split OAuth blockers", plan.Summary)
	}
}

func TestAuthNLegacySummaryDoesNotContainSensitiveValues(t *testing.T) {
	plan := buildAuthNReconcilePlan(
		[]legacyAuthNAccount{{
			ID:         10,
			UserID:     100,
			Type:       "opera",
			ExternalID: "secret-username",
		}},
		[]legacyAuthNCredential{{
			ID:        20,
			AccountID: 10,
			Type:      "password",
			Material:  []byte("secret-password-hash"),
			Algo:      "argon2id",
		}},
		emptyCanonicalAuthNState(),
		newAuthNLegacySummary(false),
	)
	encoded, err := json.Marshal(plan.Summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret-username", "secret-password-hash", "argon2id"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("summary leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestAuthNLegacyApplySourceContainsNoCanonicalUpsert(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	sourcePath := filepath.Join(filepath.Dir(currentFile), "authn_legacy_reconcile.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(source)), "ON DUPLICATE KEY UPDATE") {
		t.Fatal("AuthN legacy reconciliation must never overwrite canonical rows")
	}
}

func emptyCanonicalAuthNState() authNCanonicalState {
	return authNCanonicalState{
		IdentitiesByKey: make(authNIdentityMap),
		IdentityIDs:     make(map[uint64]struct{}),
		PasswordByLogin: make(map[uint64]uint64),
		CredentialIDs:   make(map[uint64]struct{}),
	}
}
