package input

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIDPreservesLegacySscanfBehavior(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain", raw: "42", want: "42"},
		{name: "leading space", raw: " 42", want: "42"},
		{name: "trailing non digit", raw: "42abc", want: "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := ParseUserID(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, userID.String())

			profileID, err := ParseProfileID(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, profileID.String())
		})
	}
}

func TestParseIDRejectsEmptyAndNonNumeric(t *testing.T) {
	for _, raw := range []string{"", "abc"} {
		_, err := ParseUserID(raw)
		require.Error(t, err)

		_, err = ParseProfileID(raw)
		require.Error(t, err)
	}
}

func TestParseOptionalContactValues(t *testing.T) {
	phone, err := ParseOptionalPhone("")
	require.NoError(t, err)
	assert.True(t, phone.IsEmpty())

	phone, err = ParseOptionalPhone("13800138000")
	require.NoError(t, err)
	assert.Equal(t, "+8613800138000", phone.String())

	email, err := ParseOptionalEmail("")
	require.NoError(t, err)
	assert.True(t, email.IsEmpty())

	email, err = ParseOptionalEmail("User@Example.COM")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", email.String())
}

func TestProfileMeasurementConversions(t *testing.T) {
	height, err := ParseHeightCm(120)
	require.NoError(t, err)
	assert.Equal(t, uint32(120), HeightCm(height))

	weight, err := ParseWeightGrams(25500)
	require.NoError(t, err)
	assert.Equal(t, uint32(25500), WeightGrams(weight))
}

func TestBirthdayAndGenderRemainLooseValues(t *testing.T) {
	assert.Equal(t, uint8(9), ParseGender(9).Value())
	assert.Equal(t, "not-a-date", ParseBirthday("not-a-date").String())
}
