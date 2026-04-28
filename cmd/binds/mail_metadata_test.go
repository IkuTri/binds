package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadMailMetadataInline(t *testing.T) {
	metadata := `{"kind":"manual_tool_session","tool":"claude-code","mode":"manual"}`
	got, err := readMailMetadata(metadata, "")
	if err != nil {
		t.Fatalf("readMailMetadata returned error: %v", err)
	}
	if string(got) != metadata {
		t.Fatalf("metadata = %q, want %q", string(got), metadata)
	}

	data, err := json.Marshal(map[string]interface{}{"metadata": got})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if string(data) != `{"metadata":{"kind":"manual_tool_session","tool":"claude-code","mode":"manual"}}` {
		t.Fatalf("payload JSON = %s", data)
	}
}

func TestReadMailMetadataFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.json")
	metadata := `{"source":"file","state":"in_progress"}`
	if err := os.WriteFile(path, []byte(metadata), 0644); err != nil {
		t.Fatalf("write metadata file: %v", err)
	}

	got, err := readMailMetadata("@"+path, "")
	if err != nil {
		t.Fatalf("readMailMetadata @file returned error: %v", err)
	}
	if string(got) != metadata {
		t.Fatalf("metadata = %q, want %q", string(got), metadata)
	}

	got, err = readMailMetadata("", path)
	if err != nil {
		t.Fatalf("readMailMetadata --metadata-file returned error: %v", err)
	}
	if string(got) != metadata {
		t.Fatalf("metadata = %q, want %q", string(got), metadata)
	}
}

func TestReadMailMetadataRejectsInvalidJSON(t *testing.T) {
	if _, err := readMailMetadata("{invalid", ""); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestReadMailMetadataRejectsAmbiguousInput(t *testing.T) {
	if _, err := readMailMetadata(`{"inline":true}`, "metadata.json"); err == nil {
		t.Fatal("expected ambiguous metadata input error")
	}
}

func TestReadMailMetadataRejectsMissingFilePath(t *testing.T) {
	if _, err := readMailMetadata("@", ""); err == nil {
		t.Fatal("expected missing metadata file path error")
	}
}
