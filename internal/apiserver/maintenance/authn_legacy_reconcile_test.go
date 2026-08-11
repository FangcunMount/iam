package maintenance

import (
	"database/sql"
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
		{ID: 10, UserID: 100, Type: "mock-consumer", AppID: "mock-consumer", ExternalID: "consumer"},
		{ID: 11, UserID: 101, Type: "wc-minip", AppID: "wx-app", ExternalID: "openid"},
	}
	credentials := []legacyAuthNCredential{
		{ID: 20, AccountID: 10, Type: "oauth_wx_minip", AppID: "mock-consumer", IDPIdentifier: "consumer", Status: 1},
		{ID: 21, AccountID: 11, Type: "oauth_wx_minip", AppID: "wx-app", IDPIdentifier: "openid", Status: 1},
		{ID: 22, AccountID: 999, Type: "oauth_wx_scan", AppID: "wx-open", IDPIdentifier: "orphan", Status: 1},
	}
	usernameKey := authNProviderKey{Provider: "username", Realm: "default", Identifier: "consumer"}
	wechatKey := authNProviderKey{Provider: "wechat_minip", Realm: "wx-app", Identifier: "openid"}
	consumerWechatKey := authNProviderKey{Provider: "wechat_minip", Realm: "mock-consumer", Identifier: "consumer-openid"}
	state := authNCanonicalState{
		IdentitiesByKey: authNIdentityMap{
			usernameKey:       {ID: 110, UserID: 100, Key: usernameKey},
			wechatKey:         {ID: 111, UserID: 101, Key: wechatKey},
			consumerWechatKey: {ID: 112, UserID: 100, Key: consumerWechatKey, GlobalIdentifier: "consumer"},
		},
		IdentityIDs:     map[uint64]struct{}{110: {}, 111: {}, 112: {}},
		PasswordByLogin: make(map[uint64]uint64),
		CredentialIDs:   make(map[uint64]struct{}),
	}

	plan := buildAuthNReconcilePlan(accounts, credentials, state, newAuthNLegacySummary(false))

	if plan.Summary.OAuthPresent != 2 || plan.Summary.OAuthMissing != 0 {
		t.Fatalf("summary = %+v, want direct and global canonical OAuth preservation", plan.Summary)
	}
	if plan.Summary.OAuthWechatMinipRows != 2 || plan.Summary.OAuthWechatScanRows != 1 {
		t.Fatalf("summary = %+v, want controlled credential type counts", plan.Summary)
	}
	if plan.Summary.OAuthUsernameAccountRows != 1 || plan.Summary.OAuthWechatMinipAccountRows != 1 {
		t.Fatalf("summary = %+v, want controlled account type counts", plan.Summary)
	}
	if plan.Summary.OAuthMockConsumerRows != 1 || plan.Summary.OAuthOperaAccountRows != 0 {
		t.Fatalf("summary = %+v, want separated username account types", plan.Summary)
	}
	if plan.Summary.OAuthDirectIdentityMatches != 1 || plan.Summary.OAuthGlobalIdentityMatches != 1 {
		t.Fatalf("summary = %+v, want direct and global credential-shape matches", plan.Summary)
	}
	if plan.Summary.OAuthOwnerProviderMatches != 2 || plan.Summary.OAuthOwnerRealmMatches != 2 {
		t.Fatalf("summary = %+v, want owner/provider aggregate matches", plan.Summary)
	}
	if plan.Summary.OAuthAppIDAccountMatches != 2 || plan.Summary.OAuthIdentifierExtMatches != 2 {
		t.Fatalf("summary = %+v, want account relation aggregates", plan.Summary)
	}
	if plan.Summary.OAuthMockAppIDLiteralRows != 1 || plan.Summary.OAuthMockAppIDMatches != 1 ||
		plan.Summary.OAuthMockIdentifierMatches != 1 || plan.Summary.OAuthMockMaterialRows != 0 {
		t.Fatalf("summary = %+v, want mock-consumer artifact shape", plan.Summary)
	}
	if plan.Summary.OAuthActiveRows != 3 || plan.Summary.OAuthOrphanActiveRows != 1 {
		t.Fatalf("summary = %+v, want disjoint credential-state aggregates", plan.Summary)
	}
	if plan.Summary.OAuthProviderMismatches != 1 || plan.Summary.OAuthAccountOrphans != 1 {
		t.Fatalf("summary = %+v, want split OAuth blockers", plan.Summary)
	}
	if plan.Summary.OAuthUnreachableRows != 1 || !plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want runtime-unreachable orphan excluded from migration", plan.Summary)
	}
}

func TestAuthNLegacyPlanMigratesOAuthAsMarkerScopedLoginIdentity(t *testing.T) {
	account := legacyAuthNAccount{
		ID: 10, UserID: 100, Type: "mock-consumer", ExternalID: "consumer", Status: 1,
	}
	credential := legacyAuthNCredential{
		ID: 20, AccountID: 10, Type: "oauth_wx_minip", AppID: "wx-app",
		IDPIdentifier: "legacy-open-or-union", Status: 1,
	}

	plan := buildAuthNReconcilePlan(
		[]legacyAuthNAccount{account},
		[]legacyAuthNCredential{credential},
		emptyCanonicalAuthNState(),
		newAuthNLegacySummary(false),
	)

	if plan.Summary.HardConflicts != 0 || plan.Summary.OAuthMissing != 1 ||
		plan.Summary.PlannedLoginIdentityInserts != 2 {
		t.Fatalf("summary = %+v, want account and OAuth identity inserts", plan.Summary)
	}
	if len(plan.Identities) != 2 {
		t.Fatalf("identities = %d, want account and OAuth identities", len(plan.Identities))
	}
	oauth := plan.Identities[1]
	if oauth.UserID != 100 || oauth.Key != (authNProviderKey{
		Provider: "wechat_minip", Realm: "wx-app", Identifier: "legacy-open-or-union",
	}) {
		t.Fatalf("OAuth identity = %+v", oauth)
	}
	var meta map[string]any
	if err := json.Unmarshal(oauth.Meta, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["legacy_identifier_semantics"] != "openid_or_unionid" {
		t.Fatalf("OAuth identity meta = %v", meta)
	}
}

func TestAuthNLegacyPlanUsesCanonicalOAuthOwnerAsAuthority(t *testing.T) {
	account := legacyAuthNAccount{ID: 10, UserID: 100, Type: "mock-consumer", ExternalID: "consumer"}
	credential := legacyAuthNCredential{
		ID: 20, AccountID: 10, Type: "oauth_wx_minip", AppID: "wx-app", IDPIdentifier: "shared", Status: 1,
	}
	key := authNProviderKey{Provider: "wechat_minip", Realm: "wx-app", Identifier: "shared"}
	accountKey := authNProviderKey{Provider: "username", Realm: "default", Identifier: "consumer"}
	state := emptyCanonicalAuthNState()
	state.IdentitiesByKey[key] = canonicalAuthNIdentity{ID: 200, UserID: 999, Key: key}
	state.IdentitiesByKey[accountKey] = canonicalAuthNIdentity{ID: 201, UserID: 100, Key: accountKey}
	state.IdentityIDs[200] = struct{}{}
	state.IdentityIDs[201] = struct{}{}

	plan := buildAuthNReconcilePlan(
		[]legacyAuthNAccount{account},
		[]legacyAuthNCredential{credential},
		state,
		newAuthNLegacySummary(false),
	)

	if plan.Summary.OAuthCanonicalOverrides != 1 || plan.Summary.OAuthDirectOverrides != 1 ||
		plan.Summary.HardConflicts != 0 || !plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want canonical owner override without canonical mutation", plan.Summary)
	}
	if len(plan.Identities) != 0 {
		t.Fatalf("canonical authority must not be overwritten: %+v", plan.Identities)
	}
}

func TestAuthNLegacyPlanFailsClosedOnAmbiguousOAuthSources(t *testing.T) {
	accounts := []legacyAuthNAccount{
		{ID: 10, UserID: 100, Type: "mock-consumer", ExternalID: "one"},
		{ID: 11, UserID: 101, Type: "mock-consumer", ExternalID: "two"},
	}
	credentials := []legacyAuthNCredential{
		{ID: 20, AccountID: 10, Type: "oauth_wx_minip", AppID: "wx-app", IDPIdentifier: "shared", Status: 1},
		{ID: 21, AccountID: 11, Type: "oauth_wx_minip", AppID: "wx-app", IDPIdentifier: "shared", Status: 1},
	}

	state := emptyCanonicalAuthNState()
	for index, account := range accounts {
		key := authNProviderKey{Provider: "username", Realm: "default", Identifier: account.ExternalID}
		id := uint64(200 + index)
		state.IdentitiesByKey[key] = canonicalAuthNIdentity{ID: id, UserID: account.UserID, Key: key}
		state.IdentityIDs[id] = struct{}{}
	}
	plan := buildAuthNReconcilePlan(accounts, credentials, state, newAuthNLegacySummary(false))

	if plan.Summary.OAuthDuplicateSourceKeys != 1 || plan.Summary.OAuthAmbiguousSourceKeys != 1 ||
		plan.Summary.HardConflicts == 0 || plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want fail-closed cross-owner OAuth key", plan.Summary)
	}
	if plan.Summary.OAuthMissing != 0 {
		t.Fatalf("ambiguous key must not schedule an arbitrary owner: %+v", plan.Summary)
	}
}

func TestAuthNLegacyPlanFailsClosedOnAmbiguousCanonicalGlobalLookup(t *testing.T) {
	account := legacyAuthNAccount{ID: 10, UserID: 100, Type: "mock-consumer", ExternalID: "consumer"}
	credential := legacyAuthNCredential{
		ID: 20, AccountID: 10, Type: "oauth_wx_minip", AppID: "wx-app", IDPIdentifier: "union", Status: 1,
	}
	state := emptyCanonicalAuthNState()
	accountKey := authNProviderKey{Provider: "username", Realm: "default", Identifier: "consumer"}
	globalOne := authNProviderKey{Provider: "wechat_minip", Realm: "wx-one", Identifier: "open-one"}
	globalTwo := authNProviderKey{Provider: "wechat_minip", Realm: "wx-two", Identifier: "open-two"}
	state.IdentitiesByKey[accountKey] = canonicalAuthNIdentity{ID: 200, UserID: 100, Key: accountKey}
	state.IdentitiesByKey[globalOne] = canonicalAuthNIdentity{
		ID: 201, UserID: 100, Key: globalOne, GlobalIdentifier: "union",
	}
	state.IdentitiesByKey[globalTwo] = canonicalAuthNIdentity{
		ID: 202, UserID: 100, Key: globalTwo, GlobalIdentifier: "union",
	}
	state.IdentityIDs = map[uint64]struct{}{200: {}, 201: {}, 202: {}}

	plan := buildAuthNReconcilePlan(
		[]legacyAuthNAccount{account},
		[]legacyAuthNCredential{credential},
		state,
		newAuthNLegacySummary(false),
	)

	if plan.Summary.OAuthAmbiguousGlobalKeys != 1 || plan.Summary.HardConflicts == 0 ||
		plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want fail-closed ambiguous global lookup", plan.Summary)
	}
}

func TestLimitAuthNReconcilePlanKeepsDependencyOrder(t *testing.T) {
	plan := authNReconcilePlan{
		Identities: []plannedAuthNIdentity{{ID: 1}, {ID: 2}},
		Passwords:  []plannedAuthNPassword{{ID: 3}, {ID: 4}},
	}
	limited := limitAuthNReconcilePlan(plan, 3)
	if len(limited.Identities) != 2 || len(limited.Passwords) != 1 || limited.Passwords[0].ID != 3 {
		t.Fatalf("limited plan = %+v", limited)
	}
}

func TestAuthNLegacyPlanReportsOrphanCredentialStates(t *testing.T) {
	credentials := []legacyAuthNCredential{
		{ID: 20, AccountID: 999, Type: "password", Material: []byte("hash"), Algo: "bcrypt", Status: 1},
		{ID: 21, AccountID: 998, Type: "password", Material: []byte("hash"), Algo: "bcrypt", Status: 0},
		{ID: 22, AccountID: 997, Type: "password", Material: []byte("hash"), Algo: "bcrypt", Status: 1, DeletedAt: sql.NullTime{Valid: true}},
	}

	state := emptyCanonicalAuthNState()
	state.IdentityIDs[999] = struct{}{}
	state.PasswordByLogin[999] = 120
	state.PasswordFacts[999] = canonicalAuthNPassword{Material: []byte("hash"), Algo: "bcrypt"}
	plan := buildAuthNReconcilePlan(nil, credentials, state, newAuthNLegacySummary(false))

	if plan.Summary.PasswordOrphans != 3 || plan.Summary.PasswordOrphanActiveRows != 1 ||
		plan.Summary.PasswordOrphanDisabledRows != 1 || plan.Summary.PasswordOrphanDeletedRows != 1 {
		t.Fatalf("summary = %+v, want disjoint password orphan states", plan.Summary)
	}
	if plan.Summary.PasswordOrphanIdentityIDs != 1 || plan.Summary.PasswordOrphanExactMatches != 1 {
		t.Fatalf("summary = %+v, want canonical orphan preservation aggregates", plan.Summary)
	}
	if plan.Summary.PasswordUnreachableRows != 3 || plan.Summary.HardConflicts != 0 || !plan.Summary.RetirementEligible {
		t.Fatalf("summary = %+v, want account-less password rows classified as runtime unreachable", plan.Summary)
	}
}

func TestAuthNLegacyPlanReportsOAuthOrphanCanonicalRelations(t *testing.T) {
	directKey := authNProviderKey{Provider: "wechat_open", Realm: "open-app", Identifier: "direct-id"}
	globalKey := authNProviderKey{Provider: "wechat_open", Realm: "open-app", Identifier: "other-id"}
	state := emptyCanonicalAuthNState()
	state.IdentitiesByKey[directKey] = canonicalAuthNIdentity{ID: 999, UserID: 100, Key: directKey}
	state.IdentitiesByKey[globalKey] = canonicalAuthNIdentity{
		ID: 998, UserID: 101, Key: globalKey, GlobalIdentifier: "global-id",
	}
	state.IdentityIDs[999] = struct{}{}
	state.IdentityIDs[998] = struct{}{}
	credentials := []legacyAuthNCredential{
		{ID: 20, AccountID: 999, Type: "oauth_wx_scan", AppID: "open-app", IDPIdentifier: "direct-id", Status: 1},
		{ID: 21, AccountID: 997, Type: "oauth_wx_scan", AppID: "open-app", IDPIdentifier: "global-id", Status: 1},
	}

	plan := buildAuthNReconcilePlan(nil, credentials, state, newAuthNLegacySummary(false))

	if plan.Summary.OAuthAccountOrphans != 2 || plan.Summary.OAuthOrphanIdentityIDs != 1 ||
		plan.Summary.OAuthOrphanDirectMatches != 1 || plan.Summary.OAuthOrphanGlobalMatches != 1 {
		t.Fatalf("summary = %+v, want canonical OAuth orphan relations", plan.Summary)
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
		PasswordFacts:   make(map[uint64]canonicalAuthNPassword),
		CredentialIDs:   make(map[uint64]struct{}),
	}
}
