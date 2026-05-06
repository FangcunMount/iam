package profilelink

import "strings"

// ParseRelation maps external text to the profile link domain vocabulary.
func ParseRelation(relation string) Relation {
	switch strings.ToLower(strings.TrimSpace(relation)) {
	case "self":
		return RelSelf
	case "parent":
		return RelParent
	case "grandparent":
		return RelGrandparent
	case "other":
		return RelOther
	default:
		return RelOther
	}
}

// NormalizeRelation returns the stable public relation text for a raw input.
func NormalizeRelation(relation string) string {
	return string(ParseRelation(relation))
}
