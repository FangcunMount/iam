package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gorm.io/gorm"
)

const chunkSize = 1000
const rowLimit = 1 << 20

// Only these six tables can be physically changed. Roles, policy versions,
// durable messages and session-revocation records are never archived for purge.
var tableOrder = []string{"authz_assignments", "auth_credentials", "auth_login_identities", "profile_links", "profiles", "users"}

type column struct {
	Name      string `json:"name"`
	Type      string `json:"sql_type"`
	Generated bool   `json:"generated"`
}
type cell struct {
	Null bool   `json:"null"`
	Raw  []byte `json:"raw"`
}
type row struct {
	Values []cell `json:"values"`
}
type tableImage struct {
	Name    string   `json:"table"`
	DDL     string   `json:"create_table"`
	Columns []column `json:"columns"`
	Rows    []row    `json:"rows"`
}
type archive struct {
	Format    int          `json:"format_version"`
	Kind      string       `json:"kind"`
	Operation string       `json:"operation_id"`
	Source    string       `json:"compiled_source"`
	GroupSHA  string       `json:"group_manifest_sha256"`
	Identity  identity     `json:"identity"`
	Tables    []tableImage `json:"tables"`
	Protected []tableImage `json:"protected"`
}
type sqlReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func quoted(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }
func slots(n int) string     { return strings.TrimRight(strings.Repeat("?,", n), ",") }
func values(ids []string) []any {
	a := make([]any, len(ids))
	for i, s := range ids {
		a[i] = s
	}
	return a
}
func scopePredicate(table string, s iamScope) (string, []any) {
	ids := s.Users
	col := "id"
	switch table {
	case "profiles":
		ids = s.Profiles
	case "auth_login_identities":
		ids = s.LoginIdentities
	case "auth_credentials":
		col = "login_identity_id"
		ids = s.LoginIdentities
	case "authz_assignments":
		col = "subject_id"
	case "profile_links":
		p1, a1 := idPredicate("user_id", s.Users)
		p2, a2 := idPredicate("profile_id", s.Profiles)
		return "(" + p1 + " OR " + p2 + ")", append(a1, a2...)
	}
	p, a := idPredicate(col, ids)
	if table == "authz_assignments" {
		p = "subject_type='user' AND " + p
	}
	return p, a
}
func idPredicate(col string, ids []string) (string, []any) {
	if len(ids) == 0 {
		return "1=0", nil
	}
	return quoted(col) + " IN (" + slots(len(ids)) + ")", values(ids)
}
func columns(ctx context.Context, q sqlReader, table string) (out []column, retErr error) {
	r, e := q.QueryContext(ctx, "SELECT COLUMN_NAME,COLUMN_TYPE,EXTRA FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", table)
	if e != nil {
		return nil, e
	}
	defer func() { retErr = errors.Join(retErr, r.Close()) }()
	out = []column{}
	for r.Next() {
		var c column
		var extra string
		if e = r.Scan(&c.Name, &c.Type, &extra); e != nil {
			return nil, e
		}
		c.Generated = generatedColumn(extra)
		out = append(out, c)
	}
	if e = r.Err(); e != nil {
		return nil, e
	}
	if len(out) == 0 || out[0].Name != "id" {
		return nil, errors.New("fixed table schema missing id")
	}
	return out, nil
}
func generatedColumn(extra string) bool {
	return strings.Contains(extra, "VIRTUAL GENERATED") || strings.Contains(extra, "STORED GENERATED")
}
func readImage(ctx context.Context, q sqlReader, table string, cols []column, pred string, args []any) (tableImage, error) {
	return readImageMode(ctx, q, table, cols, pred, args, false)
}
func readImageMode(ctx context.Context, q sqlReader, table string, cols []column, pred string, args []any, lock bool) (out tableImage, retErr error) {
	out = tableImage{Name: table, Columns: cols, Rows: []row{}}
	selects := make([]string, len(cols))
	for i, c := range cols {
		selects[i] = "CAST(" + quoted(c.Name) + " AS BINARY)"
	}
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	r, e := q.QueryContext(ctx, "SELECT "+strings.Join(selects, ",")+" FROM "+quoted(table)+" WHERE "+pred+" ORDER BY id"+suffix, args...)
	if e != nil {
		return out, e
	}
	defer func() { retErr = errors.Join(retErr, r.Close()) }()
	for r.Next() {
		raw := make([]sql.RawBytes, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if e = r.Scan(ptrs...); e != nil {
			return out, e
		}
		v := row{Values: make([]cell, len(cols))}
		size := 0
		for i, b := range raw {
			v.Values[i] = cell{Null: b == nil, Raw: append([]byte{}, b...)}
			size += len(b)
		}
		if size > rowLimit {
			return out, errors.New("row exceeds bounded restore budget")
		}
		out.Rows = append(out.Rows, v)
		if len(out.Rows) > 1_000_000 {
			return out, errors.New("archive row bound exceeded")
		}
	}
	return out, r.Err()
}
func readScope(ctx context.Context, q sqlReader, s iamScope) ([]tableImage, error) {
	out := make([]tableImage, 0, len(tableOrder))
	for _, name := range tableOrder {
		cols, e := columns(ctx, q, name)
		if e != nil {
			return nil, e
		}
		im := tableImage{Name: name, Columns: cols, Rows: []row{}}
		ddlRows, er := q.QueryContext(ctx, "SHOW CREATE TABLE "+quoted(name))
		if er != nil {
			return nil, er
		}
		var actualName string
		if !ddlRows.Next() {
			_ = ddlRows.Close()
			return nil, errors.New("create table missing")
		}
		er = ddlRows.Scan(&actualName, &im.DDL)
		extra := ddlRows.Next()
		er = errors.Join(er, ddlRows.Err(), ddlRows.Close())
		if er != nil {
			return nil, er
		}
		if actualName != name || extra || im.DDL == "" {
			return nil, errors.New("create table shape rejected")
		}
		seen := map[string]row{}
		for _, batch := range scopeBatches(s) {
			p, a := scopePredicate(name, batch)
			part, er := readImage(ctx, q, name, cols, p, a)
			if er != nil {
				return nil, er
			}
			for _, r := range part.Rows {
				id := string(r.Values[0].Raw)
				if old, ok := seen[id]; ok {
					if !reflect.DeepEqual(old, r) {
						return nil, errors.New("duplicate page row changed")
					}
					continue
				}
				seen[id] = r
				im.Rows = append(im.Rows, r)
				if len(im.Rows) > 1_000_000 {
					return nil, errors.New("cohort row bound exceeded")
				}
			}
		}
		out = append(out, im)
	}
	return out, nil
}
func scopeBatches(s iamScope) []iamScope {
	n := max(len(s.Users), len(s.Profiles), len(s.LoginIdentities))
	out := []iamScope{}
	part := func(ids []string, i int) []string {
		if i >= len(ids) {
			return []string{}
		}
		return ids[i:min(i+chunkSize, len(ids))]
	}
	for i := 0; i < n; i += chunkSize {
		out = append(out, iamScope{part(s.Users, i), part(s.Profiles, i), part(s.LoginIdentities, i)})
	}
	if len(out) == 0 {
		out = append(out, iamScope{[]string{}, []string{}, []string{}})
	}
	return out
}
func field(im tableImage, r row, name string) cell {
	for i, c := range im.Columns {
		if c.Name == name {
			return r.Values[i]
		}
	}
	return cell{Null: true}
}
func lookup(images []tableImage, name string) tableImage {
	for _, im := range images {
		if im.Name == name {
			return im
		}
	}
	return tableImage{}
}
func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func validateClosure(images []tableImage, s iamScope) error {
	users, profiles, logins := idSet(s.Users), idSet(s.Profiles), idSet(s.LoginIdentities)
	for name, n := range map[string]int{"users": len(s.Users), "profiles": len(s.Profiles), "auth_login_identities": len(s.LoginIdentities)} {
		if len(lookup(images, name).Rows) != n {
			return errors.New("manifest identity missing or broadened")
		}
	}
	for name, ids := range map[string][]string{"users": s.Users, "profiles": s.Profiles, "auth_login_identities": s.LoginIdentities} {
		set := idSet(ids)
		for _, id := range tableIDs(lookup(images, name)) {
			if !set[id] {
				return errors.New("archive typed root differs from manifest")
			}
		}
	}
	for _, im := range images {
		for _, r := range im.Rows {
			switch im.Name {
			case "auth_credentials":
				if !logins[string(field(im, r, "login_identity_id").Raw)] {
					return errors.New("credential outside selected login identities")
				}
			case "authz_assignments":
				if string(field(im, r, "subject_type").Raw) != "user" || !users[string(field(im, r, "subject_id").Raw)] {
					return errors.New("assignment outside selected users")
				}
			case "profile_links":
				if !users[string(field(im, r, "user_id").Raw)] || !profiles[string(field(im, r, "profile_id").Raw)] {
					return errors.New("shared profile relationship outside cohort")
				}
			case "auth_login_identities":
				if !users[string(field(im, r, "user_id").Raw)] {
					return errors.New("retained user owns selected login identity")
				}
			}
		}
	}
	return nil
}
func idSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
func rowChunks(rows []row) [][]row {
	out := [][]row{}
	for start := 0; start < len(rows); {
		end := start
		bytes := 0
		for end < len(rows) && end-start < chunkSize {
			size := len(encode(rows[end]))
			if end > start && bytes+size > 8<<20 {
				break
			}
			bytes += size
			end++
		}
		out = append(out, rows[start:end])
		start = end
	}
	return out
}
func validateAssignmentOrgs(images []tableImage, orgs []string) error {
	im := lookup(images, "authz_assignments")
	for _, r := range im.Rows {
		if !contains(orgs, string(field(im, r, "org_id").Raw)) {
			return errors.New("cross-organization assignment outside confirmed cohort")
		}
	}
	return nil
}

// Discover missing LoginIdentity selections, including soft-deleted rows. A
// selected user is never removed while an unselected credential root remains.
func validateExternal(ctx context.Context, q sqlReader, s iamScope) error {
	allowedLogins := map[string]bool{}
	for _, id := range s.LoginIdentities {
		allowedLogins[id] = true
	}
	for _, batch := range scopeBatches(s) {
		p, a := idPredicate("user_id", batch.Users)
		r, e := q.QueryContext(ctx, "SELECT id FROM auth_login_identities WHERE "+p, a...)
		if e != nil {
			return e
		}
		for r.Next() {
			var id string
			if e = r.Scan(&id); e != nil {
				_ = r.Close()
				return e
			}
			if !allowedLogins[id] {
				_ = r.Close()
				return errors.New("unselected login identity references cohort")
			}
		}
		e = errors.Join(r.Err(), r.Close())
		if e != nil {
			return e
		}
	}
	// A new reference column/table is a review boundary, not implicit permission.
	r, e := q.QueryContext(ctx, "SELECT TABLE_NAME,COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND COLUMN_NAME IN ('user_id','profile_id','login_identity_id') ORDER BY TABLE_NAME,COLUMN_NAME")
	if e != nil {
		return e
	}
	refs := [][2]string{}
	for r.Next() {
		var t, c string
		if e = r.Scan(&t, &c); e != nil {
			_ = r.Close()
			return e
		}
		refs = append(refs, [2]string{t, c})
	}
	e = errors.Join(r.Err(), r.Close())
	if e != nil {
		return e
	}
	allowed := map[string]bool{"profile_links": true, "auth_login_identities": true, "auth_credentials": true, "identity_session_revocation_outbox": true}
	for _, ref := range refs {
		if allowed[ref[0]] {
			continue
		}
		ids := s.Users
		if ref[1] == "profile_id" {
			ids = s.Profiles
		} else if ref[1] == "login_identity_id" {
			ids = s.LoginIdentities
		}
		for start := 0; start < len(ids); start += chunkSize {
			p, a := idPredicate(ref[1], ids[start:min(start+chunkSize, len(ids))])
			rr, e := q.QueryContext(ctx, "SELECT COUNT(*) FROM "+quoted(ref[0])+" WHERE "+p, a...)
			if e != nil {
				return e
			}
			var n int
			if !rr.Next() {
				_ = rr.Close()
				return errors.New("reference count missing")
			}
			e = errors.Join(rr.Scan(&n), rr.Err(), rr.Close())
			if e != nil {
				return e
			}
			if n != 0 {
				return fmt.Errorf("unreviewed retained reference: %s.%s", ref[0], ref[1])
			}
		}
	}
	return nil
}
func readIdentity(ctx context.Context, q sqlReader) (out identity, retErr error) {
	r, e := q.QueryContext(ctx, "SELECT DATABASE(),@@server_uuid")
	if e != nil {
		return out, e
	}
	if !r.Next() {
		_ = r.Close()
		return out, errors.New("database identity missing")
	}
	e = errors.Join(r.Scan(&out.Database, &out.ServerUUID), r.Err(), r.Close())
	if e != nil {
		return out, e
	}
	r, e = q.QueryContext(ctx, "SELECT version,dirty FROM schema_migrations")
	if e != nil {
		return out, e
	}
	defer func() { retErr = errors.Join(retErr, r.Close()) }()
	if !r.Next() {
		return out, errors.New("migration state missing")
	}
	if e = r.Scan(&out.Version, &out.Dirty); e != nil {
		return out, e
	}
	if r.Next() {
		return out, errors.New("multiple migration rows")
	}
	if e = r.Err(); e != nil {
		return out, e
	}
	if out.Database == "" || out.ServerUUID == "" || out.Version != 40 || out.Dirty {
		return out, errors.New("IAM requires known clean head 40")
	}
	return out, nil
}
func schemaEqual(a, b []tableImage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].DDL != b[i].DDL || !reflect.DeepEqual(a[i].Columns, b[i].Columns) {
			return false
		}
	}
	return true
}
func imageMap(images []tableImage) map[string]row {
	out := map[string]row{}
	for _, im := range images {
		for _, r := range im.Rows {
			out[im.Name+":"+string(field(im, r, "id").Raw)] = r
		}
	}
	return out
}
func imageRowsEqual(a, b []tableImage) bool {
	return schemaEqual(a, b) && reflect.DeepEqual(imageMap(a), imageMap(b))
}
func tableIDs(im tableImage) []string {
	out := make([]string, len(im.Rows))
	for i, r := range im.Rows {
		out[i] = string(field(im, r, "id").Raw)
	}
	return out
}
func currentRows(ctx context.Context, q sqlReader, im tableImage, rows []row) (tableImage, error) {
	return currentRowsMode(ctx, q, im, rows, false)
}
func currentRowsLocked(ctx context.Context, q sqlReader, im tableImage, rows []row) (tableImage, error) {
	return currentRowsMode(ctx, q, im, rows, true)
}
func currentRowsMode(ctx context.Context, q sqlReader, im tableImage, rows []row, lock bool) (tableImage, error) {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = string(field(im, r, "id").Raw)
	}
	p, a := idPredicate("id", ids)
	out, err := readImageMode(ctx, q, im.Name, im.Columns, p, a, lock)
	out.DDL = im.DDL
	return out, err
}
func sortedRows(rows []row) []row {
	out := append([]row{}, rows...)
	sort.Slice(out, func(i, j int) bool { return string(out[i].Values[0].Raw) < string(out[j].Values[0].Raw) })
	return out
}
func rowsEqual(a, b []row) bool { return reflect.DeepEqual(sortedRows(a), sortedRows(b)) }
func deleteRows(ctx context.Context, tx *gorm.DB, im tableImage, rows []row) error {
	if len(rows) > chunkSize {
		return errors.New("SQL chunk exceeds 1000 PK")
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = string(field(im, r, "id").Raw)
	}
	if len(ids) == 0 {
		return nil
	}
	p, a := idPredicate("id", ids)
	r := tx.WithContext(ctx).Exec("DELETE FROM "+quoted(im.Name)+" WHERE "+p, a...)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != int64(len(ids)) {
		return errors.New("delete count changed")
	}
	return nil
}
func insertRow(ctx context.Context, tx *gorm.DB, im tableImage, r row) error {
	cols := []string{}
	args := []any{}
	for i, c := range im.Columns {
		if c.Generated {
			continue
		}
		cols = append(cols, quoted(c.Name))
		var v any
		if !r.Values[i].Null {
			v = r.Values[i].Raw
		}
		args = append(args, v)
	}
	// GORM's raw-expression builder expands a slice in the first VALUES(?)
	// position, including []byte. Bind the captured cells directly through the
	// same transaction's database/sql connection so NULL, empty and binary cells
	// remain exactly one driver parameter each.
	result, err := tx.Statement.ConnPool.ExecContext(ctx, "INSERT INTO "+quoted(im.Name)+" ("+strings.Join(cols, ",")+") VALUES ("+slots(len(cols))+")", args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("restore insert count changed")
	}
	return nil
}
