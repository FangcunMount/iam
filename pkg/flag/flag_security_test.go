package flag

import "testing"

func TestFlagValueForLogRedactsSensitiveValues(t *testing.T) {
	tests := []struct {
		name      string
		flagName  string
		flagValue string
		want      string
	}{
		{name: "password set", flagName: "mysql.password", flagValue: "password-sentinel", want: "<set>"},
		{name: "secret unset", flagName: "seed-mock-auth.shared-secret", flagValue: "", want: "<unset>"},
		{name: "token set", flagName: "api-token", flagValue: "token-sentinel", want: "<set>"},
		{name: "private key path", flagName: "secure.tls.private-key-file", flagValue: "/secret/key.pem", want: "<set>"},
		{name: "non-sensitive", flagName: "mysql.host", flagValue: "db.internal", want: "db.internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagValueForLog(tt.flagName, tt.flagValue); got != tt.want {
				t.Fatalf("flagValueForLog(%q, %q) = %q, want %q", tt.flagName, tt.flagValue, got, tt.want)
			}
		})
	}
}
