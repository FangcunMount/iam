package cd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparePackageWritesAliyunSMSCredentialEnv(t *testing.T) {
	tmp := t.TempDir()
	packageDir := filepath.Join(tmp, "deploy-package")
	archive := filepath.Join(tmp, "deploy-package-apiserver.tar.gz")

	output, err := runPreparePackage(t, packageDir, archive, []string{
		"SMS_ALIYUN_ACCESS_KEY_ID=ak-test",
		"SMS_ALIYUN_ACCESS_KEY_SECRET=sk-test",
	})
	if err != nil {
		t.Fatalf("prepare-package.sh error = %v\n%s", err, output)
	}

	envFile := filepath.Join(packageDir, "configs", "env", "config.prod.env")
	body, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read generated env file: %v", err)
	}
	env := string(body)
	for _, want := range []string{
		"IAM_APISERVER_SMS_ALIYUN_ACCESS_KEY_ID=ak-test",
		"IAM_APISERVER_SMS_ALIYUN_ACCESS_KEY_SECRET=sk-test",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("generated env missing %q\n%s", want, env)
		}
	}
	for _, unwanted := range []string{
		"IAM_APISERVER_SMS_PROVIDER=",
		"IAM_APISERVER_SMS_ALIYUN_SIGN_NAME=",
		"IAM_APISERVER_SMS_ALIYUN_TEMPLATE_CODE=",
		"IAM_APISERVER_SMS_ALIYUN_ENDPOINT=",
		"IAM_APISERVER_SMS_ALIYUN_CODE_PARAM_NAME=",
		"IAM_APISERVER_SMS_ALIYUN_TIMEOUT_MILLIS=",
	} {
		if strings.Contains(env, unwanted) {
			t.Fatalf("generated env should not inject config-file setting %q\n%s", unwanted, env)
		}
	}
	if strings.Contains(output, "sk-test") {
		t.Fatalf("redacted output leaked SMS access key secret:\n%s", output)
	}
}

func TestPreparePackageRejectsPartialAliyunCredential(t *testing.T) {
	tmp := t.TempDir()
	packageDir := filepath.Join(tmp, "deploy-package")
	archive := filepath.Join(tmp, "deploy-package-apiserver.tar.gz")

	output, err := runPreparePackage(t, packageDir, archive, []string{
		"SMS_ALIYUN_ACCESS_KEY_ID=ak-test",
	})
	if err == nil {
		t.Fatalf("prepare-package.sh should reject partial Aliyun credentials\n%s", output)
	}
	if !strings.Contains(output, "must be both set or both empty") {
		t.Fatalf("unexpected prepare-package.sh output:\n%s", output)
	}
}

func runPreparePackage(t *testing.T, packageDir, archive string, extraEnv []string) (string, error) {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	cmd := exec.Command("bash", "scripts/cd/prepare-package.sh")
	cmd.Dir = repoRoot
	cmd.Env = append(basePreparePackageEnv(packageDir, archive), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func basePreparePackageEnv(packageDir, archive string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"SERVICE=apiserver",
		"DEPLOY_PACKAGE_DIR=" + packageDir,
		"DEPLOY_PACKAGE=" + archive,
		"MYSQL_HOST=mysql.internal",
		"MYSQL_PORT=3306",
		"MYSQL_USERNAME=iam",
		"MYSQL_PASSWORD=mysql-pass",
		"MYSQL_DBNAME=iam",
		"REDIS_HOST=redis.internal",
		"REDIS_PORT=6379",
		"REDIS_DB=0",
		"IPD_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef",
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		env = append(env, "TMPDIR="+tmp)
	}
	return env
}
