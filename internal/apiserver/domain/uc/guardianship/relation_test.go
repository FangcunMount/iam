package guardianship

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRelationNormalizesKnownValues(t *testing.T) {
	assert.Equal(t, RelSelf, ParseRelation(" self "))
	assert.Equal(t, RelParent, ParseRelation("PARENT"))
	assert.Equal(t, RelGrandparent, ParseRelation("grandparent"))
	assert.Equal(t, RelOther, ParseRelation("other"))
}

func TestParseRelationFallsBackToOther(t *testing.T) {
	assert.Equal(t, RelOther, ParseRelation(""))
	assert.Equal(t, RelOther, ParseRelation("unknown"))
	assert.Equal(t, "other", NormalizeRelation("unknown"))
}
