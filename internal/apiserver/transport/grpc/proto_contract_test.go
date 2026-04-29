package grpc

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTransportGRPCServicesCoverEveryIAMProtoService(t *testing.T) {
	root := repoRoot(t)
	serviceRoot := filepath.Join(root, "internal", "apiserver", "transport", "grpc", "service")
	serviceSource := readGoSourceTree(t, serviceRoot)
	serviceRe := regexp.MustCompile(`(?m)^service\s+(\w+)`)

	err := filepath.WalkDir(filepath.Join(root, "api", "grpc", "iam"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range serviceRe.FindAllStringSubmatch(string(data), -1) {
			serviceName := match[1]
			registration := "Register" + serviceName + "Server"
			if !strings.Contains(serviceSource, registration) {
				t.Fatalf("%s declares service %s but transport/grpc/service does not call %s", path, serviceName, registration)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readGoSourceTree(t *testing.T, root string) string {
	t.Helper()
	var builder strings.Builder
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		builder.Write(data)
		builder.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return builder.String()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(dir, "../../../.."))
}
