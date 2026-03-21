package vaultfs

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── ACL tests ────────────────────────────────────────────────────────────────

func TestEmptyACLDeniesAll(t *testing.T) {
	acl := EmptyACL()
	if acl.Allowed("any_secret", 65001) {
		t.Error("EmptyACL should deny all access")
	}
}

func TestLoadACLAllowedAndDenied(t *testing.T) {
	content := `
db_password:
  - 65001
  - 65002
api_key:
  - 65003
`
	path := writeTempFile(t, content)
	acl, err := LoadACL(path)
	if err != nil {
		t.Fatalf("LoadACL: %v", err)
	}

	cases := []struct {
		secret string
		uid    uint32
		want   bool
	}{
		{"db_password", 65001, true},
		{"db_password", 65002, true},
		{"db_password", 65003, false},
		{"api_key", 65003, true},
		{"api_key", 65001, false},
		{"nonexistent", 65001, false},
	}
	for _, c := range cases {
		got := acl.Allowed(c.secret, c.uid)
		if got != c.want {
			t.Errorf("Allowed(%q, %d) = %v, want %v", c.secret, c.uid, got, c.want)
		}
	}
}

func TestLoadACLEmptyFile(t *testing.T) {
	path := writeTempFile(t, "{}")
	acl, err := LoadACL(path)
	if err != nil {
		t.Fatalf("LoadACL on empty map: %v", err)
	}
	if acl.Allowed("anything", 65001) {
		t.Error("ACL loaded from {} should deny all access")
	}
}

func TestLoadACLMissingFile(t *testing.T) {
	_, err := LoadACL("/nonexistent/path/acl.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadACLInvalidYAML(t *testing.T) {
	path := writeTempFile(t, "not: valid: yaml: [[[")
	_, err := LoadACL(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// ─── EnvFiles tests ───────────────────────────────────────────────────────────

func TestEmptyEnvFilesHasNoEntries(t *testing.T) {
	ef := EmptyEnvFiles()
	if len(ef) != 0 {
		t.Errorf("EmptyEnvFiles: expected 0 entries, got %d", len(ef))
	}
}

func TestLoadEnvFilesParsesCorrectly(t *testing.T) {
	content := `
nextcloud:
  uid: 65001
  env:
    NEXTCLOUD_ADMIN_PASSWORD: nextcloud_admin_password
    NEXTCLOUD_DB_PASSWORD: db_password_nextcloud
myapp:
  uid: 65002
  env:
    DATABASE_URL: db_password_myapp
`
	path := writeTempFile(t, content)
	ef, err := LoadEnvFiles(path)
	if err != nil {
		t.Fatalf("LoadEnvFiles: %v", err)
	}

	if len(ef) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ef))
	}

	nc, ok := ef["nextcloud"]
	if !ok {
		t.Fatal("missing 'nextcloud' entry")
	}
	if nc.UID != 65001 {
		t.Errorf("nextcloud UID: got %d, want 65001", nc.UID)
	}
	if nc.Env["NEXTCLOUD_ADMIN_PASSWORD"] != "nextcloud_admin_password" {
		t.Errorf("nextcloud env mapping wrong: %v", nc.Env)
	}
	if nc.Env["NEXTCLOUD_DB_PASSWORD"] != "db_password_nextcloud" {
		t.Errorf("nextcloud env mapping wrong: %v", nc.Env)
	}

	app, ok := ef["myapp"]
	if !ok {
		t.Fatal("missing 'myapp' entry")
	}
	if app.UID != 65002 {
		t.Errorf("myapp UID: got %d, want 65002", app.UID)
	}
	if app.Env["DATABASE_URL"] != "db_password_myapp" {
		t.Errorf("myapp env mapping wrong: %v", app.Env)
	}
}

func TestLoadEnvFilesMissingFile(t *testing.T) {
	_, err := LoadEnvFiles("/nonexistent/path/envfiles.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadEnvFilesInvalidYAML(t *testing.T) {
	path := writeTempFile(t, "not: valid: yaml: [[[")
	_, err := LoadEnvFiles(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadEnvFilesEmptyFile(t *testing.T) {
	path := writeTempFile(t, "{}")
	ef, err := LoadEnvFiles(path)
	if err != nil {
		t.Fatalf("LoadEnvFiles on empty map: %v", err)
	}
	if len(ef) != 0 {
		t.Errorf("expected 0 entries, got %d", len(ef))
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return path
}
