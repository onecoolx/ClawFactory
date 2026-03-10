package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	_ "modernc.org/sqlite"
)

// SQLiteStore is the SQLite-based StateStore implementation.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates and initializes a SQLite store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Enable WAL mode and foreign key constraints
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init tables: %w", err)
	}
	return s, nil
}

// DB returns the underlying database connection (for components that need direct queries).
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) initTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			agent_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			capabilities TEXT NOT NULL,
			version TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'online',
			roles TEXT NOT NULL DEFAULT '[]',
			last_heartbeat DATETIME,
			registered_at DATETIME NOT NULL,
			UNIQUE(name, version)
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_definitions (
			definition_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			definition_json TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_instances (
			instance_id TEXT PRIMARY KEY,
			definition_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (definition_id) REFERENCES workflow_definitions(definition_id)
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			task_id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			type TEXT NOT NULL,
			capabilities TEXT NOT NULL,
			input TEXT DEFAULT '{}',
			output TEXT DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'pending',
			priority INTEGER NOT NULL DEFAULT 0,
			assigned_to TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			error TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (workflow_id) REFERENCES workflow_instances(instance_id)
		)`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			agent_id TEXT NOT NULL,
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			allowed BOOLEAN NOT NULL,
			reason TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tool_rate_limits (
			agent_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			window_start DATETIME NOT NULL,
			call_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (agent_id, tool_name, window_start)
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			event_id    TEXT PRIMARY KEY,
			event_type  TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id   TEXT NOT NULL,
			detail      TEXT NOT NULL DEFAULT '{}',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_entity ON events(entity_id)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			webhook_id  TEXT PRIMARY KEY,
			url         TEXT NOT NULL,
			event_types TEXT NOT NULL DEFAULT '[]',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return tx.Commit()
}

// --- Agent CRUD ---

func (s *SQLiteStore) SaveAgent(agent model.AgentInfo) error {
	caps, _ := json.Marshal(agent.Capabilities)
	roles, _ := json.Marshal(agent.Roles)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO agents (agent_id, name, capabilities, version, status, roles, last_heartbeat, registered_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.AgentID, agent.Name, string(caps), agent.Version, agent.Status,
		string(roles), agent.LastHeartbeat, agent.RegisteredAt,
	)
	return err
}

func (s *SQLiteStore) GetAgent(agentID string) (model.AgentInfo, error) {
	var a model.AgentInfo
	var caps, roles string
	err := s.db.QueryRow(
		`SELECT agent_id, name, capabilities, version, status, roles, last_heartbeat, registered_at FROM agents WHERE agent_id = ?`,
		agentID,
	).Scan(&a.AgentID, &a.Name, &caps, &a.Version, &a.Status, &roles, &a.LastHeartbeat, &a.RegisteredAt)
	if err != nil {
		return a, err
	}
	json.Unmarshal([]byte(caps), &a.Capabilities)
	json.Unmarshal([]byte(roles), &a.Roles)
	return a, nil
}

func (s *SQLiteStore) ListAgents() ([]model.AgentInfo, error) {
	rows, err := s.db.Query(`SELECT agent_id, name, capabilities, version, status, roles, last_heartbeat, registered_at FROM agents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []model.AgentInfo
	for rows.Next() {
		var a model.AgentInfo
		var caps, roles string
		if err := rows.Scan(&a.AgentID, &a.Name, &caps, &a.Version, &a.Status, &roles, &a.LastHeartbeat, &a.RegisteredAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(caps), &a.Capabilities)
		json.Unmarshal([]byte(roles), &a.Roles)
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *SQLiteStore) UpdateAgentStatus(agentID string, status string, lastHeartbeat time.Time) error {
	res, err := s.db.Exec(
		`UPDATE agents SET status = ?, last_heartbeat = ? WHERE agent_id = ?`,
		status, lastHeartbeat, agentID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %s not found", agentID)
	}
	return nil
}

// --- Task CRUD ---

func (s *SQLiteStore) SaveTask(task model.Task) error {
	caps, _ := json.Marshal(task.Capabilities)
	input, _ := json.Marshal(task.Input)
	output, _ := json.Marshal(task.Output)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO tasks (task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskID, task.WorkflowID, task.NodeID, task.Type, string(caps),
		string(input), string(output), task.Status, task.Priority,
		task.AssignedTo, task.RetryCount, task.Error, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetTask(taskID string) (model.Task, error) {
	var t model.Task
	var caps, input, output string
	err := s.db.QueryRow(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE task_id = ?`, taskID,
	).Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
		&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	json.Unmarshal([]byte(caps), &t.Capabilities)
	json.Unmarshal([]byte(input), &t.Input)
	json.Unmarshal([]byte(output), &t.Output)
	return t, nil
}

func (s *SQLiteStore) GetTasksByWorkflow(workflowID string) ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE workflow_id = ?`, workflowID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var caps, input, output string
		if err := rows.Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
			&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(caps), &t.Capabilities)
		json.Unmarshal([]byte(input), &t.Input)
		json.Unmarshal([]byte(output), &t.Output)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *SQLiteStore) UpdateTaskStatus(taskID string, status string, output map[string]string, errMsg string) error {
	outputJSON, _ := json.Marshal(output)
	res, err := s.db.Exec(
		`UPDATE tasks SET status = ?, output = ?, error = ?, updated_at = ? WHERE task_id = ?`,
		status, string(outputJSON), errMsg, time.Now(), taskID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}
	return nil
}

// --- Workflow CRUD ---

func (s *SQLiteStore) SaveWorkflow(instance model.WorkflowInstance, definition model.WorkflowDefinition) error {
	defJSON, _ := json.Marshal(definition)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO workflow_definitions (definition_id, name, definition_json, created_at) VALUES (?, ?, ?, ?)`,
		definition.ID, definition.Name, string(defJSON), time.Now(),
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO workflow_instances (instance_id, definition_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		instance.InstanceID, instance.DefinitionID, instance.Status, instance.CreatedAt, instance.UpdatedAt,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetWorkflow(instanceID string) (model.WorkflowInstance, model.WorkflowDefinition, error) {
	var inst model.WorkflowInstance
	var def model.WorkflowDefinition
	var defJSON string
	err := s.db.QueryRow(
		`SELECT wi.instance_id, wi.definition_id, wi.status, wi.created_at, wi.updated_at, wd.definition_json
		 FROM workflow_instances wi
		 JOIN workflow_definitions wd ON wi.definition_id = wd.definition_id
		 WHERE wi.instance_id = ?`, instanceID,
	).Scan(&inst.InstanceID, &inst.DefinitionID, &inst.Status, &inst.CreatedAt, &inst.UpdatedAt, &defJSON)
	if err != nil {
		return inst, def, err
	}
	json.Unmarshal([]byte(defJSON), &def)
	return inst, def, nil
}

func (s *SQLiteStore) UpdateWorkflowStatus(instanceID string, status string) error {
	res, err := s.db.Exec(
		`UPDATE workflow_instances SET status = ?, updated_at = ? WHERE instance_id = ?`,
		status, time.Now(), instanceID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("workflow %s not found", instanceID)
	}
	return nil
}

// --- Log ---

func (s *SQLiteStore) SaveLog(entry model.LogEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO logs (agent_id, task_id, level, message, timestamp) VALUES (?, ?, ?, ?, ?)`,
		entry.AgentID, entry.TaskID, entry.Level, entry.Message, entry.Timestamp,
	)
	return err
}

func (s *SQLiteStore) GetLogs(agentID string, taskID string, since time.Time, until time.Time) ([]model.LogEntry, error) {
	query := `SELECT agent_id, task_id, level, message, timestamp FROM logs WHERE 1=1`
	var args []interface{}
	if agentID != "" {
		query += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	if taskID != "" {
		query += ` AND task_id = ?`
		args = append(args, taskID)
	}
	if !since.IsZero() {
		query += ` AND timestamp >= ?`
		args = append(args, since)
	}
	if !until.IsZero() {
		query += ` AND timestamp <= ?`
		args = append(args, until)
	}
	query += ` ORDER BY timestamp ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []model.LogEntry
	for rows.Next() {
		var l model.LogEntry
		if err := rows.Scan(&l.AgentID, &l.TaskID, &l.Level, &l.Message, &l.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- Artifact ---

func (s *SQLiteStore) SaveArtifact(artifact model.Artifact) error {
	_, err := s.db.Exec(
		`INSERT INTO artifacts (workflow_id, task_id, name, path, created_at) VALUES (?, ?, ?, ?, ?)`,
		artifact.WorkflowID, artifact.TaskID, artifact.Name, artifact.Path, artifact.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) GetArtifacts(workflowID string) ([]model.Artifact, error) {
	rows, err := s.db.Query(
		`SELECT workflow_id, task_id, name, path, created_at FROM artifacts WHERE workflow_id = ? ORDER BY created_at ASC`,
		workflowID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artifacts []model.Artifact
	for rows.Next() {
		var a model.Artifact
		if err := rows.Scan(&a.WorkflowID, &a.TaskID, &a.Name, &a.Path, &a.CreatedAt); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

// --- Audit Log ---

func (s *SQLiteStore) SaveAuditLog(entry model.AuditLogEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_logs (timestamp, agent_id, action, resource, allowed, reason) VALUES (?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.AgentID, entry.Action, entry.Resource, entry.Allowed, entry.Reason,
	)
	return err
}

// --- Extended StateStore methods (v0.2) ---

// matchCapabilities checks if at least one task capability matches one agent capability.
func matchCapabilities(taskCaps, agentCaps []string) bool {
	capSet := make(map[string]bool)
	for _, c := range agentCaps {
		capSet[c] = true
	}
	for _, c := range taskCaps {
		if capSet[c] {
			return true
		}
	}
	return false
}

// ListPendingTasks returns pending tasks matching the given capabilities,
// ordered by priority DESC, created_at ASC.
// Capability matching is done in Go layer: at least one task capability matches one agent capability.
func (s *SQLiteStore) ListPendingTasks(capabilities []string) ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE status = 'pending' ORDER BY priority DESC, created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var caps, input, output string
		if err := rows.Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
			&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pending task: %w", err)
		}
		json.Unmarshal([]byte(caps), &t.Capabilities)
		json.Unmarshal([]byte(input), &t.Input)
		json.Unmarshal([]byte(output), &t.Output)
		// Filter by capability match in Go layer
		if matchCapabilities(t.Capabilities, capabilities) {
			tasks = append(tasks, t)
		}
	}
	return tasks, rows.Err()
}

// ListUnfinishedTasks returns tasks with status pending, assigned, or running,
// ordered by priority DESC.
func (s *SQLiteStore) ListUnfinishedTasks() ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE status IN ('pending', 'assigned', 'running') ORDER BY priority DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list unfinished tasks: %w", err)
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var caps, input, output string
		if err := rows.Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
			&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan unfinished task: %w", err)
		}
		json.Unmarshal([]byte(caps), &t.Capabilities)
		json.Unmarshal([]byte(input), &t.Input)
		json.Unmarshal([]byte(output), &t.Output)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// CountAgentActiveTasks returns the count of assigned/running tasks for the given agent.
func (s *SQLiteStore) CountAgentActiveTasks(agentID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE assigned_to = ? AND status IN ('assigned', 'running')`,
		agentID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count agent active tasks: %w", err)
	}
	return count, nil
}

// IncrementTaskRetryCount atomically increments the retry_count of the specified task.
func (s *SQLiteStore) IncrementTaskRetryCount(taskID string) error {
	res, err := s.db.Exec(
		`UPDATE tasks SET retry_count = retry_count + 1, updated_at = ? WHERE task_id = ?`,
		time.Now(), taskID,
	)
	if err != nil {
		return fmt.Errorf("increment retry count: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}
	return nil
}

// GetTasksByAssignee returns assigned/running tasks for the specified agent.
func (s *SQLiteStore) GetTasksByAssignee(agentID string) ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE assigned_to = ? AND status IN ('assigned', 'running')`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tasks by assignee: %w", err)
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var caps, input, output string
		if err := rows.Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
			&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan assignee task: %w", err)
		}
		json.Unmarshal([]byte(caps), &t.Capabilities)
		json.Unmarshal([]byte(input), &t.Input)
		json.Unmarshal([]byte(output), &t.Output)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// UpdateTaskAssignment updates the assigned_to field of the specified task.
func (s *SQLiteStore) UpdateTaskAssignment(taskID string, agentID string) error {
	res, err := s.db.Exec(
		`UPDATE tasks SET assigned_to = ?, updated_at = ? WHERE task_id = ?`,
		agentID, time.Now(), taskID,
	)
	if err != nil {
		return fmt.Errorf("update task assignment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}
	return nil
}

// --- Event Storage (v0.3) ---

// SaveEvent inserts an event into the events table.
func (s *SQLiteStore) SaveEvent(event model.Event) error {
	_, err := s.db.Exec(
		`INSERT INTO events (event_id, event_type, entity_type, entity_id, detail, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		event.EventID, event.EventType, event.EntityType, event.EntityID, event.Detail, event.CreatedAt,
	)
	return err
}

// ListEvents returns events matching the given filter, ordered by created_at ASC.
func (s *SQLiteStore) ListEvents(filter model.EventFilter) ([]model.Event, error) {
	query := `SELECT event_id, event_type, entity_type, entity_id, detail, created_at FROM events WHERE 1=1`
	var args []interface{}
	if filter.EventType != "" {
		query += ` AND event_type = ?`
		args = append(args, filter.EventType)
	}
	if filter.EntityID != "" {
		query += ` AND entity_id = ?`
		args = append(args, filter.EntityID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var events []model.Event
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(&e.EventID, &e.EventType, &e.EntityType, &e.EntityID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- Webhook Storage (v0.3) ---

// SaveWebhook inserts a webhook subscription into the webhooks table.
func (s *SQLiteStore) SaveWebhook(webhook model.WebhookSubscription) error {
	eventTypesJSON, err := json.Marshal(webhook.EventTypes)
	if err != nil {
		return fmt.Errorf("marshal event_types: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO webhooks (webhook_id, url, event_types, created_at) VALUES (?, ?, ?, ?)`,
		webhook.WebhookID, webhook.URL, string(eventTypesJSON), webhook.CreatedAt,
	)
	return err
}

// ListWebhooks returns all webhook subscriptions.
func (s *SQLiteStore) ListWebhooks() ([]model.WebhookSubscription, error) {
	rows, err := s.db.Query(`SELECT webhook_id, url, event_types, created_at FROM webhooks`)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()
	var webhooks []model.WebhookSubscription
	for rows.Next() {
		var w model.WebhookSubscription
		var eventTypesStr string
		if err := rows.Scan(&w.WebhookID, &w.URL, &eventTypesStr, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		if err := json.Unmarshal([]byte(eventTypesStr), &w.EventTypes); err != nil {
			return nil, fmt.Errorf("unmarshal event_types: %w", err)
		}
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

// DeleteWebhook removes a webhook subscription by ID.
func (s *SQLiteStore) DeleteWebhook(webhookID string) error {
	_, err := s.db.Exec(`DELETE FROM webhooks WHERE webhook_id = ?`, webhookID)
	return err
}
