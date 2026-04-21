package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	TokenPrefix    = "bnd_"
	tokenBytes     = 32
	presenceReap   = 90 * time.Second
	reaperInterval = 30 * time.Second
)

type Server struct {
	store     *Store
	mux       *http.ServeMux
	server    *http.Server
	configDir string
	hub       *Hub

	localToken    string
	localIdentity string

	reaperDone chan struct{}
	mu         sync.RWMutex
}

type Config struct {
	Port          int
	Listen        string
	ConfigDir     string
	LocalIdentity string
	LocalAgentType string
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Port:      8890,
		Listen:    "127.0.0.1",
		ConfigDir: filepath.Join(home, ".config", "binds"),
	}
}

func New(cfg *Config) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	dbPath := filepath.Join(cfg.ConfigDir, "server.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open server store: %w", err)
	}

	s := &Server{
		store:         store,
		mux:           http.NewServeMux(),
		configDir:     cfg.ConfigDir,
		hub:           newHub(),
		localIdentity: cfg.LocalIdentity,
		reaperDone:    make(chan struct{}),
	}

	if s.localIdentity != "" {
		agentType := cfg.LocalAgentType
		if agentType == "" {
			agentType = "cc"
		}
		s.ensureLocalAgent(s.localIdentity, agentType)
	}

	s.registerRoutes()

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port),
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := s.writeLocalToken(); err != nil {
		store.Close()
		return nil, fmt.Errorf("write local token: %w", err)
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	go s.presenceReaper(ctx)

	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.server.Addr, err)
	}

	log.Printf("binds server listening on %s", s.server.Addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}

	<-s.reaperDone
	s.store.Close()
	return nil
}

func (s *Server) registerRoutes() {
	// Health
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Agents (register is unauthenticated if no agents exist yet, otherwise needs auth)
	s.mux.HandleFunc("POST /api/agents/register", s.handleAgentRegister)
	s.mux.HandleFunc("GET /api/agents", s.authed(s.handleAgentList))
	s.mux.HandleFunc("DELETE /api/agents/{name}", s.authed(s.handleAgentRevoke))

	// Identity
	s.mux.HandleFunc("GET /api/whoami", s.authed(s.handleWhoami))

	// Aliases
	s.mux.HandleFunc("POST /api/mail/aliases", s.authed(s.handleAliasCreate))
	s.mux.HandleFunc("GET /api/mail/aliases", s.authed(s.handleAliasList))
	s.mux.HandleFunc("DELETE /api/mail/aliases/{alias}", s.authed(s.handleAliasDelete))

	// Presence
	s.mux.HandleFunc("POST /api/presence/heartbeat", s.authed(s.handleHeartbeat))
	s.mux.HandleFunc("GET /api/presence", s.authed(s.handlePresence))

	// Mail
	s.mux.HandleFunc("POST /api/mail", s.authed(s.handleMailSend))
	s.mux.HandleFunc("GET /api/mail/inbox", s.authed(s.handleMailInbox))
	s.mux.HandleFunc("PATCH /api/mail/{id}/read", s.authed(s.handleMailRead))
	s.mux.HandleFunc("PATCH /api/mail/read-all", s.authed(s.handleMailReadAll))
	s.mux.HandleFunc("GET /api/mail/history", s.authed(s.handleMailHistory))
	s.mux.HandleFunc("GET /api/mail/threads", s.authed(s.handleMailThreads))
	s.mux.HandleFunc("GET /api/mail/status", s.authed(s.handleMailStatus))

	// Rooms
	s.mux.HandleFunc("POST /api/rooms", s.authed(s.handleRoomCreate))
	s.mux.HandleFunc("GET /api/rooms", s.authed(s.handleRoomList))
	s.mux.HandleFunc("GET /api/rooms/{name}", s.authed(s.handleRoomGet))
	s.mux.HandleFunc("POST /api/rooms/{name}/send", s.authed(s.handleRoomSend))
	s.mux.HandleFunc("PATCH /api/rooms/{name}", s.authed(s.handleRoomUpdate))
	s.mux.HandleFunc("DELETE /api/rooms/{name}", s.authed(s.handleRoomArchive))

	// Reservations
	s.mux.HandleFunc("POST /api/reservations", s.authed(s.handleReservationCreate))
	s.mux.HandleFunc("POST /api/reservations/check", s.authed(s.handleReservationCheck))
	s.mux.HandleFunc("GET /api/reservations", s.authed(s.handleReservationList))
	s.mux.HandleFunc("DELETE /api/reservations/{id}", s.authed(s.handleReservationRelease))

	// Checkpoints
	s.mux.HandleFunc("POST /api/checkpoints", s.authed(s.handleCheckpointCreate))
	s.mux.HandleFunc("GET /api/checkpoints", s.authed(s.handleCheckpointList))
	s.mux.HandleFunc("GET /api/checkpoints/{ref}", s.authed(s.handleCheckpointGet))
	s.mux.HandleFunc("PATCH /api/checkpoints/{id}", s.authed(s.handleCheckpointUpdate))

	// Issues (cross-repo index)
	s.mux.HandleFunc("POST /api/issues/push", s.authed(s.handleIssuePush))
	s.mux.HandleFunc("GET /api/issues", s.authed(s.handleIssueList))
	s.mux.HandleFunc("GET /api/issues/stats", s.authed(s.handleIssueStats))

	// WebSocket (auth handled inside handler)
	s.mux.HandleFunc("GET /ws", s.handleWebSocket)
}

// --- Auth ---

func GenerateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return TokenPrefix + hex.EncodeToString(b), nil
}

func HashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *Server) writeLocalToken() error {
	token, err := GenerateToken()
	if err != nil {
		return err
	}
	s.localToken = token

	tokenPath := filepath.Join(s.configDir, ".local-token")
	return os.WriteFile(tokenPath, []byte(token), 0600)
}

func (s *Server) ensureLocalAgent(name, agentType string) {
	ctx := context.Background()
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		return
	}
	for _, a := range agents {
		if a.Name == name {
			return
		}
	}
	tok, err := GenerateToken()
	if err != nil {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(tok), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	_, _ = s.store.CreateAgent(ctx, name, agentType, string(hash))
}

func (s *Server) authenticate(r *http.Request) (string, error) {
	token := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimPrefix(auth, "Bearer ")
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return "", fmt.Errorf("missing authorization")
	}

	if token == s.localToken {
		if !isLoopback(r) {
			return "", fmt.Errorf("local token rejected from remote address %s — register an agent token with POST /api/agents/register", r.RemoteAddr)
		}
		if s.localIdentity != "" {
			return s.localIdentity, nil
		}
		return "_local", nil
	}

	agents, err := s.store.ListAgents(r.Context())
	if err != nil {
		return "", fmt.Errorf("auth lookup failed")
	}

	for _, a := range agents {
		if a.RevokedAt != nil {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(a.TokenHash), []byte(token)) == nil {
			return a.Name, nil
		}
	}

	return "", fmt.Errorf("invalid token")
}

func isLoopback(r *http.Request) bool {
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func (s *Server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent, err := s.authenticate(r)
		if err != nil {
			jsonError(w, err.Error(), http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyAgent, agent)
		next(w, r.WithContext(ctx))
	}
}

type contextKey string

const ctxKeyAgent contextKey = "agent"

func agentFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyAgent).(string); ok {
		return v
	}
	return ""
}

// --- Presence Reaper ---

func (s *Server) presenceReaper(ctx context.Context) {
	defer close(s.reaperDone)
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.store.ReapOfflineAgents(ctx, presenceReap); err == nil && n > 0 {
				log.Printf("reaped %d offline agents", n)
			}
		}
	}
}

// --- Handlers: Health ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":  "ok",
		"service": "binds",
		"listen":  s.server.Addr,
		"version": "0.1.0",
	}
	if s.localIdentity != "" {
		resp["local_identity"] = s.localIdentity
	}
	jsonResp(w, resp)
}

// --- Handlers: Agents ---

func (s *Server) handleAgentRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		AgentType string `json:"agent_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.AgentType == "" {
		jsonError(w, "name and agent_type required", http.StatusBadRequest)
		return
	}

	existing, _ := s.store.GetAgentByName(r.Context(), req.Name)
	if existing != nil && existing.RevokedAt == nil {
		jsonError(w, fmt.Sprintf("agent %q already registered", req.Name), http.StatusConflict)
		return
	}

	token, err := GenerateToken()
	if err != nil {
		jsonError(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	hash, err := HashToken(token)
	if err != nil {
		jsonError(w, "token hashing failed", http.StatusInternalServerError)
		return
	}

	var agent *Agent
	if existing != nil && existing.RevokedAt != nil {
		agent, err = s.store.ReinstateAgent(r.Context(), req.Name, req.AgentType, hash)
	} else {
		agent, err = s.store.CreateAgent(r.Context(), req.Name, req.AgentType, hash)
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, map[string]interface{}{
		"id":         agent.ID,
		"name":       agent.Name,
		"agent_type": agent.AgentType,
		"token":      token,
	})
}

func (s *Server) handleAgentList(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.ListAgents(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(agents))
	for _, a := range agents {
		entry := map[string]interface{}{
			"name":       a.Name,
			"agent_type": a.AgentType,
			"status":     a.Status,
			"workspace":  a.Workspace,
			"created_at": a.CreatedAt.Format(time.RFC3339),
		}
		if a.LastSeen != nil {
			entry["last_seen"] = a.LastSeen.Format(time.RFC3339)
		}
		result = append(result, entry)
	}

	jsonResp(w, map[string]interface{}{"agents": result})
}

func (s *Server) handleAgentRevoke(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.RevokeAgent(r.Context(), name); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonResp(w, map[string]string{"status": "revoked", "name": name})
}

// --- Handlers: Identity ---

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	resp := map[string]string{
		"identity":   agent,
		"server_url": fmt.Sprintf("http://%s", r.Host),
	}
	if agent == s.localIdentity || agent == "_local" {
		resp["token_source"] = "local (.local-token)"
	} else {
		resp["token_source"] = "registered agent token"
	}
	jsonResp(w, resp)
}

// --- Handlers: Aliases ---

func (s *Server) handleAliasCreate(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	var req struct {
		Alias  string `json:"alias"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Alias == "" || req.Target == "" {
		jsonError(w, "alias and target required", http.StatusBadRequest)
		return
	}
	if err := s.store.CreateAlias(r.Context(), req.Alias, req.Target, agent); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"alias": req.Alias, "target": req.Target, "status": "created"})
}

func (s *Server) handleAliasList(w http.ResponseWriter, r *http.Request) {
	aliases, err := s.store.ListAliases(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result := make([]map[string]string, 0, len(aliases))
	for _, a := range aliases {
		result = append(result, map[string]string{
			"alias":      a.AliasName,
			"target":     a.Target,
			"created_by": a.CreatedBy,
			"created_at": a.CreatedAt,
		})
	}
	jsonResp(w, map[string]interface{}{"aliases": result})
}

func (s *Server) handleAliasDelete(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if err := s.store.DeleteAlias(r.Context(), alias); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonResp(w, map[string]string{"alias": alias, "status": "deleted"})
}

// --- Handlers: Presence ---

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	var req struct {
		Workspace string `json:"workspace"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Status == "" {
		req.Status = "online"
	}

	if err := s.store.UpdatePresence(r.Context(), agent, req.Workspace, req.Status); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
	s.hub.Broadcast(&Event{Type: "presence.changed", Payload: map[string]interface{}{"agent": agent, "status": req.Status, "workspace": req.Workspace}})
}

func (s *Server) handlePresence(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.GetOnlineAgents(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(agents))
	for _, a := range agents {
		entry := map[string]interface{}{
			"name":       a.Name,
			"agent_type": a.AgentType,
			"status":     a.Status,
			"workspace":  a.Workspace,
		}
		if a.LastSeen != nil {
			entry["last_seen"] = a.LastSeen.Format(time.RFC3339)
		}
		result = append(result, entry)
	}

	reservations, _ := s.store.ListActiveReservations(r.Context())
	resResult := make([]map[string]interface{}, 0, len(reservations))
	for _, rv := range reservations {
		resResult = append(resResult, map[string]interface{}{
			"id":        rv.ID,
			"holder":    rv.Holder,
			"pattern":   rv.Pattern,
			"reason":    rv.Reason,
			"exclusive": rv.Exclusive,
			"expires":   rv.ExpiresAt.Format(time.RFC3339),
		})
	}

	jsonResp(w, map[string]interface{}{
		"agents":       result,
		"reservations": resResult,
	})
}

// --- Handlers: Mail ---

func (s *Server) handleMailSend(w http.ResponseWriter, r *http.Request) {
	sender := agentFromCtx(r.Context())
	var req struct {
		Recipient string `json:"recipient"`
		Body      string `json:"body"`
		Subject   string `json:"subject"`
		MsgType   string `json:"msg_type"`
		Priority  string `json:"priority"`
		ReplyTo   *int64 `json:"reply_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Recipient == "" || req.Body == "" {
		jsonError(w, "recipient and body required", http.StatusBadRequest)
		return
	}

	msg, err := s.store.SendMessage(r.Context(), sender, req.Recipient, req.Body, req.Subject, req.MsgType, req.Priority, req.ReplyTo)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, messageToJSON(msg))
	s.hub.Broadcast(&Event{Type: "mail.received", Target: req.Recipient, Payload: messageToJSON(msg)})
}

func (s *Server) handleMailInbox(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	since := r.URL.Query().Get("since")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	msgs, err := s.store.GetInbox(r.Context(), agent, unreadOnly, since, limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, map[string]interface{}{"messages": messagesToJSON(msgs)})
}

func (s *Server) handleMailRead(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid message id", http.StatusBadRequest)
		return
	}
	if err := s.store.MarkRead(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "read"})
}

func (s *Server) handleMailReadAll(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	n, err := s.store.MarkAllRead(r.Context(), agent)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]interface{}{"marked": n})
}

func (s *Server) handleMailHistory(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	with := r.URL.Query().Get("with")
	if with == "" {
		jsonError(w, "with parameter required", http.StatusBadRequest)
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	msgs, err := s.store.GetHistory(r.Context(), agent, with, limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]interface{}{"messages": messagesToJSON(msgs)})
}

func (s *Server) handleMailThreads(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	msgs, err := s.store.GetThreads(r.Context(), agent)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]interface{}{"messages": messagesToJSON(msgs)})
}

func (s *Server) handleMailStatus(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	total, unread, err := s.store.GetMailStatus(r.Context(), agent)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]interface{}{"total": total, "unread": unread})
}

// --- Handlers: Rooms ---

func (s *Server) handleRoomCreate(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	var req struct {
		Name  string `json:"name"`
		Topic string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name required", http.StatusBadRequest)
		return
	}

	room, err := s.store.CreateRoom(r.Context(), req.Name, req.Topic, agent)
	if err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}
	jsonResp(w, roomToJSON(room))
}

func (s *Server) handleRoomList(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.store.ListRooms(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(rooms))
	for _, rm := range rooms {
		result = append(result, roomToJSON(rm))
	}
	jsonResp(w, map[string]interface{}{"rooms": result})
}

func (s *Server) handleRoomGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	room, err := s.store.GetRoom(r.Context(), name)
	if err != nil {
		jsonError(w, fmt.Sprintf("room %q not found", name), http.StatusNotFound)
		return
	}

	since := r.URL.Query().Get("since")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	msgs, err := s.store.GetRoomMessages(r.Context(), name, since, limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := roomToJSON(room)
	resp["messages"] = messagesToJSON(msgs)
	jsonResp(w, resp)
}

func (s *Server) handleRoomSend(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	name := r.PathValue("name")
	var req struct {
		Body    string `json:"body"`
		ReplyTo *int64 `json:"reply_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Body == "" {
		jsonError(w, "body required", http.StatusBadRequest)
		return
	}

	msg, err := s.store.PostRoomMessage(r.Context(), name, agent, req.Body, req.ReplyTo)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResp(w, messageToJSON(msg))
	s.hub.Broadcast(&Event{Type: "room.message", Payload: map[string]interface{}{"room": name, "message": messageToJSON(msg)}})
}

func (s *Server) handleRoomUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Topic string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.store.UpdateRoomTopic(r.Context(), name, req.Topic); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonResp(w, map[string]string{"status": "updated"})
}

func (s *Server) handleRoomArchive(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.ArchiveRoom(r.Context(), name); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	jsonResp(w, map[string]string{"status": "archived"})
}

// --- Handlers: Reservations ---

func (s *Server) handleReservationCreate(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	var req struct {
		Patterns  []string `json:"patterns"`
		Reason    string   `json:"reason"`
		TTL       string   `json:"ttl"`
		Exclusive *bool    `json:"exclusive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Patterns) == 0 {
		jsonError(w, "patterns required", http.StatusBadRequest)
		return
	}

	ttl := 30 * time.Minute
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil {
			ttl = d
		}
	}

	exclusive := true
	if req.Exclusive != nil {
		exclusive = *req.Exclusive
	}

	conflicts, _ := s.store.CheckConflicts(r.Context(), req.Patterns, agent)
	if len(conflicts) > 0 {
		conflictData := make([]map[string]interface{}, 0, len(conflicts))
		for _, c := range conflicts {
			conflictData = append(conflictData, map[string]interface{}{
				"id":      c.ID,
				"holder":  c.Holder,
				"pattern": c.Pattern,
				"reason":  c.Reason,
			})
		}
		jsonResp(w, map[string]interface{}{
			"status":    "conflict",
			"conflicts": conflictData,
		})
		return
	}

	var created []map[string]interface{}
	for _, p := range req.Patterns {
		rv, err := s.store.CreateReservation(r.Context(), agent, p, req.Reason, exclusive, ttl)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		created = append(created, map[string]interface{}{
			"id":         rv.ID,
			"pattern":    rv.Pattern,
			"expires_at": rv.ExpiresAt.Format(time.RFC3339),
		})
	}

	jsonResp(w, map[string]interface{}{"status": "reserved", "reservations": created})
	s.hub.Broadcast(&Event{Type: "reservation.created", Payload: map[string]interface{}{"agent": agent, "patterns": req.Patterns}})
}

func (s *Server) handleReservationCheck(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	var req struct {
		Patterns []string `json:"patterns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	conflicts, err := s.store.CheckConflicts(r.Context(), req.Patterns, agent)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	conflictData := make([]map[string]interface{}, 0, len(conflicts))
	for _, c := range conflicts {
		conflictData = append(conflictData, map[string]interface{}{
			"id":      c.ID,
			"holder":  c.Holder,
			"pattern": c.Pattern,
			"reason":  c.Reason,
		})
	}

	jsonResp(w, map[string]interface{}{"conflicts": conflictData, "clear": len(conflicts) == 0})
}

func (s *Server) handleReservationList(w http.ResponseWriter, r *http.Request) {
	reservations, err := s.store.ListActiveReservations(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(reservations))
	for _, rv := range reservations {
		result = append(result, map[string]interface{}{
			"id":         rv.ID,
			"holder":     rv.Holder,
			"pattern":    rv.Pattern,
			"exclusive":  rv.Exclusive,
			"reason":     rv.Reason,
			"created_at": rv.CreatedAt.Format(time.RFC3339),
			"expires_at": rv.ExpiresAt.Format(time.RFC3339),
		})
	}

	jsonResp(w, map[string]interface{}{"reservations": result})
}

func (s *Server) handleReservationRelease(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid reservation id", http.StatusBadRequest)
		return
	}
	if err := s.store.ReleaseReservation(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "released"})
	s.hub.Broadcast(&Event{Type: "reservation.released", Payload: map[string]interface{}{"id": id}})
}

// --- Handlers: Checkpoints ---

func (s *Server) handleCheckpointCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		WorkspaceID string `json:"workspace_id"`
		BindsIDs    string `json:"binds_ids"`
		Slug        string `json:"slug"`
		ParentID    *int64 `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		jsonError(w, "title required", http.StatusBadRequest)
		return
	}

	cp, err := s.store.CreateCheckpoint(r.Context(), req.Title, req.Description, req.Priority, req.WorkspaceID, req.BindsIDs, req.Slug, req.ParentID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, checkpointToJSON(cp))
}

func (s *Server) handleCheckpointList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	checkpoints, err := s.store.ListCheckpoints(r.Context(), status)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(checkpoints))
	for _, cp := range checkpoints {
		result = append(result, checkpointToJSON(cp))
	}
	jsonResp(w, map[string]interface{}{"checkpoints": result})
}

func (s *Server) handleCheckpointGet(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("ref")
	id, err := strconv.ParseInt(ref, 10, 64)

	var cp *Checkpoint
	if err == nil {
		cp, err = s.store.GetCheckpoint(r.Context(), id)
	} else {
		cp, err = s.store.GetCheckpointBySlug(r.Context(), ref)
	}

	if err != nil {
		jsonError(w, fmt.Sprintf("checkpoint %q not found", ref), http.StatusNotFound)
		return
	}
	jsonResp(w, checkpointToJSON(cp))
}

func (s *Server) handleCheckpointUpdate(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("id")
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		cp, slugErr := s.store.GetCheckpointBySlug(r.Context(), ref)
		if slugErr != nil {
			jsonError(w, fmt.Sprintf("checkpoint %q not found", ref), http.StatusNotFound)
			return
		}
		id = cp.ID
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		var req struct {
			Status string `json:"status"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		status = req.Status
	}
	if status == "" {
		jsonError(w, "status required", http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateCheckpointStatus(r.Context(), id, status); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.hub.Broadcast(&Event{
		Type:    "checkpoint.updated",
		Payload: map[string]interface{}{"id": id, "status": status},
	})

	jsonResp(w, map[string]interface{}{"status": status, "id": id})
}

// --- JSON helpers ---

func jsonResp(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func messageToJSON(m *Message) map[string]interface{} {
	result := map[string]interface{}{
		"id":         m.ID,
		"sender":     m.Sender,
		"recipient":  m.Recipient,
		"msg_type":   m.MsgType,
		"body":       m.Body,
		"priority":   m.Priority,
		"is_read":    m.IsRead,
		"created_at": m.CreatedAt.Format(time.RFC3339),
	}
	if m.Subject != "" {
		result["subject"] = m.Subject
	}
	if m.RoomID != nil {
		result["room_id"] = *m.RoomID
	}
	if m.ReplyTo != nil {
		result["reply_to"] = *m.ReplyTo
	}
	if m.ThreadID != nil {
		result["thread_id"] = *m.ThreadID
	}
	if m.ReadAt != nil {
		result["read_at"] = m.ReadAt.Format(time.RFC3339)
	}
	return result
}

func messagesToJSON(msgs []*Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, messageToJSON(m))
	}
	return result
}

func checkpointToJSON(c *Checkpoint) map[string]interface{} {
	result := map[string]interface{}{
		"id":         c.ID,
		"title":      c.Title,
		"priority":   c.Priority,
		"status":     c.Status,
		"created_at": c.CreatedAt.Format(time.RFC3339),
		"updated_at": c.UpdatedAt.Format(time.RFC3339),
	}
	if c.ParentID != nil {
		result["parent_id"] = *c.ParentID
	}
	if c.Description != "" {
		result["description"] = c.Description
	}
	if c.WorkspaceID != "" {
		result["workspace_id"] = c.WorkspaceID
	}
	if c.BindsIDs != "" {
		result["binds_ids"] = c.BindsIDs
	}
	if c.Slug != "" {
		result["slug"] = c.Slug
	}
	if c.CompletedAt != nil {
		result["completed_at"] = c.CompletedAt.Format(time.RFC3339)
	}
	return result
}

func roomToJSON(r *Room) map[string]interface{} {
	result := map[string]interface{}{
		"id":         r.ID,
		"name":       r.Name,
		"topic":      r.Topic,
		"created_by": r.CreatedBy,
		"created_at": r.CreatedAt.Format(time.RFC3339),
	}
	if r.ArchivedAt != nil {
		result["archived_at"] = r.ArchivedAt.Format(time.RFC3339)
	}
	return result
}

// --- Issue Index Handlers ---

func (s *Server) handleIssuePush(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r.Context())
	var req struct {
		Issues []struct {
			Workspace     string `json:"workspace"`
			WorkspacePath string `json:"workspace_path"`
			IssueID       string `json:"issue_id"`
			Title         string `json:"title"`
			Status        string `json:"status"`
			Priority      int    `json:"priority"`
			IssueType     string `json:"issue_type"`
			Assignee      string `json:"assignee"`
			Labels        string `json:"labels"`
			CreatedAt     string `json:"created_at"`
			UpdatedAt     string `json:"updated_at"`
			ClosedAt      string `json:"closed_at"`
		} `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	pushed := 0
	for _, issue := range req.Issues {
		if issue.IssueID == "" || issue.Workspace == "" {
			continue
		}
		err := s.store.UpsertIssue(r.Context(), &Issue{
			Workspace:     issue.Workspace,
			WorkspacePath: issue.WorkspacePath,
			IssueID:       issue.IssueID,
			Title:         issue.Title,
			Status:        issue.Status,
			Priority:      issue.Priority,
			IssueType:     issue.IssueType,
			Assignee:      issue.Assignee,
			Labels:        issue.Labels,
			CreatedAt:     issue.CreatedAt,
			UpdatedAt:     issue.UpdatedAt,
			ClosedAt:      issue.ClosedAt,
			PushedBy:      agent,
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pushed++
	}

	jsonResp(w, map[string]interface{}{"pushed": pushed})
	if pushed > 0 {
		s.hub.Broadcast(&Event{Type: "issues.synced", Payload: map[string]interface{}{"count": pushed, "agent": agent}})
	}
}

func (s *Server) handleIssueList(w http.ResponseWriter, r *http.Request) {
	workspace := r.URL.Query().Get("workspace")
	status := r.URL.Query().Get("status")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	issues, err := s.store.ListIssues(r.Context(), workspace, status, limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(issues))
	for _, i := range issues {
		entry := map[string]interface{}{
			"workspace":      i.Workspace,
			"workspace_path": i.WorkspacePath,
			"issue_id":       i.IssueID,
			"title":          i.Title,
			"status":         i.Status,
			"priority":       i.Priority,
			"issue_type":     i.IssueType,
			"created_at":     i.CreatedAt,
			"updated_at":     i.UpdatedAt,
			"pushed_by":      i.PushedBy,
			"pushed_at":      i.PushedAt,
		}
		if i.Assignee != "" {
			entry["assignee"] = i.Assignee
		}
		if i.Labels != "" {
			entry["labels"] = i.Labels
		}
		if i.ClosedAt != "" {
			entry["closed_at"] = i.ClosedAt
		}
		result = append(result, entry)
	}

	jsonResp(w, map[string]interface{}{"issues": result, "count": len(result)})
}

func (s *Server) handleIssueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.IssueStats(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, stats)
}
