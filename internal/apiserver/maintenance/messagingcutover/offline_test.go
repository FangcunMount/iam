package messagingcutover

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestFrozenPlanRejectsWrongProtocolAndCanonicalMutation(t *testing.T) {
	initial := Snapshot{Database: "owned", ServerUUID: "fixture", Version: 41, VersionDDLHash: "version"}
	p, err := Build("schema", "primary", "m3-1", initial)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	sha := hex.EncodeToString(h[:])
	if _, err = Decode(raw, sha); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append([]byte(" "), raw...), append(raw, raw...), []byte(`{"schema":"bad"}`)} {
		sum := sha256.Sum256(bad)
		if _, err = Decode(bad, hex.EncodeToString(sum[:])); err == nil {
			t.Fatal("noncanonical plan accepted")
		}
	}
	if _, err = Decode(raw, "wrong"); err == nil {
		t.Fatal("wrong digest accepted")
	}
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Schema = "other" }, func(p *Plan) { p.MigrationDigest = "old" },
		func(p *Plan) { p.Before.Version = 39 }, func(p *Plan) { p.Before.Version = 46 },
		func(p *Plan) { p.Before.Binding = &Binding{Owner: "iam.mysql"} },
		func(p *Plan) { p.Before.Objects = []Object{{Name: "rm_tx_execution", Nonempty: true}} },
		func(p *Plan) { p.Before.Selection = &Selection{Protocol: "transaction", Generation: 2, Version: 1} },
		func(p *Plan) { p.Action = "rollback" }, func(p *Plan) { p.WriterEpoch = "" }, func(p *Plan) { p.StoreID = "primary\n" },
	} {
		bad := p
		change(&bad)
		if Validate(bad) == nil {
			t.Fatal("unsafe plan accepted")
		}
	}
}

func TestInitialSelectionNeedsInertCurrentSchemaAndNoLegacyDebt(t *testing.T) {
	s := Snapshot{Database: "owned", ServerUUID: "fixture", Version: 45, Selection: &Selection{Protocol: "outbox", Generation: 1, Version: 1}}
	if _, err := Build("select", "primary", "m3-1", s); err != nil {
		t.Fatal(err)
	}
	s.LegacyPending = true
	if _, err := Build("select", "primary", "m3-1", s); err == nil {
		t.Fatal("legacy responsibility ignored")
	}
	s.Version = 44
	if _, err := Build("schema", "primary", "m3-1", s); err != nil {
		t.Fatal("additive schema prematurely depends on legacy drain")
	}
	if _, err := Build("select", "primary", "m3-1", s); err == nil {
		t.Fatal("old head selected")
	}
}
