package handler

import (
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

func parsePositiveInt(value, field string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid %s: %s", field, value)
	}
	if result <= 0 {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "%s must be positive", field)
	}
	return result, nil
}

func parseNonNegativeInt(value, field string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid %s: %s", field, value)
	}
	if result < 0 {
		return 0, perrors.WithCode(code.ErrInvalidArgument, "%s cannot be negative", field)
	}
	return result, nil
}

func parseKeyStatus(status string) (uint8, error) {
	switch strings.ToLower(status) {
	case "active":
		return 1, nil
	case "grace":
		return 2, nil
	case "retired":
		return 3, nil
	default:
		return 0, perrors.WithCode(code.ErrInvalidArgument, "invalid status: %s (must be active, grace, or retired)", status)
	}
}
