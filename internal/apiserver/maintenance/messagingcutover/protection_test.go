package messagingcutover

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestOriginalProtectionCannotExpandToBusinessOrChangedJournal(t *testing.T) {
	ddl := "CREATE TABLE `schema_migrations` (`version` bigint NOT NULL,`dirty` boolean NOT NULL) ENGINE=InnoDB"
	before := Snapshot{Database: "owned", ServerUUID: "fixture", Version: 41, VersionDDLHash: digest([]byte(ddl))}
	plan, err := Build("schema", "primary", "m3-1", before)
	if err != nil {
		t.Fatal(err)
	}
	p := Protection{ProtectionSchema, "seven-new-objects-and-original-version-journal", "primary", "m3-1", plan.MigrationDigest, before, ddl}
	raw, err := EncodeProtection(p)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	if _, err = DecodeProtection(raw, hex.EncodeToString(h[:])); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Protection){
		func(p *Protection) { p.Scope = "full-database" }, func(p *Protection) { p.Before.Version = 42 },
		func(p *Protection) { p.Before.LegacyPending = true }, func(p *Protection) { p.Before.Binding = &Binding{Owner: "iam.mysql"} },
		func(p *Protection) { p.Before.Objects = []Object{{Name: "rm_tx_execution"}} },
		func(p *Protection) {
			p.OriginalVersionDDL = strings.ReplaceAll(p.OriginalVersionDDL, "schema_migrations", "business")
		},
		func(p *Protection) { p.MigrationDigest = "changed" }, func(p *Protection) { p.WriterEpoch = "" },
	} {
		bad := p
		mutate(&bad)
		if validateProtection(bad) == nil {
			t.Fatal("expanded protection accepted")
		}
	}
	if _, err = DecodeProtection(append(raw, ' '), hex.EncodeToString(h[:])); err == nil {
		t.Fatal("changed protection accepted")
	}
}

func TestProduction40ProtectionBindsAdditionalAuditObjectScope(t *testing.T) {
	ddl := "CREATE TABLE `schema_migrations` (`version` bigint NOT NULL,`dirty` boolean NOT NULL) ENGINE=InnoDB"
	before := Snapshot{Database: "owned", ServerUUID: "fixture", Version: 40, VersionDDLHash: digest([]byte(ddl))}
	plan, err := Build("schema", "primary", "m3-1", before)
	if err != nil {
		t.Fatal("current production schema40 must support an explicit additive plan:", err)
	}
	p := Protection{ProtectionSchema, "eight-new-objects-and-original-version-journal", "primary", "m3-1", plan.MigrationDigest, before, ddl}
	raw, err := EncodeProtection(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeProtection(raw, digest(raw)); err != nil {
		t.Fatal(err)
	}
	p.Scope = "seven-new-objects-and-original-version-journal"
	if validateProtection(p) == nil {
		t.Fatal("schema40 protection omitted its newly introduced audit table")
	}
	p.Scope = "eight-new-objects-and-original-version-journal"
	p.Before.Version = 41
	if validateProtection(p) == nil {
		t.Fatal("schema41 protection must not claim ownership of its preexisting audit table")
	}
}
