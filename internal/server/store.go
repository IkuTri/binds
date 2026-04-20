package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type Store struct {
	db   *sql.DB
	path string
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal=wal&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open server db: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate server db: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS agents (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    agent_type  TEXT NOT NULL,
    token_hash  TEXT NOT NULL,
    workspace   TEXT,
    last_seen   TEXT,
    status      TEXT DEFAULT 'offline',
    metadata    TEXT,
    created_at  TEXT NOT NULL,
    revoked_at  TEXT
);

CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    sender      TEXT NOT NULL,
    recipient   TEXT NOT NULL,
    room_id     INTEGER REFERENCES rooms(id),
    msg_type    TEXT DEFAULT 'text',
    subject     TEXT,
    body        TEXT NOT NULL,
    metadata    TEXT,
    priority    TEXT DEFAULT 'normal',
    reply_to    INTEGER REFERENCES messages(id),
    thread_id   INTEGER,
    is_read     INTEGER DEFAULT 0,
    created_at  TEXT NOT NULL,
    read_at     TEXT
);

CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);
CREATE INDEX IF NOT EXISTS idx_messages_room_id ON messages(room_id);
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

CREATE TABLE IF NOT EXISTS rooms (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    topic       TEXT,
    created_by  TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    archived_at TEXT
);

CREATE TABLE IF NOT EXISTS reservations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    holder      TEXT NOT NULL,
    pattern     TEXT NOT NULL,
    exclusive   INTEGER DEFAULT 1,
    reason      TEXT,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    released_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_reservations_holder ON reservations(holder);
CREATE INDEX IF NOT EXISTS idx_reservations_expires_at ON reservations(expires_at);

CREATE TABLE IF NOT EXISTS issues (
    workspace      TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    issue_id       TEXT NOT NULL,
    title          TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'open',
    priority       INTEGER DEFAULT 2,
    issue_type     TEXT DEFAULT 'task',
    assignee       TEXT,
    labels         TEXT,
    created_at     TEXT,
    updated_at     TEXT,
    closed_at      TEXT,
    pushed_by      TEXT,
    pushed_at      TEXT NOT NULL,
    PRIMARY KEY (workspace, issue_id)
);

CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);
CREATE INDEX IF NOT EXISTS idx_issues_workspace ON issues(workspace);

CREATE TABLE IF NOT EXISTS checkpoints (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id    INTEGER REFERENCES checkpoints(id),
    title        TEXT NOT NULL,
    description  TEXT,
    priority     TEXT DEFAULT 'P2',
    status       TEXT DEFAULT 'pending',
    workspace_id TEXT,
    binds_ids    TEXT,
    slug         TEXT UNIQUE,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    completed_at TEXT
);
`

// --- Agent CRUD ---

type Agent struct {
	ID        int64
	Name      string
	AgentType string
	TokenHash string
	Workspace string
	LastSeen  *time.Time
	Status    string
	Metadata  string
	CreatedAt time.Time
	RevokedAt *time.Time
}

func (s *Store) CreateAgent(ctx context.Context, name, agentType, tokenHash string) (*Agent, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO agents (name, agent_type, token_hash, status, created_at) VALUES (?, ?, ?, 'offline', ?)`,
		name, agentType, tokenHash, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Agent{ID: id, Name: name, AgentType: agentType, TokenHash: tokenHash, Status: "offline"}, nil
}

func (s *Store) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	return s.scanAgent(s.db.QueryRowContext(ctx,
		`SELECT id, name, agent_type, token_hash, workspace, last_seen, status, metadata, created_at, revoked_at FROM agents WHERE name = ?`, name))
}

func (s *Store) ListAgents(ctx context.Context) ([]*Agent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, agent_type, token_hash, workspace, last_seen, status, metadata, created_at, revoked_at FROM agents WHERE revoked_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		a, err := s.scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *Store) RevokeAgent(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE agents SET revoked_at = ?, status = 'offline' WHERE name = ? AND revoked_at IS NULL`, now, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %q not found or already revoked", name)
	}
	return nil
}

func (s *Store) UpdatePresence(ctx context.Context, name, workspace, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE agents SET last_seen = ?, workspace = ?, status = ? WHERE name = ? AND revoked_at IS NULL`,
		now, workspace, status, name)
	return err
}

func (s *Store) GetOnlineAgents(ctx context.Context) ([]*Agent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, agent_type, token_hash, workspace, last_seen, status, metadata, created_at, revoked_at FROM agents WHERE status != 'offline' AND revoked_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		a, err := s.scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *Store) ReapOfflineAgents(ctx context.Context, timeout time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-timeout).Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE agents SET status = 'offline' WHERE status != 'offline' AND last_seen < ? AND revoked_at IS NULL`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (s *Store) scanAgent(row scanner) (*Agent, error) {
	var a Agent
	var lastSeen, revokedAt, createdAt sql.NullString
	var workspace, metadata sql.NullString

	err := row.Scan(&a.ID, &a.Name, &a.AgentType, &a.TokenHash, &workspace, &lastSeen, &a.Status, &metadata, &createdAt, &revokedAt)
	if err != nil {
		return nil, err
	}

	a.Workspace = workspace.String
	a.Metadata = metadata.String
	if createdAt.Valid {
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if lastSeen.Valid {
		t, _ := time.Parse(time.RFC3339, lastSeen.String)
		a.LastSeen = &t
	}
	if revokedAt.Valid {
		t, _ := time.Parse(time.RFC3339, revokedAt.String)
		a.RevokedAt = &t
	}

	return &a, nil
}

// --- Mail CRUD ---

type Message struct {
	ID        int64
	Sender    string
	Recipient string
	RoomID    *int64
	MsgType   string
	Subject   string
	Body      string
	Metadata  string
	Priority  string
	ReplyTo   *int64
	ThreadID  *int64
	IsRead    bool
	CreatedAt time.Time
	ReadAt    *time.Time
}

func (s *Store) SendMessage(ctx context.Context, sender, recipient, body, subject, msgType, priority string, replyTo *int64) (*Message, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if msgType == "" {
		msgType = "text"
	}
	if priority == "" {
		priority = "normal"
	}

	var threadID *int64
	if replyTo != nil {
		var tid sql.NullInt64
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(thread_id, id) FROM messages WHERE id = ?`, *replyTo).Scan(&tid)
		if tid.Valid {
			threadID = &tid.Int64
		}
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (sender, recipient, msg_type, subject, body, priority, reply_to, thread_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sender, recipient, msgType, subject, body, priority, replyTo, threadID, now)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	createdAt, _ := time.Parse(time.RFC3339, now)
	return &Message{ID: id, Sender: sender, Recipient: recipient, MsgType: msgType, Subject: subject, Body: body, Priority: priority, ReplyTo: replyTo, ThreadID: threadID, CreatedAt: createdAt}, nil
}

func (s *Store) GetInbox(ctx context.Context, recipient string, unreadOnly bool, since string, limit int) ([]*Message, error) {
	query := `SELECT id, sender, recipient, room_id, msg_type, subject, body, metadata, priority, reply_to, thread_id, is_read, created_at, read_at FROM messages WHERE recipient = ? AND room_id IS NULL`
	args := []interface{}{recipient}

	if unreadOnly {
		query += ` AND is_read = 0`
	}
	if since != "" {
		query += ` AND created_at > ?`
		args = append(args, since)
	}

	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	return s.queryMessages(ctx, query, args...)
}

func (s *Store) MarkRead(ctx context.Context, id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET is_read = 1, read_at = ? WHERE id = ?`, now, id)
	return err
}

func (s *Store) MarkAllRead(ctx context.Context, recipient string) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `UPDATE messages SET is_read = 1, read_at = ? WHERE recipient = ? AND is_read = 0 AND room_id IS NULL`, now, recipient)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *Store) GetHistory(ctx context.Context, agent1, agent2 string, limit int) ([]*Message, error) {
	query := `SELECT id, sender, recipient, room_id, msg_type, subject, body, metadata, priority, reply_to, thread_id, is_read, created_at, read_at FROM messages WHERE room_id IS NULL AND ((sender = ? AND recipient = ?) OR (sender = ? AND recipient = ?)) ORDER BY created_at DESC`
	args := []interface{}{agent1, agent2, agent2, agent1}
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}
	return s.queryMessages(ctx, query, args...)
}

func (s *Store) GetThreads(ctx context.Context, recipient string) ([]*Message, error) {
	return s.queryMessages(ctx,
		`SELECT DISTINCT m.id, m.sender, m.recipient, m.room_id, m.msg_type, m.subject, m.body, m.metadata, m.priority, m.reply_to, m.thread_id, m.is_read, m.created_at, m.read_at FROM messages m WHERE m.thread_id IS NOT NULL AND (m.sender = ? OR m.recipient = ?) AND m.room_id IS NULL ORDER BY m.created_at DESC`,
		recipient, recipient)
}

func (s *Store) GetMailStatus(ctx context.Context, recipient string) (total, unread int, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(CASE WHEN is_read = 0 THEN 1 END) FROM messages WHERE recipient = ? AND room_id IS NULL`, recipient).Scan(&total, &unread)
	return
}

func (s *Store) queryMessages(ctx context.Context, query string, args ...interface{}) ([]*Message, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*Message
	for rows.Next() {
		m, err := s.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (s *Store) scanMessage(row scanner) (*Message, error) {
	var m Message
	var roomID, replyTo, threadID sql.NullInt64
	var subject, metadata, readAt sql.NullString
	var createdAt string

	err := row.Scan(&m.ID, &m.Sender, &m.Recipient, &roomID, &m.MsgType, &subject, &m.Body, &metadata, &m.Priority, &replyTo, &threadID, &m.IsRead, &createdAt, &readAt)
	if err != nil {
		return nil, err
	}

	m.Subject = subject.String
	m.Metadata = metadata.String
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if roomID.Valid {
		m.RoomID = &roomID.Int64
	}
	if replyTo.Valid {
		m.ReplyTo = &replyTo.Int64
	}
	if threadID.Valid {
		m.ThreadID = &threadID.Int64
	}
	if readAt.Valid {
		t, _ := time.Parse(time.RFC3339, readAt.String)
		m.ReadAt = &t
	}
	return &m, nil
}

// --- Room CRUD ---

type Room struct {
	ID         int64
	Name       string
	Topic      string
	CreatedBy  string
	CreatedAt  time.Time
	ArchivedAt *time.Time
}

func (s *Store) CreateRoom(ctx context.Context, name, topic, createdBy string) (*Room, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO rooms (name, topic, created_by, created_at) VALUES (?, ?, ?, ?)`,
		name, topic, createdBy, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	createdAt, _ := time.Parse(time.RFC3339, now)
	return &Room{ID: id, Name: name, Topic: topic, CreatedBy: createdBy, CreatedAt: createdAt}, nil
}

func (s *Store) ListRooms(ctx context.Context) ([]*Room, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, topic, created_by, created_at, archived_at FROM rooms WHERE archived_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*Room
	for rows.Next() {
		r, err := s.scanRoom(rows)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func (s *Store) GetRoom(ctx context.Context, name string) (*Room, error) {
	return s.scanRoom(s.db.QueryRowContext(ctx,
		`SELECT id, name, topic, created_by, created_at, archived_at FROM rooms WHERE name = ?`, name))
}

func (s *Store) PostRoomMessage(ctx context.Context, roomName, sender, body string, replyTo *int64) (*Message, error) {
	room, err := s.GetRoom(ctx, roomName)
	if err != nil {
		return nil, fmt.Errorf("room %q not found", roomName)
	}
	if room.ArchivedAt != nil {
		return nil, fmt.Errorf("room %q is archived", roomName)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var threadID *int64
	if replyTo != nil {
		var tid sql.NullInt64
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(thread_id, id) FROM messages WHERE id = ?`, *replyTo).Scan(&tid)
		if tid.Valid {
			threadID = &tid.Int64
		}
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO messages (sender, recipient, room_id, msg_type, body, reply_to, thread_id, created_at) VALUES (?, 'room', ?, 'text', ?, ?, ?, ?)`,
		sender, room.ID, body, replyTo, threadID, now)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	createdAt, _ := time.Parse(time.RFC3339, now)
	return &Message{ID: id, Sender: sender, Recipient: "room", RoomID: &room.ID, Body: body, ReplyTo: replyTo, ThreadID: threadID, CreatedAt: createdAt}, nil
}

func (s *Store) GetRoomMessages(ctx context.Context, roomName string, since string, limit int) ([]*Message, error) {
	room, err := s.GetRoom(ctx, roomName)
	if err != nil {
		return nil, fmt.Errorf("room %q not found", roomName)
	}

	query := `SELECT id, sender, recipient, room_id, msg_type, subject, body, metadata, priority, reply_to, thread_id, is_read, created_at, read_at FROM messages WHERE room_id = ?`
	args := []interface{}{room.ID}

	if since != "" {
		query += ` AND created_at > ?`
		args = append(args, since)
	}

	query += ` ORDER BY created_at ASC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	return s.queryMessages(ctx, query, args...)
}

func (s *Store) UpdateRoomTopic(ctx context.Context, name, topic string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE rooms SET topic = ? WHERE name = ? AND archived_at IS NULL`, topic, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("room %q not found or archived", name)
	}
	return nil
}

func (s *Store) ArchiveRoom(ctx context.Context, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `UPDATE rooms SET archived_at = ? WHERE name = ? AND archived_at IS NULL`, now, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("room %q not found or already archived", name)
	}
	return nil
}

func (s *Store) scanRoom(row scanner) (*Room, error) {
	var r Room
	var topic sql.NullString
	var createdAt string
	var archivedAt sql.NullString

	err := row.Scan(&r.ID, &r.Name, &topic, &r.CreatedBy, &createdAt, &archivedAt)
	if err != nil {
		return nil, err
	}

	r.Topic = topic.String
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if archivedAt.Valid {
		t, _ := time.Parse(time.RFC3339, archivedAt.String)
		r.ArchivedAt = &t
	}
	return &r, nil
}

// --- Reservation CRUD ---

type Reservation struct {
	ID         int64
	Holder     string
	Pattern    string
	Exclusive  bool
	Reason     string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ReleasedAt *time.Time
}

func (s *Store) CreateReservation(ctx context.Context, holder, pattern, reason string, exclusive bool, ttl time.Duration) (*Reservation, error) {
	now := time.Now().UTC()
	expires := now.Add(ttl)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO reservations (holder, pattern, exclusive, reason, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		holder, pattern, exclusive, reason, now.Format(time.RFC3339), expires.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return &Reservation{ID: id, Holder: holder, Pattern: pattern, Exclusive: exclusive, Reason: reason, CreatedAt: now, ExpiresAt: expires}, nil
}

func (s *Store) CheckConflicts(ctx context.Context, patterns []string, requestor string) ([]*Reservation, error) {
	active, err := s.ListActiveReservations(ctx)
	if err != nil {
		return nil, err
	}

	var conflicts []*Reservation
	for _, r := range active {
		if r.Holder == requestor {
			continue
		}
		for _, p := range patterns {
			if matchGlob(r.Pattern, p) || matchGlob(p, r.Pattern) {
				conflicts = append(conflicts, r)
				break
			}
		}
	}
	return conflicts, nil
}

func (s *Store) ReleaseReservation(ctx context.Context, id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE reservations SET released_at = ? WHERE id = ? AND released_at IS NULL`, now, id)
	return err
}

func (s *Store) ListActiveReservations(ctx context.Context) ([]*Reservation, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, holder, pattern, exclusive, reason, created_at, expires_at, released_at FROM reservations WHERE released_at IS NULL AND expires_at > ? ORDER BY created_at`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reservations []*Reservation
	for rows.Next() {
		r, err := s.scanReservation(rows)
		if err != nil {
			return nil, err
		}
		reservations = append(reservations, r)
	}
	return reservations, rows.Err()
}

func (s *Store) scanReservation(row scanner) (*Reservation, error) {
	var r Reservation
	var reason sql.NullString
	var createdAt, expiresAt string
	var releasedAt sql.NullString
	var exclusive int

	err := row.Scan(&r.ID, &r.Holder, &r.Pattern, &exclusive, &reason, &createdAt, &expiresAt, &releasedAt)
	if err != nil {
		return nil, err
	}

	r.Exclusive = exclusive == 1
	r.Reason = reason.String
	r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	r.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	if releasedAt.Valid {
		t, _ := time.Parse(time.RFC3339, releasedAt.String)
		r.ReleasedAt = &t
	}
	return &r, nil
}

func matchGlob(pattern, name string) bool {
	matched, _ := filepath.Match(pattern, name)
	return matched
}

// --- Checkpoint CRUD ---

type Checkpoint struct {
	ID          int64
	ParentID    *int64
	Title       string
	Description string
	Priority    string
	Status      string
	WorkspaceID string
	BindsIDs    string
	Slug        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

func (s *Store) CreateCheckpoint(ctx context.Context, title, description, priority, workspaceID, bindsIDs, slug string, parentID *int64) (*Checkpoint, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if priority == "" {
		priority = "P2"
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO checkpoints (parent_id, title, description, priority, status, workspace_id, binds_ids, slug, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?)`,
		parentID, title, description, priority, workspaceID, bindsIDs, slug, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	createdAt, _ := time.Parse(time.RFC3339, now)
	return &Checkpoint{
		ID: id, ParentID: parentID, Title: title, Description: description,
		Priority: priority, Status: "pending", WorkspaceID: workspaceID,
		BindsIDs: bindsIDs, Slug: slug, CreatedAt: createdAt, UpdatedAt: createdAt,
	}, nil
}

func (s *Store) ListCheckpoints(ctx context.Context, status string) ([]*Checkpoint, error) {
	query := `SELECT id, parent_id, title, description, priority, status, workspace_id, binds_ids, slug, created_at, updated_at, completed_at FROM checkpoints`
	var args []interface{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checkpoints []*Checkpoint
	for rows.Next() {
		c, err := s.scanCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		checkpoints = append(checkpoints, c)
	}
	return checkpoints, rows.Err()
}

func (s *Store) GetCheckpoint(ctx context.Context, id int64) (*Checkpoint, error) {
	return s.scanCheckpoint(s.db.QueryRowContext(ctx,
		`SELECT id, parent_id, title, description, priority, status, workspace_id, binds_ids, slug, created_at, updated_at, completed_at FROM checkpoints WHERE id = ?`, id))
}

func (s *Store) GetCheckpointBySlug(ctx context.Context, slug string) (*Checkpoint, error) {
	return s.scanCheckpoint(s.db.QueryRowContext(ctx,
		`SELECT id, parent_id, title, description, priority, status, workspace_id, binds_ids, slug, created_at, updated_at, completed_at FROM checkpoints WHERE slug = ?`, slug))
}

func (s *Store) UpdateCheckpointStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var completedAt interface{}
	if status == "completed" {
		completedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE checkpoints SET status = ?, updated_at = ?, completed_at = COALESCE(?, completed_at) WHERE id = ?`,
		status, now, completedAt, id)
	return err
}

func (s *Store) scanCheckpoint(row scanner) (*Checkpoint, error) {
	var c Checkpoint
	var parentID sql.NullInt64
	var description, workspaceID, bindsIDs, slug sql.NullString
	var createdAt, updatedAt string
	var completedAt sql.NullString

	err := row.Scan(&c.ID, &parentID, &c.Title, &description, &c.Priority, &c.Status,
		&workspaceID, &bindsIDs, &slug, &createdAt, &updatedAt, &completedAt)
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		c.ParentID = &parentID.Int64
	}
	c.Description = description.String
	c.WorkspaceID = workspaceID.String
	c.BindsIDs = bindsIDs.String
	c.Slug = slug.String
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		c.CompletedAt = &t
	}
	return &c, nil
}

// --- Issue Index (cross-repo) ---

type Issue struct {
	Workspace     string
	WorkspacePath string
	IssueID       string
	Title         string
	Status        string
	Priority      int
	IssueType     string
	Assignee      string
	Labels        string
	CreatedAt     string
	UpdatedAt     string
	ClosedAt      string
	PushedBy      string
	PushedAt      string
}

func (s *Store) UpsertIssue(ctx context.Context, issue *Issue) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO issues (workspace, workspace_path, issue_id, title, status, priority, issue_type, assignee, labels, created_at, updated_at, closed_at, pushed_by, pushed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace, issue_id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			priority = excluded.priority,
			issue_type = excluded.issue_type,
			assignee = excluded.assignee,
			labels = excluded.labels,
			updated_at = excluded.updated_at,
			closed_at = excluded.closed_at,
			pushed_by = excluded.pushed_by,
			pushed_at = excluded.pushed_at,
			workspace_path = excluded.workspace_path`,
		issue.Workspace, issue.WorkspacePath, issue.IssueID, issue.Title, issue.Status, issue.Priority, issue.IssueType, issue.Assignee, issue.Labels, issue.CreatedAt, issue.UpdatedAt, issue.ClosedAt, issue.PushedBy, now)
	return err
}

func (s *Store) ListIssues(ctx context.Context, workspace, status string, limit int) ([]*Issue, error) {
	query := `SELECT workspace, workspace_path, issue_id, title, status, priority, issue_type, assignee, labels, created_at, updated_at, closed_at, pushed_by, pushed_at FROM issues WHERE 1=1`
	args := []interface{}{}

	if workspace != "" {
		query += ` AND workspace = ?`
		args = append(args, workspace)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY priority ASC, updated_at DESC`

	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		var i Issue
		var closedAt, assignee, labels sql.NullString
		if err := rows.Scan(&i.Workspace, &i.WorkspacePath, &i.IssueID, &i.Title, &i.Status, &i.Priority, &i.IssueType, &assignee, &labels, &i.CreatedAt, &i.UpdatedAt, &closedAt, &i.PushedBy, &i.PushedAt); err != nil {
			return nil, err
		}
		if closedAt.Valid {
			i.ClosedAt = closedAt.String
		}
		if assignee.Valid {
			i.Assignee = assignee.String
		}
		if labels.Valid {
			i.Labels = labels.String
		}
		issues = append(issues, &i)
	}
	return issues, nil
}

func (s *Store) GetIssue(ctx context.Context, workspace, issueID string) (*Issue, error) {
	var i Issue
	var closedAt, assignee, labels sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT workspace, workspace_path, issue_id, title, status, priority, issue_type, assignee, labels, created_at, updated_at, closed_at, pushed_by, pushed_at FROM issues WHERE workspace = ? AND issue_id = ?`,
		workspace, issueID).Scan(&i.Workspace, &i.WorkspacePath, &i.IssueID, &i.Title, &i.Status, &i.Priority, &i.IssueType, &assignee, &labels, &i.CreatedAt, &i.UpdatedAt, &closedAt, &i.PushedBy, &i.PushedAt)
	if err != nil {
		return nil, err
	}
	if closedAt.Valid {
		i.ClosedAt = closedAt.String
	}
	if assignee.Valid {
		i.Assignee = assignee.String
	}
	if labels.Valid {
		i.Labels = labels.String
	}
	return &i, nil
}

func (s *Store) IssueStats(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT workspace, COUNT(*) as total,
			SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END) as open_count,
			SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END) as in_progress,
			SUM(CASE WHEN status = 'closed' THEN 1 ELSE 0 END) as closed
		FROM issues GROUP BY workspace`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[string]int{}
	total := 0
	for rows.Next() {
		var ws string
		var t, o, ip, c int
		rows.Scan(&ws, &t, &o, &ip, &c)
		stats[ws+"_total"] = t
		stats[ws+"_open"] = o
		stats[ws+"_in_progress"] = ip
		stats[ws+"_closed"] = c
		total += t
	}
	stats["total"] = total
	return stats, nil
}
