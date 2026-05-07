package user

import "testing"

func TestUserStatusUint64ReturnsNumericStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Status
		want uint64
	}{
		{name: "active", in: UserActive, want: 1},
		{name: "inactive", in: UserInactive, want: 2},
		{name: "blocked", in: UserBlocked, want: 3},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.Uint64(); got != tc.want {
				t.Fatalf("Uint64() = %d, want %d", got, tc.want)
			}
		})
	}
}
