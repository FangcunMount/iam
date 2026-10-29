package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Exercise the actual GORM initialization and insertRow call with a recording
// driver boundary. No server, credentials or SQL emulation is involved.
type insertRecorder struct {
	query    string
	args     []any
	affected int64
}

func (r *insertRecorder) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (r *insertRecorder) ExecContext(_ context.Context, q string, a ...any) (sql.Result, error) {
	r.query = q
	r.args = a
	return driver.RowsAffected(r.affected), nil
}
func (r *insertRecorder) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (r *insertRecorder) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("unexpected query row")
}
func TestRestoreBindsBinaryCellsAsSingleDriverValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value cell
	}{{"binary", cell{Raw: []byte{0, 255, 42}}}, {"empty", cell{Raw: []byte{}}}, {"null", cell{Null: true, Raw: []byte{}}}} {
		t.Run(tc.name, func(t *testing.T) {
			r := &insertRecorder{affected: 1}
			db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: r, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			im := tableImage{Name: "users", Columns: []column{{Name: "id"}, {Name: "value"}, {Name: "computed", Generated: true}}}
			original := row{Values: []cell{{Raw: []byte("810000101")}, tc.value, {Raw: []byte("1")}}}
			if err = insertRow(context.Background(), db, im, original); err != nil {
				t.Fatal(err)
			}
			if strings.Count(r.query, "?") != 2 || len(r.args) != 2 {
				t.Fatalf("raw cell expanded into multiple driver parameters: placeholders=%d args=%d", strings.Count(r.query, "?"), len(r.args))
			}
			if !reflect.DeepEqual(r.args[0], original.Values[0].Raw) {
				t.Fatal("primary-key bytes changed")
			}
			var expected any
			if !tc.value.Null {
				expected = tc.value.Raw
			}
			if !reflect.DeepEqual(r.args[1], expected) {
				t.Fatal("binary/NULL/empty cell changed")
			}
		})
	}
}
func TestRestoreRequiresOneActualInsertedRow(t *testing.T) {
	r := &insertRecorder{affected: 0}
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: r, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	im := tinyImage("users", []string{"101", "original"})
	if err = insertRow(context.Background(), db, im, im.Rows[0]); err == nil {
		t.Fatal("zero affected rows accepted as restored")
	}
}
