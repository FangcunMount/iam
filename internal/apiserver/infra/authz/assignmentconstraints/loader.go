package assignmentconstraints

import (
	"fmt"
	"os"
	"strings"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentadmission"
	"gopkg.in/yaml.v3"
)

func Load(path string) (assignmentadmission.Policy, error) {
	config, err := loadConfig(path)
	if err != nil {
		return nil, err
	}
	policy, err := assignmentadmission.New(config)
	if err != nil {
		return nil, fmt.Errorf("validate grpc authz assignment constraints: %w", err)
	}
	return policy, nil
}

// LoadWithACL loads the request-content constraints and verifies that their
// service coverage exactly matches the services which the method ACL permits
// to mutate assignments.
func LoadWithACL(path, aclPath string) (assignmentadmission.Policy, error) {
	config, err := loadConfig(path)
	if err != nil {
		return nil, err
	}
	if err := validateAgainstACL(config, aclPath); err != nil {
		return nil, err
	}
	policy, err := assignmentadmission.New(config)
	if err != nil {
		return nil, fmt.Errorf("validate grpc authz assignment constraints: %w", err)
	}
	return policy, nil
}

func loadConfig(path string) (assignmentadmission.Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return assignmentadmission.Config{}, fmt.Errorf("grpc authz assignment constraints file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return assignmentadmission.Config{}, fmt.Errorf("read grpc authz assignment constraints: %w", err)
	}
	var config assignmentadmission.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return assignmentadmission.Config{}, fmt.Errorf("parse grpc authz assignment constraints: %w", err)
	}
	if err := assignmentadmission.Validate(config); err != nil {
		return assignmentadmission.Config{}, fmt.Errorf("validate grpc authz assignment constraints: %w", err)
	}
	return config, nil
}

const (
	grantAssignmentMethod    = "/iam.authz.v3.AuthorizationService/GrantAssignment"
	revokeAssignmentMethod   = "/iam.authz.v3.AuthorizationService/RevokeAssignment"
	replaceAssignmentsMethod = "/iam.authz.v3.AuthorizationService/ReplaceManagedAssignments"
)

func validateAgainstACL(config assignmentadmission.Config, aclPath string) error {
	aclPath = strings.TrimSpace(aclPath)
	if aclPath == "" {
		return fmt.Errorf("grpc acl config file is required when assignment constraints are enabled")
	}
	data, err := os.ReadFile(aclPath)
	if err != nil {
		return fmt.Errorf("read grpc acl for assignment constraints: %w", err)
	}
	var acl interceptors.ACLConfig
	if err := yaml.Unmarshal(data, &acl); err != nil {
		return fmt.Errorf("parse grpc acl for assignment constraints: %w", err)
	}

	mutatingServices := make(map[string]struct{})
	for _, service := range acl.Services {
		if service == nil || !service.Enabled {
			continue
		}
		if allowsMethod(service.AllowedMethods, grantAssignmentMethod) ||
			allowsMethod(service.AllowedMethods, revokeAssignmentMethod) ||
			allowsMethod(service.AllowedMethods, replaceAssignmentsMethod) {
			mutatingServices[strings.TrimSpace(service.ServiceName)] = struct{}{}
		}
	}
	for service := range mutatingServices {
		if _, configured := config.Services[service]; !configured {
			return fmt.Errorf("grpc acl service %s can mutate assignments but has no request constraint", service)
		}
	}
	for service := range config.Services {
		if _, allowed := mutatingServices[service]; !allowed {
			return fmt.Errorf("assignment constraint service %s is not allowed to mutate assignments by grpc acl", service)
		}
	}
	return nil
}

func allowsMethod(patterns []string, method string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == method {
			return true
		}
		if strings.HasSuffix(pattern, "/*") &&
			strings.HasPrefix(method, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}
