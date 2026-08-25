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
		"VERSION_PACKAGE := github.com/FangcunMount/iam/v3/pkg/version",
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
		"github.com/FangcunMount/iam/v3/pkg/version.GitVersion",
		"github.com/FangcunMount/iam/v3/pkg/version.BuildDate",
		"github.com/FangcunMount/iam/v3/pkg/version.GitCommit",
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

func TestAuthZCutoverCanPauseOnlyAutomaticProductionDeploys(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "cd.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	if count := strings.Count(workflow, "vars.AUTHZ_CUTOVER_AUTO_DEPLOY_PAUSED != 'true'"); count != 2 {
		t.Fatalf("automatic deploy pause guard count = %d, want 2", count)
	}
	if !strings.Contains(workflow, "github.event_name == 'workflow_dispatch' ||") {
		t.Fatal("manual production deploy path must remain available during the cutover")
	}
}

func TestAuthZProductionCutoverWorkflowKeepsTheMaintenanceOrder(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "authz-cutover.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	ordered := []string{
		"Validate immutable release evidence",
		"Stop IAM authorization consumers",
		"Create verified maintenance-window database backup",
		"Recheck RoleBinding migration prerequisite",
		"Convert policies and retire legacy authorization schema",
		"Verify final database status",
	}
	position := -1
	for _, token := range ordered {
		next := strings.Index(workflow, token)
		if next <= position {
			t.Fatalf("cutover workflow step %q is missing or out of order", token)
		}
		position = next
	}
	for _, token := range []string{
		"environment:\n      name: production",
		"group: iam-production-controlled-database-operation",
		"CUTOVER_AUTHZ_V3",
		"expected_iam_sha",
		"expected_qs_sha",
		"qs_stop_run_id",
	} {
		if !strings.Contains(workflow, token) {
			t.Fatalf("cutover workflow is missing %q", token)
		}
	}
}

func TestAuthZProductionCutoverScriptsAreExactAndFailClosed(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) string {
		t.Helper()
		body, readErr := os.ReadFile(filepath.Join(repoRoot, path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		return string(body)
	}
	consumerControl := read("scripts/cd/authz-consumer-control.sh")
	for _, token := range []string{
		"--filter 'name=^/iam-apiserver$'",
		"${RELEASE_SHA}.iam-containers",
		"run_privileged mkdir -p -- \"$STATE_ROOT\"",
		"run_privileged chown \"$(id -u):$(id -g)\" \"$STATE_ROOT\"",
		"cp -- \"$temporary_state\" \"$STATE_FILE\"",
		"run_privileged docker stop \"$container_id\"",
	} {
		if !strings.Contains(consumerControl, token) {
			t.Fatalf("consumer control is missing %q", token)
		}
	}
	if strings.Contains(consumerControl, "run_privileged install") {
		t.Fatal("consumer control must not require sudo install outside the production sudoers contract")
	}

	databaseCutover := read("scripts/cd/authz-database-cutover.sh")
	ordered := []string{
		"01-migrate-additive",
		"02-preflight",
		"03-apply",
		"04-verify",
		"05-evidence",
		"06-retire-legacy",
	}
	position := -1
	for _, token := range ordered {
		next := strings.Index(databaseCutover, token)
		if next <= position {
			t.Fatalf("database cutover step %q is missing or out of order", token)
		}
		position = next
	}
	for _, token := range []string{
		"CUTOVER_AUTHZ_V3",
		"iam-apiserver must be stopped",
		"/tmp/iam-authz-cutover-${IAM_AUTHZ_RELEASE_SHA}/iam-maintenance",
		"run_privileged mkdir -p -- \"$EVIDENCE_DIR\"",
		"run_privileged chown \"$(id -u):$(id -g)\" \"$EVIDENCE_DIR\"",
		"cp -- \"$evidence_file\" \"$EVIDENCE_DIR/$(basename \"$evidence_file\")\"",
		"sha256sum -c checksums.sha256",
	} {
		if !strings.Contains(databaseCutover, token) {
			t.Fatalf("database cutover is missing %q", token)
		}
	}
	if strings.Contains(databaseCutover, "run_privileged install") {
		t.Fatal("database cutover must not require sudo install outside the production sudoers contract")
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
