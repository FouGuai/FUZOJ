package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fuzoj/internal/cli/command"
	"fuzoj/tests/testutil"
)

func TestBuildSubmitCreateWithSourceFile(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.cpp")
	if err := os.WriteFile(sourcePath, []byte("int main() {}"), 0o600); err != nil {
		t.Fatalf("write temp source failed: %v", err)
	}

	cmd := command.Registry()["submit create"]
	params := command.Params{}
	params.Set("problem_id", "1")
	params.Set("user_id", "2")
	params.Set("language_id", "cpp")
	params.Set("source_file", sourcePath)
	params.Set("source_code", "_file_")

	req, err := command.BuildRequest(cmd, params)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	var payload map[string]interface{}
	testutil.MustUnmarshalJSON(t, req.Body, &payload)
	testutil.AssertEqual(t, payload["source_code"], "int main() {}")
}

func TestBuildUploadCompleteWithFiles(t *testing.T) {
	dir := t.TempDir()
	partsPath := filepath.Join(dir, "parts.json")
	manifestPath := filepath.Join(dir, "manifest.json")
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(partsPath, []byte(`[{"part_number":1,"etag":"abc"}]`), 0o600); err != nil {
		t.Fatalf("write parts failed: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"limits":{"time":1}}`), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cmd := command.Registry()["problem upload-complete"]
	params := command.Params{}
	params.Set("id", "1")
	params.Set("upload_id", "2")
	params.Set("parts_file", partsPath)
	params.Set("parts_json", "_file_")
	params.Set("manifest_file", manifestPath)
	params.Set("manifest_json", "_file_")
	params.Set("config_file", configPath)
	params.Set("config_json", "_file_")
	params.Set("manifest_hash", "mh")
	params.Set("data_pack_hash", "dh")

	req, err := command.BuildRequest(cmd, params)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	var payload map[string]json.RawMessage
	testutil.MustUnmarshalJSON(t, req.Body, &payload)
	testutil.AssertTrue(t, json.Valid(payload["parts"]), "parts should be valid json")
	testutil.AssertTrue(t, json.Valid(payload["manifest_json"]), "manifest_json should be valid json")
	testutil.AssertTrue(t, json.Valid(payload["config_json"]), "config_json should be valid json")
}

func TestBuildPathParams(t *testing.T) {
	cmd := command.Registry()["problem latest"]
	params := command.Params{}
	params.Set("id", "99")
	req, err := command.BuildRequest(cmd, params)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	testutil.AssertEqual(t, req.Path, "/api/v1/problems/99/latest")
}
