package main

import (
	"encoding/json"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/accountprovision"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountInputAndReportKeepCredentialsPrivate(t *testing.T) {
	valid := `{"request_id":"store-test","actor_id":"10001","username":"store_test_admin","name":"门店管理员","reason":"approved","password":"PrivateTestPassword-123456"}`
	for _, tc := range []struct {
		name, body string
		mode       os.FileMode
		ok         bool
	}{
		{"valid", valid, 0600, true}, {"readable", valid, 0644, false}, {"extra", valid + ` {}`, 0600, false}, {"unknown", `{"oops":1}`, 0600, false}, {"oversized", valid + strings.Repeat(" ", 65536), 0600, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(p, []byte(tc.body), tc.mode); err != nil {
				t.Fatal(err)
			}
			_, err := readAccountInput(p)
			if (err == nil) != tc.ok {
				t.Fatal("unexpected input validation", err)
			}
		})
	}
	raw, err := json.Marshal(accountprovision.Report{State: "pending", Username: "store_test_admin"})
	if err != nil || strings.Contains(string(raw), "password") {
		t.Fatal("credential in report")
	}
}
