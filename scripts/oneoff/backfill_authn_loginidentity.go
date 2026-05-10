package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
)

const (
	providerUsername    = "username"
	providerPhone       = "phone"
	providerWechatMinip = "wechat_minip"
	providerWecom       = "wecom"

	realmDefault = "default"
	realmGlobal  = "global"

	statusDisabled = "disabled"
	statusActive   = "active"
	statusArchived = "archived"
	statusDeleted  = "deleted"

	credentialStatusDisabled = "disabled"
	credentialStatusEnabled  = "enabled"
)

type options struct {
	dsn                   string
	apply                 bool
	legacyCredentialTable string
}

type summary struct {
	Mode                         string `json:"mode"`
	LegacyCredentialTable        string `json:"legacy_credential_table,omitempty"`
	LegacyAccounts               int    `json:"legacy_accounts"`
	LoginIdentitiesPreviewed     int    `json:"login_identities_previewed"`
	LoginIdentitiesApplied       int    `json:"login_identities_applied"`
	UnsupportedAccountsSkipped   int    `json:"unsupported_accounts_skipped"`
	LegacyCredentials            int    `json:"legacy_credentials"`
	PasswordCredentialsPreviewed int    `json:"password_credentials_previewed"`
	PasswordCredentialsApplied   int    `json:"password_credentials_applied"`
	PhoneIdentitiesPreviewed     int    `json:"phone_identities_previewed"`
	PhoneIdentitiesApplied       int    `json:"phone_identities_applied"`
	CredentialsSkipped           int    `json:"credentials_skipped"`
}

type legacyAccount struct {
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

type legacyCredential struct {
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

type providerKey struct {
	Provider         string
	Realm            string
	Identifier       string
	GlobalIdentifier string
}

type loginIdentityRow struct {
	ID         uint64
	UserID     uint64
	Key        providerKey
	Status     string
	VerifiedAt sql.NullTime
	LinkedAt   time.Time
	Profile    []byte
	Meta       []byte
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  sql.NullTime
	CreatedBy  uint64
	UpdatedBy  uint64
	DeletedBy  uint64
	Version    uint64
}

type passwordCredentialRow struct {
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

func main() {
	opts := parseOptions()
	if opts.dsn == "" {
		fmt.Fprintln(os.Stderr, "missing MySQL DSN; pass --dsn or set IAM_APISERVER_MYSQL_* / MYSQL_* env vars")
		os.Exit(2)
	}

	db, err := sql.Open("mysql", opts.dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open mysql: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping mysql: %v\n", err)
		os.Exit(1)
	}

	sum, err := run(ctx, db, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill failed: %v\n", err)
		os.Exit(1)
	}
	out, _ := json.MarshalIndent(sum, "", "  ")
	fmt.Println(string(out))
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.dsn, "dsn", dsnFromEnv(), "MySQL DSN, for example user:pass@tcp(127.0.0.1:3306)/iam?parseTime=true&loc=Local")
	flag.BoolVar(&opts.apply, "apply", false, "apply changes; default is dry-run")
	flag.StringVar(&opts.legacyCredentialTable, "legacy-credential-table", "", "legacy credential table name; default auto-detects auth_credentials_legacy then old-shaped auth_credentials")
	flag.Parse()
	return opts
}

func run(ctx context.Context, db *sql.DB, opts options) (summary, error) {
	sum := summary{Mode: "dry-run"}
	if opts.apply {
		sum.Mode = "apply"
	}

	if ok, err := tableExists(ctx, db, "auth_accounts"); err != nil {
		return sum, err
	} else if !ok {
		return sum, fmt.Errorf("legacy table auth_accounts does not exist")
	}

	credentialTable, err := resolveLegacyCredentialTable(ctx, db, opts.legacyCredentialTable)
	if err != nil {
		return sum, err
	}
	sum.LegacyCredentialTable = credentialTable

	accounts, err := loadLegacyAccounts(ctx, db)
	if err != nil {
		return sum, err
	}
	sum.LegacyAccounts = len(accounts)

	accountIdentities := make(map[uint64]loginIdentityRow, len(accounts))
	for _, acc := range accounts {
		row, ok := accountLoginIdentity(acc)
		if !ok {
			sum.UnsupportedAccountsSkipped++
			continue
		}
		accountIdentities[acc.ID] = row
		sum.LoginIdentitiesPreviewed++
	}

	var credentials []legacyCredential
	if credentialTable != "" {
		credentials, err = loadLegacyCredentials(ctx, db, credentialTable)
		if err != nil {
			return sum, err
		}
		sum.LegacyCredentials = len(credentials)
		for _, cred := range credentials {
			switch {
			case isPasswordCredential(cred):
				if _, ok := accountIdentities[cred.AccountID]; ok {
					sum.PasswordCredentialsPreviewed++
				} else {
					sum.CredentialsSkipped++
				}
			case isPhoneCredential(cred):
				if cred.IDPIdentifier != "" {
					if _, ok := accounts[cred.AccountID]; !ok {
						sum.CredentialsSkipped++
						continue
					}
					sum.PhoneIdentitiesPreviewed++
				} else {
					sum.CredentialsSkipped++
				}
			default:
				sum.CredentialsSkipped++
			}
		}
	}

	if !opts.apply {
		return sum, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return sum, err
	}
	defer tx.Rollback()

	resolvedIdentityIDs := make(map[uint64]uint64, len(accountIdentities))
	for accountID, row := range accountIdentities {
		id, err := upsertLoginIdentity(ctx, tx, row)
		if err != nil {
			return sum, fmt.Errorf("upsert login identity for legacy account %d: %w", accountID, err)
		}
		resolvedIdentityIDs[accountID] = id
		sum.LoginIdentitiesApplied++
	}

	for _, cred := range credentials {
		switch {
		case isPasswordCredential(cred):
			loginIdentityID, ok := resolvedIdentityIDs[cred.AccountID]
			if !ok {
				continue
			}
			row := passwordCredentialFromLegacy(cred, loginIdentityID)
			if err := upsertPasswordCredential(ctx, tx, row); err != nil {
				return sum, fmt.Errorf("upsert password credential %d: %w", cred.ID, err)
			}
			sum.PasswordCredentialsApplied++
		case isPhoneCredential(cred):
			acc, ok := accounts[cred.AccountID]
			if !ok || strings.TrimSpace(cred.IDPIdentifier) == "" {
				continue
			}
			row := phoneLoginIdentity(acc, cred)
			if _, err := upsertLoginIdentity(ctx, tx, row); err != nil {
				return sum, fmt.Errorf("upsert phone login identity from legacy credential %d: %w", cred.ID, err)
			}
			sum.PhoneIdentitiesApplied++
		}
	}

	if err := tx.Commit(); err != nil {
		return sum, err
	}
	return sum, nil
}

func accountLoginIdentity(acc legacyAccount) (loginIdentityRow, bool) {
	key, ok := accountProviderKey(acc)
	if !ok || !key.isValid() {
		return loginIdentityRow{}, false
	}
	now := fallbackTime(acc.CreatedAt)
	return loginIdentityRow{
		ID:         acc.ID,
		UserID:     acc.UserID,
		Key:        key,
		Status:     accountStatus(acc.Status),
		VerifiedAt: nullableTime(now),
		LinkedAt:   now,
		Profile:    cloneBytes(acc.Profile),
		Meta: mergeLegacyMeta(acc.Meta, map[string]any{
			"legacy_table":        "auth_accounts",
			"legacy_account_id":   acc.ID,
			"legacy_account_type": acc.Type,
		}),
		CreatedAt: now,
		UpdatedAt: fallbackTime(acc.UpdatedAt),
		DeletedAt: acc.DeletedAt,
		CreatedBy: acc.CreatedBy,
		UpdatedBy: acc.UpdatedBy,
		DeletedBy: acc.DeletedBy,
		Version:   nonZero(acc.Version, 1),
	}, true
}

func phoneLoginIdentity(acc legacyAccount, cred legacyCredential) loginIdentityRow {
	now := fallbackTime(cred.CreatedAt)
	status := accountStatus(acc.Status)
	if status == statusActive && credentialStatus(cred.Status) == credentialStatusDisabled {
		status = statusDisabled
	}
	return loginIdentityRow{
		ID:     uint64(idutil.GetIntID()),
		UserID: acc.UserID,
		Key: providerKey{
			Provider:   providerPhone,
			Realm:      realmGlobal,
			Identifier: strings.TrimSpace(cred.IDPIdentifier),
		},
		Status:     status,
		VerifiedAt: nullableTime(now),
		LinkedAt:   now,
		Meta: mergeLegacyMeta(cred.ParamsJSON, map[string]any{
			"legacy_table":           "auth_credentials",
			"legacy_credential_id":   cred.ID,
			"legacy_account_id":      cred.AccountID,
			"legacy_credential_type": cred.Type,
		}),
		CreatedAt: now,
		UpdatedAt: fallbackTime(cred.UpdatedAt),
		DeletedAt: cred.DeletedAt,
		CreatedBy: cred.CreatedBy,
		UpdatedBy: cred.UpdatedBy,
		DeletedBy: cred.DeletedBy,
		Version:   nonZero(cred.Version, 1),
	}
}

func passwordCredentialFromLegacy(cred legacyCredential, loginIdentityID uint64) passwordCredentialRow {
	return passwordCredentialRow{
		ID:              cred.ID,
		LoginIdentityID: loginIdentityID,
		Material:        cloneBytes(cred.Material),
		Algo:            cred.Algo,
		ParamsJSON: mergeLegacyMeta(cred.ParamsJSON, map[string]any{
			"legacy_table":         "auth_credentials",
			"legacy_credential_id": cred.ID,
			"legacy_account_id":    cred.AccountID,
		}),
		Status:         credentialStatus(cred.Status),
		FailedAttempts: cred.FailedAttempts,
		LockedUntil:    cred.LockedUntil,
		LastSuccessAt:  cred.LastSuccessAt,
		LastFailureAt:  cred.LastFailureAt,
		CreatedAt:      fallbackTime(cred.CreatedAt),
		UpdatedAt:      fallbackTime(cred.UpdatedAt),
		DeletedAt:      cred.DeletedAt,
		CreatedBy:      cred.CreatedBy,
		UpdatedBy:      cred.UpdatedBy,
		DeletedBy:      cred.DeletedBy,
		Version:        nonZero(cred.Version, 1),
	}
}

func accountProviderKey(acc legacyAccount) (providerKey, bool) {
	accountType := strings.TrimSpace(acc.Type)
	switch accountType {
	case "opera":
		realm := realmDefault
		if acc.ScopedTenantID != 0 {
			realm = strconv.FormatUint(acc.ScopedTenantID, 10)
		}
		return providerKey{Provider: providerUsername, Realm: realm, Identifier: strings.TrimSpace(acc.ExternalID)}, true
	case "mock-consumer":
		return providerKey{Provider: providerUsername, Realm: realmDefault, Identifier: strings.TrimSpace(acc.ExternalID)}, true
	case "wc-minip":
		return providerKey{
			Provider:         providerWechatMinip,
			Realm:            strings.TrimSpace(acc.AppID),
			Identifier:       strings.TrimSpace(acc.ExternalID),
			GlobalIdentifier: strings.TrimSpace(acc.UniqueID),
		}, true
	case "wc-com":
		return providerKey{
			Provider:         providerWecom,
			Realm:            strings.TrimSpace(acc.AppID),
			Identifier:       strings.TrimSpace(acc.ExternalID),
			GlobalIdentifier: strings.TrimSpace(acc.UniqueID),
		}, true
	default:
		return providerKey{}, false
	}
}

func (k providerKey) isValid() bool {
	return strings.TrimSpace(k.Provider) != "" &&
		strings.TrimSpace(k.Realm) != "" &&
		strings.TrimSpace(k.Identifier) != ""
}

func isPasswordCredential(cred legacyCredential) bool {
	if strings.TrimSpace(cred.Type) == "password" {
		return len(cred.Material) > 0 && strings.TrimSpace(cred.Algo) != ""
	}
	return strings.TrimSpace(cred.IDP) == "" && len(cred.Material) > 0 && strings.TrimSpace(cred.Algo) != ""
}

func isPhoneCredential(cred legacyCredential) bool {
	return strings.TrimSpace(cred.Type) == "phone_otp" || strings.TrimSpace(cred.IDP) == "phone"
}

func accountStatus(status int) string {
	switch status {
	case 1:
		return statusActive
	case 2:
		return statusArchived
	case 3:
		return statusDeleted
	default:
		return statusDisabled
	}
}

func credentialStatus(status int) string {
	if status == 1 {
		return credentialStatusEnabled
	}
	return credentialStatusDisabled
}

func loadLegacyAccounts(ctx context.Context, db *sql.DB) (map[uint64]legacyAccount, error) {
	scopedExpr := "0 AS scoped_tenant_id"
	hasScoped, err := columnExists(ctx, db, "auth_accounts", "scoped_tenant_id")
	if err != nil {
		return nil, err
	}
	if hasScoped {
		scopedExpr = "`scoped_tenant_id`"
	}

	query := fmt.Sprintf(`SELECT id, user_id, type, COALESCE(app_id, ''), external_id,
COALESCE(unique_id, ''), %s, profile, meta, status, created_at, updated_at, deleted_at,
created_by, updated_by, deleted_by, version
FROM auth_accounts`, scopedExpr)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uint64]legacyAccount)
	for rows.Next() {
		var acc legacyAccount
		if err := rows.Scan(
			&acc.ID,
			&acc.UserID,
			&acc.Type,
			&acc.AppID,
			&acc.ExternalID,
			&acc.UniqueID,
			&acc.ScopedTenantID,
			&acc.Profile,
			&acc.Meta,
			&acc.Status,
			&acc.CreatedAt,
			&acc.UpdatedAt,
			&acc.DeletedAt,
			&acc.CreatedBy,
			&acc.UpdatedBy,
			&acc.DeletedBy,
			&acc.Version,
		); err != nil {
			return nil, err
		}
		result[acc.ID] = acc
	}
	return result, rows.Err()
}

func loadLegacyCredentials(ctx context.Context, db *sql.DB, table string) ([]legacyCredential, error) {
	if !safeIdentifier(table) {
		return nil, fmt.Errorf("unsafe legacy credential table name: %s", table)
	}
	query := fmt.Sprintf(`SELECT id, account_id, type, COALESCE(idp, ''), idp_identifier,
COALESCE(app_id, ''), material, COALESCE(algo, ''), params_json, status, failed_attempts,
locked_until, last_success_at, last_failure_at, created_at, updated_at, deleted_at,
created_by, updated_by, deleted_by, version
FROM %s`, table)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []legacyCredential
	for rows.Next() {
		var cred legacyCredential
		if err := rows.Scan(
			&cred.ID,
			&cred.AccountID,
			&cred.Type,
			&cred.IDP,
			&cred.IDPIdentifier,
			&cred.AppID,
			&cred.Material,
			&cred.Algo,
			&cred.ParamsJSON,
			&cred.Status,
			&cred.FailedAttempts,
			&cred.LockedUntil,
			&cred.LastSuccessAt,
			&cred.LastFailureAt,
			&cred.CreatedAt,
			&cred.UpdatedAt,
			&cred.DeletedAt,
			&cred.CreatedBy,
			&cred.UpdatedBy,
			&cred.DeletedBy,
			&cred.Version,
		); err != nil {
			return nil, err
		}
		result = append(result, cred)
	}
	return result, rows.Err()
}

func upsertLoginIdentity(ctx context.Context, tx *sql.Tx, row loginIdentityRow) (uint64, error) {
	id, ok, err := findLoginIdentityIDByKey(ctx, tx, row.Key)
	if err != nil {
		return 0, err
	}
	if ok {
		row.ID = id
	} else {
		id, err = availableID(ctx, tx, "auth_login_identities", row.ID)
		if err != nil {
			return 0, err
		}
		row.ID = id
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO auth_login_identities
(id, user_id, provider, realm, identifier, global_identifier, status, verified_at, linked_at,
 profile_json, meta_json, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version)
VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
 user_id = VALUES(user_id),
 global_identifier = VALUES(global_identifier),
 status = VALUES(status),
 verified_at = VALUES(verified_at),
 profile_json = VALUES(profile_json),
 meta_json = VALUES(meta_json),
 updated_at = VALUES(updated_at),
 deleted_at = VALUES(deleted_at),
 updated_by = VALUES(updated_by),
 deleted_by = VALUES(deleted_by)`,
		row.ID,
		row.UserID,
		row.Key.Provider,
		row.Key.Realm,
		row.Key.Identifier,
		row.Key.GlobalIdentifier,
		row.Status,
		nullTimeArg(row.VerifiedAt),
		row.LinkedAt,
		nilIfEmpty(row.Profile),
		nilIfEmpty(row.Meta),
		row.CreatedAt,
		row.UpdatedAt,
		nullTimeArg(row.DeletedAt),
		row.CreatedBy,
		row.UpdatedBy,
		row.DeletedBy,
		row.Version,
	)
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

func upsertPasswordCredential(ctx context.Context, tx *sql.Tx, row passwordCredentialRow) error {
	id, ok, err := findPasswordCredentialID(ctx, tx, row.LoginIdentityID)
	if err != nil {
		return err
	}
	if ok {
		row.ID = id
	} else {
		id, err = availableID(ctx, tx, "auth_credentials", row.ID)
		if err != nil {
			return err
		}
		row.ID = id
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO auth_credentials
(id, login_identity_id, type, material, algo, params_json, status, failed_attempts, locked_until,
 last_success_at, last_failure_at, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by, version)
VALUES (?, ?, 'password', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
 login_identity_id = VALUES(login_identity_id),
 material = VALUES(material),
 algo = VALUES(algo),
 params_json = VALUES(params_json),
 status = VALUES(status),
 failed_attempts = VALUES(failed_attempts),
 locked_until = VALUES(locked_until),
 last_success_at = VALUES(last_success_at),
 last_failure_at = VALUES(last_failure_at),
 updated_at = VALUES(updated_at),
 deleted_at = VALUES(deleted_at),
 updated_by = VALUES(updated_by),
 deleted_by = VALUES(deleted_by)`,
		row.ID,
		row.LoginIdentityID,
		nilIfEmpty(row.Material),
		emptyToNil(row.Algo),
		nilIfEmpty(row.ParamsJSON),
		row.Status,
		row.FailedAttempts,
		nullTimeArg(row.LockedUntil),
		nullTimeArg(row.LastSuccessAt),
		nullTimeArg(row.LastFailureAt),
		row.CreatedAt,
		row.UpdatedAt,
		nullTimeArg(row.DeletedAt),
		row.CreatedBy,
		row.UpdatedBy,
		row.DeletedBy,
		row.Version,
	)
	return err
}

func findLoginIdentityIDByKey(ctx context.Context, tx *sql.Tx, key providerKey) (uint64, bool, error) {
	var id uint64
	err := tx.QueryRowContext(ctx, `SELECT id FROM auth_login_identities
WHERE provider = ? AND realm = ? AND identifier = ? LIMIT 1`,
		key.Provider,
		key.Realm,
		key.Identifier,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func findPasswordCredentialID(ctx context.Context, tx *sql.Tx, loginIdentityID uint64) (uint64, bool, error) {
	var id uint64
	err := tx.QueryRowContext(ctx, `SELECT id FROM auth_credentials
WHERE login_identity_id = ? AND type = 'password' LIMIT 1`, loginIdentityID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func availableID(ctx context.Context, tx *sql.Tx, table string, proposed uint64) (uint64, error) {
	if !safeIdentifier(table) {
		return 0, fmt.Errorf("unsafe table name: %s", table)
	}
	id := proposed
	if id == 0 {
		id = uint64(idutil.GetIntID())
	}
	for {
		exists, err := idExists(ctx, tx, table, id)
		if err != nil {
			return 0, err
		}
		if !exists {
			return id, nil
		}
		id = uint64(idutil.GetIntID())
	}
}

func idExists(ctx context.Context, tx *sql.Tx, table string, id uint64) (bool, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", table)
	var count int
	if err := tx.QueryRowContext(ctx, query, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func resolveLegacyCredentialTable(ctx context.Context, db *sql.DB, explicit string) (string, error) {
	if explicit != "" {
		if !safeIdentifier(explicit) {
			return "", fmt.Errorf("unsafe legacy credential table name: %s", explicit)
		}
		ok, err := tableExists(ctx, db, explicit)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("explicit legacy credential table does not exist: %s", explicit)
		}
		return explicit, nil
	}
	if ok, err := tableExists(ctx, db, "auth_credentials_legacy"); err != nil {
		return "", err
	} else if ok {
		return "auth_credentials_legacy", nil
	}
	if ok, err := columnExists(ctx, db, "auth_credentials", "account_id"); err != nil {
		return "", err
	} else if ok {
		return "auth_credentials", nil
	}
	return "", nil
}

func tableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*)
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table).Scan(&count)
	return count > 0, err
}

func columnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).Scan(&count)
	return count > 0, err
}

func mergeLegacyMeta(raw []byte, legacy map[string]any) []byte {
	merged := make(map[string]any, len(legacy)+1)
	if len(raw) > 0 {
		var existing map[string]any
		if err := json.Unmarshal(raw, &existing); err == nil && existing != nil {
			for k, v := range existing {
				merged[k] = v
			}
		} else {
			merged["legacy_meta_raw"] = string(raw)
		}
	}
	for k, v := range legacy {
		merged[k] = v
	}
	data, _ := json.Marshal(merged)
	return data
}

func dsnFromEnv() string {
	if dsn := os.Getenv("IAM_MYSQL_DSN"); dsn != "" {
		return dsn
	}
	if dsn := os.Getenv("MYSQL_DSN"); dsn != "" {
		return dsn
	}
	host := firstEnv("IAM_APISERVER_MYSQL_HOST", "MYSQL_HOST")
	user := firstEnv("IAM_APISERVER_MYSQL_USERNAME", "IAM_APISERVER_MYSQL_USER", "MYSQL_USER", "MYSQL_USERNAME")
	pass := firstEnv("IAM_APISERVER_MYSQL_PASSWORD", "MYSQL_PASSWORD", "MYSQL_PASS")
	dbName := firstEnv("IAM_APISERVER_MYSQL_DATABASE", "IAM_APISERVER_MYSQL_DBNAME", "MYSQL_DATABASE", "MYSQL_DBNAME")
	if host == "" || user == "" || dbName == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		port := firstEnv("IAM_APISERVER_MYSQL_PORT", "MYSQL_PORT")
		if port == "" {
			port = "3306"
		}
		host = net.JoinHostPort(host, port)
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true", user, pass, host, dbName)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func fallbackTime(t sql.NullTime) time.Time {
	if t.Valid && !t.Time.IsZero() {
		return t.Time
	}
	return time.Now()
}

func nullableTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}

func nullTimeArg(t sql.NullTime) any {
	if !t.Valid || t.Time.IsZero() {
		return nil
	}
	return t.Time
}

func nilIfEmpty(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	return data
}

func emptyToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func nonZero(value, fallback uint64) uint64 {
	if value == 0 {
		return fallback
	}
	return value
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func safeIdentifier(value string) bool {
	return identifierPattern.MatchString(value)
}
