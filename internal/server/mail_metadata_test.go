package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessageStoresMetadata(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/server.db")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	metadata := `{"kind":"manual_tool_session","tool":"claude-code","mode":"manual","boundary":"human_operated_external_tool"}`
	msg, err := store.SendMessage(context.Background(), "codex", "obscura", "tracked progress", "worklog", "text", "normal", metadata, nil)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.Metadata != metadata {
		t.Fatalf("message metadata = %q, want %q", msg.Metadata, metadata)
	}

	inbox, err := store.GetInbox(context.Background(), "obscura", false, "", 10)
	if err != nil {
		t.Fatalf("GetInbox: %v", err)
	}
	if len(inbox) != 1 {
		t.Fatalf("inbox length = %d, want 1", len(inbox))
	}
	if inbox[0].Metadata != metadata {
		t.Fatalf("stored metadata = %q, want %q", inbox[0].Metadata, metadata)
	}
}

func TestMailAPIMetadataRoundTrip(t *testing.T) {
	s, err := New(&Config{ConfigDir: t.TempDir(), LocalIdentity: "codex", LocalAgentType: "codex"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.store.Close()

	body := []byte(`{"recipient":"obscura","body":"tracked progress","metadata":{"kind":"manual_tool_session","tool":"claude-code","mode":"manual","boundary":"human_operated_external_tool"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/mail", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Authorization", "Bearer "+s.localToken)
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response struct {
		ID       int64           `json:"id"`
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID == 0 {
		t.Fatal("expected response id")
	}

	var metadata map[string]string
	if err := json.Unmarshal(response.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["kind"] != "manual_tool_session" || metadata["boundary"] != "human_operated_external_tool" {
		t.Fatalf("metadata = %#v", metadata)
	}

	inbox, err := s.store.GetInbox(context.Background(), "obscura", false, "", 10)
	if err != nil {
		t.Fatalf("GetInbox: %v", err)
	}
	if len(inbox) != 1 {
		t.Fatalf("inbox length = %d, want 1", len(inbox))
	}
	if inbox[0].Metadata == "" {
		t.Fatal("expected persisted metadata")
	}
}
