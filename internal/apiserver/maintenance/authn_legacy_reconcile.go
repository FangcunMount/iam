package maintenance

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
)

const AuthNLegacyReconcileFormatVersion = 3

var ErrAuthNLegacyConflicts = errors.New("authn legacy reconciliation has hard conflicts")

type AuthNLegacyReconcileOptions struct {
	Apply bool
}

// AuthNLegacyReconcileSummary contains aggregate evidence only. It must never
// include identifiers, credential material, database addresses, or secrets.
type AuthNLegacyReconcileSummary struct {
	FormatVersion               int    `json:"format_version"`
	Mode                        string `json:"mode"`
	State                       string `json:"state"`
	LegacyCredentialTable       string `json:"legacy_credential_table,omitempty"`
	LegacyAccounts              int    `json:"legacy_accounts"`
	AccountPresent              int    `json:"account_present"`
	AccountMissing              int    `json:"account_missing"`
	AccountInvalid              int    `json:"account_invalid"`
	AccountUnsupported          int    `json:"account_unsupported"`
	AccountOwnerConflicts       int    `json:"account_owner_conflicts"`
	AccountGlobalIDConflicts    int    `json:"account_global_id_conflicts"`
	AccountDuplicateSources     int    `json:"account_duplicate_sources"`
	LegacyCredentials           int    `json:"legacy_credentials"`
	PasswordPresent             int    `json:"password_present"`
	PasswordMissing             int    `json:"password_missing"`
	PasswordInvalid             int    `json:"password_invalid"`
	PasswordOrphans             int    `json:"password_orphans"`
	PasswordDuplicateSources    int    `json:"password_duplicate_sources"`
	PhonePresent                int    `json:"phone_present"`
	PhoneMissing                int    `json:"phone_missing"`
	PhoneBlankIdentifiers       int    `json:"phone_blank_identifiers"`
	PhoneOrphans                int    `json:"phone_orphans"`
	PhoneOwnerConflicts         int    `json:"phone_owner_conflicts"`
	PhoneDuplicateSources       int    `json:"phone_duplicate_sources"`
	OAuthPresent                int    `json:"oauth_present"`
	OAuthMissing                int    `json:"oauth_missing"`
	OAuthWechatMinipRows        int    `json:"oauth_wechat_minip_rows"`
	OAuthWechatOpenRows         int    `json:"oauth_wechat_open_rows"`
	OAuthWechatScanRows         int    `json:"oauth_wechat_scan_rows"`
	OAuthWecomRows              int    `json:"oauth_wecom_rows"`
	OAuthOperaAccountRows       int    `json:"oauth_opera_account_rows"`
	OAuthMockConsumerRows       int    `json:"oauth_mock_consumer_account_rows"`
	OAuthUsernameAccountRows    int    `json:"oauth_username_account_rows"`
	OAuthWechatMinipAccountRows int    `json:"oauth_wechat_minip_account_rows"`
	OAuthWecomAccountRows       int    `json:"oauth_wecom_account_rows"`
	OAuthUnsupportedAccountRows int    `json:"oauth_unsupported_account_rows"`
	OAuthAccountOrphans         int    `json:"oauth_account_orphans"`
	OAuthOrphanActiveRows       int    `json:"oauth_orphan_active_rows"`
	OAuthOrphanDisabledRows     int    `json:"oauth_orphan_disabled_rows"`
	OAuthOrphanDeletedRows      int    `json:"oauth_orphan_deleted_rows"`
	OAuthIdentityUnresolved     int    `json:"oauth_identity_unresolved"`
	OAuthProviderMismatches     int    `json:"oauth_provider_mismatches"`
	OAuthBlankAppIDs            int    `json:"oauth_blank_app_ids"`
	OAuthBlankIdentifiers       int    `json:"oauth_blank_identifiers"`
	OAuthDirectIdentityMatches  int    `json:"oauth_direct_identity_matches"`
	OAuthDirectOwnerConflicts   int    `json:"oauth_direct_owner_conflicts"`
	OAuthGlobalIdentityMatches  int    `json:"oauth_global_identity_matches"`
	OAuthOwnerProviderMatches   int    `json:"oauth_owner_provider_matches"`
	OAuthOwnerRealmMatches      int    `json:"oauth_owner_provider_realm_matches"`
	OAuthDuplicateSourceKeys    int    `json:"oauth_duplicate_source_keys"`
	OAuthActiveRows             int    `json:"oauth_active_rows"`
	OAuthDisabledRows           int    `json:"oauth_disabled_rows"`
	OAuthDeletedRows            int    `json:"oauth_deleted_rows"`
	PasswordOrphanActiveRows    int    `json:"password_orphan_active_rows"`
	PasswordOrphanDisabledRows  int    `json:"password_orphan_disabled_rows"`
	PasswordOrphanDeletedRows   int    `json:"password_orphan_deleted_rows"`
	PasswordOrphanIdentityIDs   int    `json:"password_orphan_identity_id_matches"`
	PasswordOrphanExactMatches  int    `json:"password_orphan_exact_canonical_matches"`
	OAuthAppIDAccountMatches    int    `json:"oauth_app_id_account_matches"`
	OAuthIdentifierExtMatches   int    `json:"oauth_identifier_account_external_matches"`
	OAuthIdentifierGlobalMatch  int    `json:"oauth_identifier_account_global_matches"`
	OAuthIdentifierAtAppMatches int    `json:"oauth_identifier_at_app_account_external_matches"`
	OAuthMockAppIDLiteralRows   int    `json:"oauth_mock_app_id_literal_rows"`
	OAuthMockAppIDMatches       int    `json:"oauth_mock_app_id_account_matches"`
	OAuthMockIdentifierMatches  int    `json:"oauth_mock_identifier_account_external_matches"`
	OAuthMockMaterialRows       int    `json:"oauth_mock_material_rows"`
	OAuthMockParamsRows         int    `json:"oauth_mock_params_rows"`
	OAuthMockOwnerConflicts     int    `json:"oauth_mock_direct_owner_conflicts"`
	OAuthOrphanIdentityIDs      int    `json:"oauth_orphan_identity_id_matches"`
	OAuthOrphanDirectMatches    int    `json:"oauth_orphan_direct_identity_matches"`
	OAuthOrphanGlobalMatches    int    `json:"oauth_orphan_global_identity_matches"`
	UnknownCredentials          int    `json:"unknown_credentials"`
	PlannedLoginIdentityInserts int    `json:"planned_login_identity_inserts"`
	PlannedPasswordInserts      int    `json:"planned_password_inserts"`
	AppliedLoginIdentityInserts int    `json:"applied_login_identity_inserts"`
	AppliedPasswordInserts      int    `json:"applied_password_inserts"`
	HardConflicts               int    `json:"hard_conflicts"`
	RetirementEligible          bool   `json:"retirement_eligible"`
}

type legacyAuthNAccount struct {
	ID             uint64
	UserID         uint64
	Type           string
	AppID          string
	ExternalID     string
	UniqueID       string
	ScopedTenantID uint64
	Profile        []byte
	Meta           []byte
	Status         int
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	DeletedAt      sql.NullTime
	CreatedBy      uint64
	UpdatedBy      uint64
	DeletedBy      uint64
	Version        uint64
}

type legacyAuthNCredential struct {
	ID             uint64
	AccountID      uint64
	Type           string
	IDP            string
	IDPIdentifier  string
	AppID          string
	Material       []byte
	Algo           string
	ParamsJSON     []byte
	Status         int
	FailedAttempts int
	LockedUntil    sql.NullTime
	LastSuccessAt  sql.NullTime
	LastFailureAt  sql.NullTime
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	DeletedAt      sql.NullTime
	CreatedBy      uint64
	UpdatedBy      uint64
	DeletedBy      uint64
	Version        uint64
}

type authNProviderKey struct {
	Provider   string
	Realm      string
	Identifier string
}

type authNOwnerProviderKey struct {
	UserID   uint64
	Provider string
}

type authNOwnerProviderRealmKey struct {
	UserID   uint64
	Provider string
	Realm    string
}

type authNOwnerProviderRealmGlobalKey struct {
	UserID           uint64
	Provider         string
	Realm            string
	GlobalIdentifier string
}

type authNProviderRealmGlobalKey struct {
	Provider         string
	Realm            string
	GlobalIdentifier string
}

type canonicalAuthNIdentity struct {
	ID               uint64
	UserID           uint64
	Key              authNProviderKey
	GlobalIdentifier string
}

type plannedAuthNIdentity struct {
	ID               uint64
	UserID           uint64
	Key              authNProviderKey
	GlobalIdentifier string
	Status           string
	VerifiedAt       sql.NullTime
	LinkedAt         time.Time
	Profile          []byte
	Meta             []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        sql.NullTime
	CreatedBy        uint64
	UpdatedBy        uint64
	DeletedBy        uint64
	Version          uint64
}

type plannedAuthNPassword struct {
	ID              uint64
	LoginIdentityID uint64
	Material        []byte
	Algo            string
	ParamsJSON      []byte
	Status          string
	FailedAttempts  int
	LockedUntil     sql.NullTime
	LastSuccessAt   sql.NullTime
	LastFailureAt   sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       sql.NullTime
	CreatedBy       uint64
	UpdatedBy       uint64
	DeletedBy       uint64
	Version         uint64
}

type authNCanonicalState struct {
	IdentitiesByKey authNIdentityMap
	IdentityIDs     map[uint64]struct{}
	PasswordByLogin map[uint64]uint64
	PasswordFacts   map[uint64]canonicalAuthNPassword
	CredentialIDs   map[uint64]struct{}
}

type canonicalAuthNPassword struct {
	Material []byte
	Algo     string
}

type authNIdentityMap map[authNProviderKey]canonicalAuthNIdentity

type authNAccountResolution struct {
	Identity canonicalAuthNIdentity
	OK       bool
}

type authNReconcilePlan struct {
	Summary    AuthNLegacyReconcileSummary
	Identities []plannedAuthNIdentity
	Passwords  []plannedAuthNPassword
}

type authNQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// ReconcileAuthNLegacy performs a dry-run by default. Apply mode inserts only
// missing canonical facts and never updates existing canonical rows.
func ReconcileAuthNLegacy(
	ctx context.Context,
	db *sql.DB,
	opts AuthNLegacyReconcileOptions,
) (AuthNLegacyReconcileSummary, error) {
	summary := newAuthNLegacySummary(opts.Apply)
	if db == nil {
		return summary, fmt.Errorf("database is required")
	}

	if !opts.Apply {
		plan, err := analyzeAuthNLegacy(ctx, db, summary)
		if err != nil {
			return plan.Summary, err
		}
		if plan.Summary.HardConflicts != 0 {
			return plan.Summary, ErrAuthNLegacyConflicts
		}
		return plan.Summary, nil
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return summary, fmt.Errorf("begin authn legacy reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	plan, err := analyzeAuthNLegacy(ctx, tx, summary)
	if err != nil {
		return plan.Summary, err
	}
	if plan.Summary.HardConflicts != 0 {
		return plan.Summary, ErrAuthNLegacyConflicts
	}
	if err := applyAuthNReconcilePlan(ctx, tx, plan); err != nil {
		return plan.Summary, err
	}

	post, err := analyzeAuthNLegacy(ctx, tx, newAuthNLegacySummary(true))
	if err != nil {
		return post.Summary, err
	}
	if !post.Summary.RetirementEligible {
		return post.Summary, fmt.Errorf("authn legacy reconciliation did not converge")
	}
	post.Summary.AppliedLoginIdentityInserts = len(plan.Identities)
	post.Summary.AppliedPasswordInserts = len(plan.Passwords)
	if err := tx.Commit(); err != nil {
		return post.Summary, fmt.Errorf("commit authn legacy reconciliation: %w", err)
	}
	return post.Summary, nil
}

func newAuthNLegacySummary(apply bool) AuthNLegacyReconcileSummary {
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	return AuthNLegacyReconcileSummary{
		FormatVersion: AuthNLegacyReconcileFormatVersion,
		Mode:          mode,
		State:         "unknown",
	}
}

func analyzeAuthNLegacy(
	ctx context.Context,
	q authNQueryer,
	base AuthNLegacyReconcileSummary,
) (authNReconcilePlan, error) {
	accountsPresent, err := authNTableExists(ctx, q, "auth_accounts")
	if err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	legacyCredentialsPresent, err := authNTableExists(ctx, q, "auth_credentials_legacy")
	if err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	oldCredentialShape, err := authNColumnExists(ctx, q, "auth_credentials", "account_id")
	if err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	if !accountsPresent && !legacyCredentialsPresent && !oldCredentialShape {
		if err := requireCanonicalAuthNSchema(ctx, q); err != nil {
			base.State = "canonical_schema_missing"
			return authNReconcilePlan{Summary: base}, err
		}
		base.State = "already_absent"
		base.RetirementEligible = true
		return authNReconcilePlan{Summary: base}, nil
	}
	if oldCredentialShape {
		base.State = "old_shaped_auth_credentials"
		base.HardConflicts = 1
		return authNReconcilePlan{Summary: base}, ErrAuthNLegacyConflicts
	}
	if !accountsPresent || !legacyCredentialsPresent {
		base.State = "partial_legacy_tables"
		base.HardConflicts = 1
		return authNReconcilePlan{Summary: base}, ErrAuthNLegacyConflicts
	}
	base.State = "present"
	base.LegacyCredentialTable = "auth_credentials_legacy"

	if err := requireCanonicalAuthNSchema(ctx, q); err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	accounts, err := loadLegacyAuthNAccounts(ctx, q)
	if err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	credentials, err := loadLegacyAuthNCredentials(ctx, q)
	if err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	canonical, err := loadCanonicalAuthNState(ctx, q)
	if err != nil {
		return authNReconcilePlan{Summary: base}, err
	}
	plan := buildAuthNReconcilePlan(accounts, credentials, canonical, base)
	return plan, nil
}

func requireCanonicalAuthNSchema(ctx context.Context, q authNQueryer) error {
	for _, table := range []string{"auth_login_identities", "auth_credentials"} {
		present, err := authNTableExists(ctx, q, table)
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("canonical authn table is missing")
		}
	}
	hasLoginIdentityID, err := authNColumnExists(ctx, q, "auth_credentials", "login_identity_id")
	if err != nil {
		return err
	}
	if !hasLoginIdentityID {
		return fmt.Errorf("canonical authn credential schema is missing login_identity_id")
	}
	return nil
}

func buildAuthNReconcilePlan(
	accounts []legacyAuthNAccount,
	credentials []legacyAuthNCredential,
	canonical authNCanonicalState,
	base AuthNLegacyReconcileSummary,
) authNReconcilePlan {
	plan := authNReconcilePlan{Summary: base}
	plan.Summary.LegacyAccounts = len(accounts)
	plan.Summary.LegacyCredentials = len(credentials)
	resolutions := make(map[uint64]authNAccountResolution, len(accounts))
	seenAccountKeys := make(map[authNProviderKey]uint64, len(accounts))
	ownerProviders := make(map[authNOwnerProviderKey]int, len(canonical.IdentitiesByKey))
	ownerProviderRealms := make(map[authNOwnerProviderRealmKey]int, len(canonical.IdentitiesByKey))
	ownerProviderRealmGlobals := make(map[authNOwnerProviderRealmGlobalKey]int, len(canonical.IdentitiesByKey))
	providerRealmGlobals := make(map[authNProviderRealmGlobalKey]int, len(canonical.IdentitiesByKey))
	for _, identity := range canonical.IdentitiesByKey {
		provider := strings.TrimSpace(identity.Key.Provider)
		realm := strings.TrimSpace(identity.Key.Realm)
		ownerProviders[authNOwnerProviderKey{UserID: identity.UserID, Provider: provider}]++
		ownerProviderRealms[authNOwnerProviderRealmKey{
			UserID: identity.UserID, Provider: provider, Realm: realm,
		}]++
		globalIdentifier := strings.TrimSpace(identity.GlobalIdentifier)
		if globalIdentifier != "" {
			ownerProviderRealmGlobals[authNOwnerProviderRealmGlobalKey{
				UserID: identity.UserID, Provider: provider, Realm: realm, GlobalIdentifier: globalIdentifier,
			}]++
			providerRealmGlobals[authNProviderRealmGlobalKey{
				Provider: provider, Realm: realm, GlobalIdentifier: globalIdentifier,
			}]++
		}
	}

	for _, account := range accounts {
		key, globalIdentifier, supported := legacyAccountProviderKey(account)
		if !supported {
			plan.Summary.AccountUnsupported++
			continue
		}
		if !validAuthNProviderKey(key) {
			plan.Summary.AccountInvalid++
			continue
		}
		if _, duplicate := seenAccountKeys[key]; duplicate {
			plan.Summary.AccountDuplicateSources++
			continue
		}
		seenAccountKeys[key] = account.ID

		identity, exists := canonical.IdentitiesByKey[key]
		if exists {
			if identity.UserID != account.UserID {
				plan.Summary.AccountOwnerConflicts++
				continue
			}
			if globalIdentifier != "" && identity.GlobalIdentifier != globalIdentifier {
				plan.Summary.AccountGlobalIDConflicts++
				continue
			}
			plan.Summary.AccountPresent++
			resolutions[account.ID] = authNAccountResolution{Identity: identity, OK: true}
			continue
		}

		row := authNIdentityFromLegacyAccount(account, key, globalIdentifier)
		row.ID = allocateAuthNID(row.ID, canonical.IdentityIDs)
		identity = canonicalAuthNIdentity{
			ID:               row.ID,
			UserID:           row.UserID,
			Key:              row.Key,
			GlobalIdentifier: row.GlobalIdentifier,
		}
		canonical.IdentitiesByKey[key] = identity
		plan.Identities = append(plan.Identities, row)
		plan.Summary.AccountMissing++
		resolutions[account.ID] = authNAccountResolution{Identity: identity, OK: true}
	}

	seenPasswords := make(map[uint64]uint64)
	seenPhones := make(map[authNProviderKey]uint64)
	seenOAuthKeys := make(map[authNProviderKey]uint64)
	reportedOAuthDuplicateKeys := make(map[authNProviderKey]struct{})
	accountsByID := make(map[uint64]legacyAuthNAccount, len(accounts))
	for _, account := range accounts {
		accountsByID[account.ID] = account
	}

	for _, credential := range credentials {
		switch classifyLegacyAuthNCredential(credential) {
		case "invalid_password":
			plan.Summary.PasswordInvalid++
		case "password":
			resolution, ok := resolutions[credential.AccountID]
			if !ok || !resolution.OK {
				plan.Summary.PasswordOrphans++
				countAuthNPasswordOrphanState(&plan.Summary, credential)
				if _, exists := canonical.IdentityIDs[credential.AccountID]; exists {
					plan.Summary.PasswordOrphanIdentityIDs++
				}
				if password, exists := canonical.PasswordFacts[credential.AccountID]; exists &&
					bytes.Equal(password.Material, credential.Material) && password.Algo == strings.TrimSpace(credential.Algo) {
					plan.Summary.PasswordOrphanExactMatches++
				}
				continue
			}
			if _, duplicate := seenPasswords[credential.AccountID]; duplicate {
				plan.Summary.PasswordDuplicateSources++
				continue
			}
			seenPasswords[credential.AccountID] = credential.ID
			if _, exists := canonical.PasswordByLogin[resolution.Identity.ID]; exists {
				plan.Summary.PasswordPresent++
				continue
			}
			row := authNPasswordFromLegacy(credential, resolution.Identity.ID)
			row.ID = allocateAuthNID(row.ID, canonical.CredentialIDs)
			canonical.PasswordByLogin[resolution.Identity.ID] = row.ID
			plan.Passwords = append(plan.Passwords, row)
			plan.Summary.PasswordMissing++
		case "phone":
			account, accountExists := accountsByID[credential.AccountID]
			resolution, resolved := resolutions[credential.AccountID]
			if !accountExists || !resolved || !resolution.OK {
				plan.Summary.PhoneOrphans++
				continue
			}
			identifier := strings.TrimSpace(credential.IDPIdentifier)
			if identifier == "" {
				plan.Summary.PhoneBlankIdentifiers++
				continue
			}
			key := authNProviderKey{Provider: "phone", Realm: "global", Identifier: identifier}
			if _, duplicate := seenPhones[key]; duplicate {
				plan.Summary.PhoneDuplicateSources++
				continue
			}
			seenPhones[key] = credential.ID
			if identity, exists := canonical.IdentitiesByKey[key]; exists {
				if identity.UserID != account.UserID {
					plan.Summary.PhoneOwnerConflicts++
					continue
				}
				plan.Summary.PhonePresent++
				continue
			}
			row := authNPhoneIdentityFromLegacy(account, credential, key)
			row.ID = allocateAuthNID(row.ID, canonical.IdentityIDs)
			canonical.IdentitiesByKey[key] = canonicalAuthNIdentity{ID: row.ID, UserID: row.UserID, Key: key}
			plan.Identities = append(plan.Identities, row)
			plan.Summary.PhoneMissing++
		case "oauth":
			countAuthNOAuthCredentialType(&plan.Summary, credential.Type)
			countAuthNOAuthCredentialState(&plan.Summary, credential)
			provider := oauthProvider(credential.Type)
			realm := strings.TrimSpace(credential.AppID)
			identifier := strings.TrimSpace(credential.IDPIdentifier)
			if realm == "" {
				plan.Summary.OAuthBlankAppIDs++
			}
			if identifier == "" {
				plan.Summary.OAuthBlankIdentifiers++
			}
			directKey := authNProviderKey{Provider: provider, Realm: realm, Identifier: identifier}
			if validAuthNProviderKey(directKey) {
				if _, duplicate := seenOAuthKeys[directKey]; duplicate {
					if _, reported := reportedOAuthDuplicateKeys[directKey]; !reported {
						plan.Summary.OAuthDuplicateSourceKeys++
						reportedOAuthDuplicateKeys[directKey] = struct{}{}
					}
				} else {
					seenOAuthKeys[directKey] = credential.ID
				}
			}
			account, accountExists := accountsByID[credential.AccountID]
			if !accountExists {
				plan.Summary.OAuthAccountOrphans++
				countAuthNOAuthOrphanState(&plan.Summary, credential)
				if _, exists := canonical.IdentityIDs[credential.AccountID]; exists {
					plan.Summary.OAuthOrphanIdentityIDs++
				}
				if _, exists := canonical.IdentitiesByKey[directKey]; exists {
					plan.Summary.OAuthOrphanDirectMatches++
				}
				if providerRealmGlobals[authNProviderRealmGlobalKey{
					Provider: provider, Realm: realm, GlobalIdentifier: identifier,
				}] > 0 {
					plan.Summary.OAuthOrphanGlobalMatches++
				}
				plan.Summary.OAuthMissing++
				continue
			}
			countAuthNOAuthAccountType(&plan.Summary, account.Type)
			countAuthNOAuthAccountRelations(&plan.Summary, account, credential)
			if identity, exists := canonical.IdentitiesByKey[directKey]; exists {
				if identity.UserID == account.UserID {
					plan.Summary.OAuthDirectIdentityMatches++
				} else {
					plan.Summary.OAuthDirectOwnerConflicts++
					if strings.TrimSpace(account.Type) == "mock-consumer" {
						plan.Summary.OAuthMockOwnerConflicts++
					}
				}
			}
			if ownerProviders[authNOwnerProviderKey{UserID: account.UserID, Provider: provider}] > 0 {
				plan.Summary.OAuthOwnerProviderMatches++
			}
			if ownerProviderRealms[authNOwnerProviderRealmKey{
				UserID: account.UserID, Provider: provider, Realm: realm,
			}] > 0 {
				plan.Summary.OAuthOwnerRealmMatches++
			}
			if identifier != "" && ownerProviderRealmGlobals[authNOwnerProviderRealmGlobalKey{
				UserID: account.UserID, Provider: provider, Realm: realm, GlobalIdentifier: identifier,
			}] > 0 {
				plan.Summary.OAuthGlobalIdentityMatches++
			}
			resolution, ok := resolutions[credential.AccountID]
			if !ok || !resolution.OK {
				plan.Summary.OAuthIdentityUnresolved++
				plan.Summary.OAuthMissing++
				continue
			}
			if resolution.Identity.Key.Provider != oauthProvider(credential.Type) {
				plan.Summary.OAuthProviderMismatches++
				plan.Summary.OAuthMissing++
				continue
			}
			plan.Summary.OAuthPresent++
		default:
			plan.Summary.UnknownCredentials++
		}
	}

	plan.Summary.PlannedLoginIdentityInserts = len(plan.Identities)
	plan.Summary.PlannedPasswordInserts = len(plan.Passwords)
	plan.Summary.HardConflicts = authNHardConflictCount(plan.Summary)
	plan.Summary.RetirementEligible = plan.Summary.HardConflicts == 0 &&
		plan.Summary.PlannedLoginIdentityInserts == 0 &&
		plan.Summary.PlannedPasswordInserts == 0
	return plan
}

func countAuthNOAuthCredentialType(summary *AuthNLegacyReconcileSummary, credentialType string) {
	switch strings.TrimSpace(credentialType) {
	case "oauth_wx_minip":
		summary.OAuthWechatMinipRows++
	case "oauth_wx_open":
		summary.OAuthWechatOpenRows++
	case "oauth_wx_scan":
		summary.OAuthWechatScanRows++
	case "oauth_wecom":
		summary.OAuthWecomRows++
	}
}

func countAuthNOAuthAccountType(summary *AuthNLegacyReconcileSummary, accountType string) {
	switch strings.TrimSpace(accountType) {
	case "opera":
		summary.OAuthOperaAccountRows++
		summary.OAuthUsernameAccountRows++
	case "mock-consumer":
		summary.OAuthMockConsumerRows++
		summary.OAuthUsernameAccountRows++
	case "wc-minip":
		summary.OAuthWechatMinipAccountRows++
	case "wc-com":
		summary.OAuthWecomAccountRows++
	default:
		summary.OAuthUnsupportedAccountRows++
	}
}

func countAuthNOAuthAccountRelations(
	summary *AuthNLegacyReconcileSummary,
	account legacyAuthNAccount,
	credential legacyAuthNCredential,
) {
	appID := strings.TrimSpace(credential.AppID)
	identifier := strings.TrimSpace(credential.IDPIdentifier)
	accountAppID := strings.TrimSpace(account.AppID)
	externalID := strings.TrimSpace(account.ExternalID)
	globalID := strings.TrimSpace(account.UniqueID)
	if appID != "" && appID == accountAppID {
		summary.OAuthAppIDAccountMatches++
	}
	if identifier != "" && identifier == externalID {
		summary.OAuthIdentifierExtMatches++
	}
	if identifier != "" && globalID != "" && identifier == globalID {
		summary.OAuthIdentifierGlobalMatch++
	}
	if identifier != "" && appID != "" && externalID == identifier+"@"+appID {
		summary.OAuthIdentifierAtAppMatches++
	}
	if strings.TrimSpace(account.Type) != "mock-consumer" {
		return
	}
	if appID == "mock-consumer" {
		summary.OAuthMockAppIDLiteralRows++
	}
	if appID != "" && appID == accountAppID {
		summary.OAuthMockAppIDMatches++
	}
	if identifier != "" && identifier == externalID {
		summary.OAuthMockIdentifierMatches++
	}
	if len(credential.Material) > 0 || strings.TrimSpace(credential.Algo) != "" {
		summary.OAuthMockMaterialRows++
	}
	if len(bytes.TrimSpace(credential.ParamsJSON)) > 0 && string(bytes.TrimSpace(credential.ParamsJSON)) != "null" {
		summary.OAuthMockParamsRows++
	}
}

func countAuthNOAuthCredentialState(summary *AuthNLegacyReconcileSummary, credential legacyAuthNCredential) {
	if credential.DeletedAt.Valid {
		summary.OAuthDeletedRows++
		return
	}
	if credential.Status != 1 {
		summary.OAuthDisabledRows++
		return
	}
	summary.OAuthActiveRows++
}

func countAuthNOAuthOrphanState(summary *AuthNLegacyReconcileSummary, credential legacyAuthNCredential) {
	if credential.DeletedAt.Valid {
		summary.OAuthOrphanDeletedRows++
		return
	}
	if credential.Status != 1 {
		summary.OAuthOrphanDisabledRows++
		return
	}
	summary.OAuthOrphanActiveRows++
}

func countAuthNPasswordOrphanState(summary *AuthNLegacyReconcileSummary, credential legacyAuthNCredential) {
	if credential.DeletedAt.Valid {
		summary.PasswordOrphanDeletedRows++
		return
	}
	if credential.Status != 1 {
		summary.PasswordOrphanDisabledRows++
		return
	}
	summary.PasswordOrphanActiveRows++
}

func authNHardConflictCount(summary AuthNLegacyReconcileSummary) int {
	return summary.AccountInvalid +
		summary.AccountUnsupported +
		summary.AccountOwnerConflicts +
		summary.AccountGlobalIDConflicts +
		summary.AccountDuplicateSources +
		summary.PasswordInvalid +
		summary.PasswordOrphans +
		summary.PasswordDuplicateSources +
		summary.PhoneBlankIdentifiers +
		summary.PhoneOrphans +
		summary.PhoneOwnerConflicts +
		summary.PhoneDuplicateSources +
		summary.OAuthMissing +
		summary.UnknownCredentials
}

func applyAuthNReconcilePlan(ctx context.Context, tx *sql.Tx, plan authNReconcilePlan) error {
	for _, row := range plan.Identities {
		if _, err := tx.ExecContext(ctx, `INSERT INTO auth_login_identities
(id, user_id, provider, realm, identifier, global_identifier, status, verified_at, linked_at,
 profile_json, meta_json, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version)
VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.ID, row.UserID, row.Key.Provider, row.Key.Realm, row.Key.Identifier,
			row.GlobalIdentifier, row.Status, nullTimeValue(row.VerifiedAt), row.LinkedAt,
			nilIfEmptyBytes(row.Profile), nilIfEmptyBytes(row.Meta), row.CreatedAt, row.UpdatedAt,
			nullTimeValue(row.DeletedAt), row.CreatedBy, row.UpdatedBy, row.DeletedBy, row.Version,
		); err != nil {
			return fmt.Errorf("insert missing canonical login identity: %w", err)
		}
	}
	for _, row := range plan.Passwords {
		if _, err := tx.ExecContext(ctx, `INSERT INTO auth_credentials
(id, login_identity_id, type, material, algo, params_json, status, failed_attempts, locked_until,
 last_success_at, last_failure_at, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version)
VALUES (?, ?, 'password', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.ID, row.LoginIdentityID, nilIfEmptyBytes(row.Material), emptyStringToNil(row.Algo),
			nilIfEmptyBytes(row.ParamsJSON), row.Status, row.FailedAttempts, nullTimeValue(row.LockedUntil),
			nullTimeValue(row.LastSuccessAt), nullTimeValue(row.LastFailureAt), row.CreatedAt, row.UpdatedAt,
			nullTimeValue(row.DeletedAt), row.CreatedBy, row.UpdatedBy, row.DeletedBy, row.Version,
		); err != nil {
			return fmt.Errorf("insert missing canonical password credential: %w", err)
		}
	}
	return nil
}

func loadLegacyAuthNAccounts(ctx context.Context, q authNQueryer) ([]legacyAuthNAccount, error) {
	scopedExpr := "0 AS scoped_tenant_id"
	hasScoped, err := authNColumnExists(ctx, q, "auth_accounts", "scoped_tenant_id")
	if err != nil {
		return nil, err
	}
	if hasScoped {
		scopedExpr = "scoped_tenant_id"
	}
	query := fmt.Sprintf(`SELECT id, user_id, type, COALESCE(app_id, ''), external_id,
COALESCE(unique_id, ''), %s, profile, meta, status, created_at, updated_at, deleted_at,
created_by, updated_by, deleted_by, version
FROM auth_accounts ORDER BY id`, scopedExpr)
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load legacy authn accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []legacyAuthNAccount
	for rows.Next() {
		var account legacyAuthNAccount
		if err := rows.Scan(
			&account.ID, &account.UserID, &account.Type, &account.AppID, &account.ExternalID,
			&account.UniqueID, &account.ScopedTenantID, &account.Profile, &account.Meta, &account.Status,
			&account.CreatedAt, &account.UpdatedAt, &account.DeletedAt, &account.CreatedBy,
			&account.UpdatedBy, &account.DeletedBy, &account.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy authn account: %w", err)
		}
		result = append(result, account)
	}
	return result, rows.Err()
}

func loadLegacyAuthNCredentials(ctx context.Context, q authNQueryer) ([]legacyAuthNCredential, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, account_id, type, COALESCE(idp, ''), idp_identifier,
COALESCE(app_id, ''), material, COALESCE(algo, ''), params_json, status, failed_attempts,
locked_until, last_success_at, last_failure_at, created_at, updated_at, deleted_at,
created_by, updated_by, deleted_by, version
FROM auth_credentials_legacy ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load legacy authn credentials: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []legacyAuthNCredential
	for rows.Next() {
		var credential legacyAuthNCredential
		if err := rows.Scan(
			&credential.ID, &credential.AccountID, &credential.Type, &credential.IDP,
			&credential.IDPIdentifier, &credential.AppID, &credential.Material, &credential.Algo,
			&credential.ParamsJSON, &credential.Status, &credential.FailedAttempts,
			&credential.LockedUntil, &credential.LastSuccessAt, &credential.LastFailureAt,
			&credential.CreatedAt, &credential.UpdatedAt, &credential.DeletedAt,
			&credential.CreatedBy, &credential.UpdatedBy, &credential.DeletedBy, &credential.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy authn credential: %w", err)
		}
		result = append(result, credential)
	}
	return result, rows.Err()
}

func loadCanonicalAuthNState(ctx context.Context, q authNQueryer) (authNCanonicalState, error) {
	state := authNCanonicalState{
		IdentitiesByKey: make(authNIdentityMap),
		IdentityIDs:     make(map[uint64]struct{}),
		PasswordByLogin: make(map[uint64]uint64),
		PasswordFacts:   make(map[uint64]canonicalAuthNPassword),
		CredentialIDs:   make(map[uint64]struct{}),
	}
	identityRows, err := q.QueryContext(ctx, `SELECT id, user_id, provider, realm, identifier, global_identifier
FROM auth_login_identities`)
	if err != nil {
		return state, fmt.Errorf("load canonical authn identities: %w", err)
	}
	for identityRows.Next() {
		var identity canonicalAuthNIdentity
		var global sql.NullString
		if err := identityRows.Scan(
			&identity.ID, &identity.UserID, &identity.Key.Provider, &identity.Key.Realm,
			&identity.Key.Identifier, &global,
		); err != nil {
			_ = identityRows.Close()
			return state, fmt.Errorf("scan canonical authn identity: %w", err)
		}
		if global.Valid {
			identity.GlobalIdentifier = global.String
		}
		state.IdentitiesByKey[identity.Key] = identity
		state.IdentityIDs[identity.ID] = struct{}{}
	}
	if err := identityRows.Err(); err != nil {
		_ = identityRows.Close()
		return state, err
	}
	_ = identityRows.Close()

	credentialRows, err := q.QueryContext(ctx, `SELECT id, login_identity_id, type, material, algo FROM auth_credentials`)
	if err != nil {
		return state, fmt.Errorf("load canonical authn credentials: %w", err)
	}
	defer func() { _ = credentialRows.Close() }()
	for credentialRows.Next() {
		var id, loginIdentityID uint64
		var credentialType string
		var material []byte
		var algo sql.NullString
		if err := credentialRows.Scan(&id, &loginIdentityID, &credentialType, &material, &algo); err != nil {
			return state, fmt.Errorf("scan canonical authn credential: %w", err)
		}
		state.CredentialIDs[id] = struct{}{}
		if credentialType == "password" {
			state.PasswordByLogin[loginIdentityID] = id
			state.PasswordFacts[loginIdentityID] = canonicalAuthNPassword{
				Material: cloneAuthNBytes(material),
				Algo:     strings.TrimSpace(algo.String),
			}
		}
	}
	return state, credentialRows.Err()
}

func legacyAccountProviderKey(account legacyAuthNAccount) (authNProviderKey, string, bool) {
	switch strings.TrimSpace(account.Type) {
	case "opera":
		realm := "default"
		if account.ScopedTenantID != 0 {
			realm = strconv.FormatUint(account.ScopedTenantID, 10)
		}
		return authNProviderKey{Provider: "username", Realm: realm, Identifier: strings.TrimSpace(account.ExternalID)}, "", true
	case "mock-consumer":
		return authNProviderKey{Provider: "username", Realm: "default", Identifier: strings.TrimSpace(account.ExternalID)}, "", true
	case "wc-minip":
		return authNProviderKey{Provider: "wechat_minip", Realm: strings.TrimSpace(account.AppID), Identifier: strings.TrimSpace(account.ExternalID)}, strings.TrimSpace(account.UniqueID), true
	case "wc-com":
		return authNProviderKey{Provider: "wecom", Realm: strings.TrimSpace(account.AppID), Identifier: strings.TrimSpace(account.ExternalID)}, strings.TrimSpace(account.UniqueID), true
	default:
		return authNProviderKey{}, "", false
	}
}

func validAuthNProviderKey(key authNProviderKey) bool {
	return key.Provider != "" && key.Realm != "" && key.Identifier != ""
}

func classifyLegacyAuthNCredential(credential legacyAuthNCredential) string {
	typeName := strings.TrimSpace(credential.Type)
	idp := strings.TrimSpace(credential.IDP)
	if typeName == "password" && (len(credential.Material) == 0 || strings.TrimSpace(credential.Algo) == "") {
		return "invalid_password"
	}
	if (typeName == "password" || idp == "") && len(credential.Material) > 0 && strings.TrimSpace(credential.Algo) != "" {
		return "password"
	}
	if typeName == "phone_otp" || idp == "phone" {
		return "phone"
	}
	switch typeName {
	case "oauth_wx_minip", "oauth_wx_open", "oauth_wx_scan", "oauth_wecom":
		return "oauth"
	default:
		return "unknown"
	}
}

func oauthProvider(credentialType string) string {
	switch strings.TrimSpace(credentialType) {
	case "oauth_wx_minip":
		return "wechat_minip"
	case "oauth_wecom":
		return "wecom"
	default:
		return "wechat_open"
	}
}

func authNIdentityFromLegacyAccount(
	account legacyAuthNAccount,
	key authNProviderKey,
	globalIdentifier string,
) plannedAuthNIdentity {
	createdAt := fallbackAuthNTime(account.CreatedAt)
	return plannedAuthNIdentity{
		ID:               account.ID,
		UserID:           account.UserID,
		Key:              key,
		GlobalIdentifier: globalIdentifier,
		Status:           legacyAuthNAccountStatus(account.Status),
		VerifiedAt:       sql.NullTime{Time: createdAt, Valid: true},
		LinkedAt:         createdAt,
		Profile:          cloneAuthNBytes(account.Profile),
		Meta: mergeAuthNLegacyMeta(account.Meta, map[string]any{
			"legacy_table":        "auth_accounts",
			"legacy_account_id":   account.ID,
			"legacy_account_type": account.Type,
		}),
		CreatedAt: createdAt,
		UpdatedAt: fallbackAuthNTime(account.UpdatedAt),
		DeletedAt: account.DeletedAt,
		CreatedBy: account.CreatedBy,
		UpdatedBy: account.UpdatedBy,
		DeletedBy: account.DeletedBy,
		Version:   nonZeroAuthN(account.Version, 1),
	}
}

func authNPhoneIdentityFromLegacy(
	account legacyAuthNAccount,
	credential legacyAuthNCredential,
	key authNProviderKey,
) plannedAuthNIdentity {
	createdAt := fallbackAuthNTime(credential.CreatedAt)
	status := legacyAuthNAccountStatus(account.Status)
	if status == "active" && legacyAuthNCredentialStatus(credential.Status) == "disabled" {
		status = "disabled"
	}
	return plannedAuthNIdentity{
		UserID:     account.UserID,
		Key:        key,
		Status:     status,
		VerifiedAt: sql.NullTime{Time: createdAt, Valid: true},
		LinkedAt:   createdAt,
		Meta: mergeAuthNLegacyMeta(credential.ParamsJSON, map[string]any{
			"legacy_table":           "auth_credentials_legacy",
			"legacy_credential_id":   credential.ID,
			"legacy_account_id":      credential.AccountID,
			"legacy_credential_type": credential.Type,
		}),
		CreatedAt: createdAt,
		UpdatedAt: fallbackAuthNTime(credential.UpdatedAt),
		DeletedAt: credential.DeletedAt,
		CreatedBy: credential.CreatedBy,
		UpdatedBy: credential.UpdatedBy,
		DeletedBy: credential.DeletedBy,
		Version:   nonZeroAuthN(credential.Version, 1),
	}
}

func authNPasswordFromLegacy(credential legacyAuthNCredential, loginIdentityID uint64) plannedAuthNPassword {
	return plannedAuthNPassword{
		ID:              credential.ID,
		LoginIdentityID: loginIdentityID,
		Material:        cloneAuthNBytes(credential.Material),
		Algo:            credential.Algo,
		ParamsJSON: mergeAuthNLegacyMeta(credential.ParamsJSON, map[string]any{
			"legacy_table":         "auth_credentials_legacy",
			"legacy_credential_id": credential.ID,
			"legacy_account_id":    credential.AccountID,
		}),
		Status:         legacyAuthNCredentialStatus(credential.Status),
		FailedAttempts: credential.FailedAttempts,
		LockedUntil:    credential.LockedUntil,
		LastSuccessAt:  credential.LastSuccessAt,
		LastFailureAt:  credential.LastFailureAt,
		CreatedAt:      fallbackAuthNTime(credential.CreatedAt),
		UpdatedAt:      fallbackAuthNTime(credential.UpdatedAt),
		DeletedAt:      credential.DeletedAt,
		CreatedBy:      credential.CreatedBy,
		UpdatedBy:      credential.UpdatedBy,
		DeletedBy:      credential.DeletedBy,
		Version:        nonZeroAuthN(credential.Version, 1),
	}
}

func allocateAuthNID(proposed uint64, used map[uint64]struct{}) uint64 {
	if proposed != 0 {
		if _, exists := used[proposed]; !exists {
			used[proposed] = struct{}{}
			return proposed
		}
	}
	for {
		id := uint64(idutil.GetIntID())
		if _, exists := used[id]; exists {
			continue
		}
		used[id] = struct{}{}
		return id
	}
}

func authNTableExists(ctx context.Context, q authNQueryer, table string) (bool, error) {
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND TABLE_TYPE = 'BASE TABLE'`, table).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect authn table state: %w", err)
	}
	return count == 1, nil
}

func authNColumnExists(ctx context.Context, q authNQueryer, table, column string) (bool, error) {
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect authn schema: %w", err)
	}
	return count == 1, nil
}

func mergeAuthNLegacyMeta(raw []byte, legacy map[string]any) []byte {
	merged := make(map[string]any, len(legacy)+1)
	if len(raw) > 0 {
		var existing map[string]any
		if err := json.Unmarshal(raw, &existing); err == nil && existing != nil {
			for key, value := range existing {
				merged[key] = value
			}
		} else {
			merged["legacy_meta_raw"] = string(raw)
		}
	}
	for key, value := range legacy {
		merged[key] = value
	}
	data, _ := json.Marshal(merged)
	return data
}

func legacyAuthNAccountStatus(status int) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "archived"
	case 3:
		return "deleted"
	default:
		return "disabled"
	}
}

func legacyAuthNCredentialStatus(status int) string {
	if status == 1 {
		return "enabled"
	}
	return "disabled"
}

func fallbackAuthNTime(value sql.NullTime) time.Time {
	if value.Valid && !value.Time.IsZero() {
		return value.Time
	}
	return time.Now()
}

func cloneAuthNBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func nonZeroAuthN(value, fallback uint64) uint64 {
	if value == 0 {
		return fallback
	}
	return value
}

func nullTimeValue(value sql.NullTime) any {
	if !value.Valid || value.Time.IsZero() {
		return nil
	}
	return value.Time
}

func nilIfEmptyBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func emptyStringToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
