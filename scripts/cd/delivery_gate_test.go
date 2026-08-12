package cd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryProbeContracts(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(repoRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	metadata := read("scripts/cd/image-metadata.sh")
	remoteDeploy := read("scripts/cd/remote-deploy.sh")
	dockerfile := read("build/docker/Dockerfile")
	compose := read("build/docker/docker-compose.prod.yml")

	for _, want := range []string{"HEALTH_PATH=\"${HEALTH_PATH:-/healthz}\"", "READINESS_PATH=\"${READINESS_PATH:-/readyz}\""} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("image metadata missing %q", want)
		}
	}
	if !strings.Contains(remoteDeploy, "iam_probe_gate_allows \"$health_status\" \"$readiness_status\"") ||
		!strings.Contains(remoteDeploy, "${READINESS_PATH}") {
		t.Fatal("remote deploy does not require the readiness gate")
	}
	readinessFailure := strings.Index(remoteDeploy, `if [ "$liveness_seen" = "1" ]`)
	livenessFailure := strings.Index(remoteDeploy, `echo "Service failed liveness`)
	if readinessFailure < 0 || livenessFailure <= readinessFailure {
		t.Fatal("remote deploy does not separate readiness and liveness failure handling")
	}
	readinessBranch := remoteDeploy[readinessFailure:livenessFailure]
	for _, forbidden := range []string{"docker restart", "docker logs"} {
		if strings.Contains(readinessBranch, forbidden) {
			t.Fatalf("readiness failure must not trigger %q", forbidden)
		}
	}
	if !strings.Contains(dockerfile, "localhost:9080/healthz") ||
		!strings.Contains(compose, "localhost:9080/healthz") {
		t.Fatal("container liveness must remain on /healthz")
	}
}

func TestRuntimeProvenanceContracts(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(repoRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	makefile := read("Makefile")
	dockerfile := read("build/docker/Dockerfile")
	serverCheck := read(".github/workflows/server-check.yml")
	for _, token := range []string{
		"VERSION_PACKAGE := github.com/FangcunMount/iam/v2/pkg/version",
		"$(VERSION_PACKAGE).GitVersion",
		"$(VERSION_PACKAGE).BuildDate",
		"$(VERSION_PACKAGE).GitCommit",
	} {
		if !strings.Contains(makefile, token) {
			t.Fatalf("Makefile does not inject runtime provenance token %s", token)
		}
	}
	if strings.Contains(makefile, "-X main.GitCommit") {
		t.Fatal("Makefile still injects the nonexistent main.GitCommit symbol")
	}
	for _, symbol := range []string{
		"github.com/FangcunMount/iam/v2/pkg/version.GitVersion",
		"github.com/FangcunMount/iam/v2/pkg/version.BuildDate",
		"github.com/FangcunMount/iam/v2/pkg/version.GitCommit",
	} {
		if !strings.Contains(dockerfile, symbol) {
			t.Fatalf("Dockerfile does not inject %s", symbol)
		}
	}
	if strings.Contains(dockerfile, "-X main.GitCommit") {
		t.Fatal("Dockerfile still injects the nonexistent main.GitCommit symbol")
	}
	for _, token := range []string{
		"{{.Config.Image}}",
		"/version",
		"gitCommit",
		"process_start_time_seconds",
		`[ "${#deployed_sha}" -ne 40 ]`,
		`${deployed_sha//[0-9a-f]/}`,
	} {
		if !strings.Contains(serverCheck, token) {
			t.Fatalf("production server check is missing runtime provenance token %q", token)
		}
	}
}

func TestRuntimeProvenanceSHAValidationMatrix(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "full lowercase SHA", value: "2a26d1a4a77334e5a38eb01b0cc786b513a97982", valid: true},
		{name: "39 characters", value: "2a26d1a4a77334e5a38eb01b0cc786b513a9798"},
		{name: "uppercase", value: "2A26d1a4a77334e5a38eb01b0cc786b513a97982"},
		{name: "non hexadecimal", value: "za26d1a4a77334e5a38eb01b0cc786b513a97982"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := `value="$1"; [ "${#value}" -eq 40 ] && [ -z "${value//[0-9a-f]/}" ]`
			err := exec.Command("bash", "-c", command, "sha-validation", tt.value).Run()
			if (err == nil) != tt.valid {
				t.Fatalf("validation result for %q: error=%v, want valid=%v", tt.value, err, tt.valid)
			}
		})
	}
}

func TestDeliveryProbeGateMatrix(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(repoRoot, "scripts/cd/image-metadata.sh")
	tests := []struct {
		name       string
		health     string
		readiness  string
		wantAllows bool
	}{
		{name: "both ready", health: "200", readiness: "200", wantAllows: true},
		{name: "liveness failed", health: "503", readiness: "200"},
		{name: "readiness failed", health: "200", readiness: "503"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := `. "$1"; iam_probe_gate_allows "$2" "$3"`
			cmd := exec.Command("sh", "-c", command, "probe-test", metadata, tt.health, tt.readiness)
			err := cmd.Run()
			if (err == nil) != tt.wantAllows {
				t.Fatalf("gate health=%s readiness=%s error=%v", tt.health, tt.readiness, err)
			}
		})
	}
}
