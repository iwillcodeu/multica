package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAlibabaCloudCredentialsFromINI(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.ini")
	content := `[credentials]
alibaba_cloud_access_key_id = AKIA_TEST_ID
alibaba_cloud_access_key_secret = secret_value_with_spaces_ok
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	id, sec, err := ReadAlibabaCloudCredentialsFromINI(p)
	if err != nil {
		t.Fatal(err)
	}
	if id != "AKIA_TEST_ID" {
		t.Fatalf("id: got %q", id)
	}
	if sec != "secret_value_with_spaces_ok" {
		t.Fatalf("secret: got %q", sec)
	}
}

func TestReadAlibabaCloudCredentialsFromINI_wrongSection(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.ini")
	if err := os.WriteFile(p, []byte(`[other]
alibaba_cloud_access_key_id = x
alibaba_cloud_access_key_secret = y
`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadAlibabaCloudCredentialsFromINI(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
