package cd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeploymentPreservesReleaseBoundMaintenanceACL(t *testing.T) {
	for _, scenario := range []string{"absent", "valid", "wrong release", "corrupt", "incomplete", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			freeze, pkg := filepath.Join(root, "freeze"), filepath.Join(root, "package")
			if err := os.MkdirAll(filepath.Join(pkg, "configs"), 0700); err != nil {
				t.Fatal(err)
			}
			write := func(path, value string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(pkg, "configs/grpc_acl.yaml")
			write(target, "release ACL")
			if scenario != "absent" {
				if err := os.Mkdir(freeze, 0700); err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(freeze, "grpc_acl.yaml"), "deny management writes")
				write(filepath.Join(freeze, "sha256"), fmt.Sprintf("%x", sha256.Sum256([]byte("deny management writes"))))
				write(filepath.Join(freeze, "release_sha"), "release-1")
			}
			switch scenario {
			case "wrong release":
				write(filepath.Join(freeze, "release_sha"), "old-release")
			case "corrupt":
				write(filepath.Join(freeze, "grpc_acl.yaml"), "changed ACL")
			case "incomplete":
				if err := os.Remove(filepath.Join(freeze, "sha256")); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				path := filepath.Join(freeze, "grpc_acl.yaml")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("bash", "-c", `set -euo pipefail
source ./maintenance-acl.sh
# Production deploy sudo permits filesystem sync and ownership changes,
# but deliberately does not permit cat, sha256sum or cp.
priv(){
  case "$1" in
    test|rsync|chown) "$@" ;;
    *) echo "sudo command not allowed: $1" >&2; return 1 ;;
  esac
}
sha256sum(){ shasum -a 256 "$@"; }
SUDO=priv
preserve_maintenance_acl "$1" "$2" release-1`, "test", freeze, pkg)
			output, err := command.CombinedOutput()
			success := scenario == "valid" || scenario == "absent"
			if (err == nil) != success {
				t.Fatalf("result=%v output=%s", err, output)
			}
			content, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			want := "release ACL"
			if scenario == "valid" {
				want = "deny management writes"
			}
			if string(content) != want {
				t.Fatalf("deployment ACL changed unexpectedly: %q", content)
			}
		})
	}
}

func TestPreparePackageIncludesMaintenanceACLGuard(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "package")
	if output, err := runPreparePackage(t, pkg, filepath.Join(root, "package.tar.gz"), []string{"SEED_MOCK_AUTH_ENABLED=false"}); err != nil {
		t.Fatalf("prepare package: %v\n%s", err, output)
	}
	source, err := os.ReadFile("maintenance-acl.sh")
	if err != nil {
		t.Fatal(err)
	}
	packaged, err := os.ReadFile(filepath.Join(pkg, "scripts/cd/maintenance-acl.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != string(packaged) {
		t.Fatal("release package lost maintenance ACL guard")
	}
}
