// Package timezone defines the application's fixed UTC+8 business timezone.
package timezone

import (
	"time"
	_ "time/tzdata" // Keep MySQL DSN location decoding independent of host zoneinfo.
)

const (
	// Etc/GMT uses the POSIX sign convention: GMT-8 is fixed UTC+8, without DST.
	Name   = "Etc/GMT-8"
	Offset = 8 * time.Hour
)

var Location = mustLocation()

func mustLocation() *time.Location {
	location, err := time.LoadLocation(Name)
	if err != nil {
		panic(err) // The named location is supplied by the embedded tzdata above.
	}
	return location
}
