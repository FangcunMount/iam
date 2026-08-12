package identity

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func parseIDArg(name string, raw string) (meta.ID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, status.Errorf(codes.InvalidArgument, "%s is required", name)
	}
	id, err := meta.ParseID(raw)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s: %s", name, raw)
	}
	return id, nil
}

func parseIDArgs(name string, rawIDs []string) ([]meta.ID, error) {
	ids := make([]meta.ID, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		id, err := parseIDArg(name, rawID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
