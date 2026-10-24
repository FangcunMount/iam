package migration

import (
	"crypto/sha256"
	"encoding/hex"
)

// MessagingMigrationDigest binds the additive audit and M3 source templates. It is
// not a database backup or an assertion that live migration 41 is installed.
func MessagingMigrationDigest() string {
	h := sha256.New()
	for version := uint(41); version <= 45; version++ {
		name := messagingMigrationFile(version)
		raw, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			panic("embedded M3 migration missing")
		}
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(raw)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
