package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harsh-batheja/taskhouse/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("pragma wal: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("pragma fk: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'pending',
			priority TEXT NOT NULL DEFAULT 'M',
			annotations TEXT NOT NULL DEFAULT '[]',
			entry TEXT NOT NULL,
			modified TEXT NOT NULL,
			due TEXT,
			done TEXT,
			urgency REAL NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS webhooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			events TEXT NOT NULL DEFAULT '[]'
		);
	`)
	return err
}

func (s *Store) CreateTask(req model.CreateTaskRequest) (model.Task, error) {
	now := time.Now().UTC()
	priority := req.Priority
	if priority == "" {
		priority = "M"
	}
	t := model.Task{
		UUID:        uuid.New().String(),
		Description: req.Description,
		Project:     req.Project,
		Tags:        req.Tags,
		Status:      "pending",
		Priority:    priority,
		Annotations: req.Annotations,
		Entry:       now,
		Modified:    now,
		Due:         req.Due,
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	if t.Annotations == nil {
		t.Annotations = []model.Annotation{}
	}
	t.Urgency = model.CalcUrgency(&t)
	tagsJSON, _ := json.Marshal(t.Tags)
	annJSON, _ := json.Marshal(t.Annotations)
	var dueStr *string
	if t.Due != nil {
		ds := t.Due.Format(time.RFC3339)
		dueStr = &ds
	}
	res, err := s.db.Exec(
		`INSERT INTO tasks (uuid,description,project,tags,status,priority,annotations,entry,modified,due,urgency) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		t.UUID, t.Description, t.Project, string(tagsJSON), t.Status, t.Priority, string(annJSON),
		t.Entry.Format(time.RFC3339), t.Modified.Format(time.RFC3339), dueStr, t.Urgency,
	)
	if err != nil {
		return model.Task{}, fmt.Errorf("insert task: %w", err)
	}
	t.ID, _ = res.LastInsertId()
	return t, nil
}

func (s *Store) GetTask(id int64) (model.Task, error) {
	row := s.db.QueryRow(
		`SELECT id,uuid,description,project,tags,status,priority,annotations,entry,modified,due,done,urgency FROM tasks WHERE id=?`, id,
	)
	return scanTask(row)
}

func (s *Store) ListTasks(project, status, tag string) ([]model.Task, error) {
	q := "SELECT id,uuid,description,project,tags,status,priority,annotations,entry,modified,due,done,urgency FROM tasks WHERE 1=1"
	var args []any
	if project != "" {
		q += " AND project=?"
		args = append(args, project)
	}
	if status != "" && status != "all" {
		q += " AND status=?"
		args = append(args, status)
	}
	if tag != "" {
		q += " AND tags LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}
	q += " ORDER BY urgency DESC, id ASC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTask(id int64, req model.UpdateTaskRequest) (model.Task, error) {
	t, err := s.GetTask(id)
	if err != nil {
		return model.Task{}, err
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.Project != nil {
		t.Project = *req.Project
	}
	if req.Tags != nil {
		t.Tags = req.Tags
	}
	if req.Priority != nil {
		t.Priority = *req.Priority
	}
	if req.Status != nil {
		t.Status = *req.Status
	}
	if req.Due != nil {
		t.Due = req.Due
	}
	t.Modified = time.Now().UTC()
	t.Urgency = model.CalcUrgency(&t)
	tagsJSON, _ := json.Marshal(t.Tags)
	annJSON, _ := json.Marshal(t.Annotations)
	var dueStr, doneStr *string
	if t.Due != nil {
		ds := t.Due.Format(time.RFC3339)
		dueStr = &ds
	}
	if t.Done != nil {
		ds := t.Done.Format(time.RFC3339)
		doneStr = &ds
	}
	_, err = s.db.Exec(
		`UPDATE tasks SET description=?,project=?,tags=?,status=?,priority=?,annotations=?,modified=?,due=?,done=?,urgency=? WHERE id=?`,
		t.Description, t.Project, string(tagsJSON), t.Status, t.Priority, string(annJSON),
		t.Modified.Format(time.RFC3339), dueStr, doneStr, t.Urgency, t.ID,
	)
	if err != nil {
		return model.Task{}, fmt.Errorf("update task: %w", err)
	}
	return t, nil
}

func (s *Store) MarkDone(id int64) (model.Task, error) {
	t, err := s.GetTask(id)
	if err != nil {
		return model.Task{}, err
	}
	now := time.Now().UTC()
	t.Status = "done"
	t.Done = &now
	t.Modified = now
	t.Urgency = 0
	tagsJSON, _ := json.Marshal(t.Tags)
	annJSON, _ := json.Marshal(t.Annotations)
	var dueStr *string
	if t.Due != nil {
		ds := t.Due.Format(time.RFC3339)
		dueStr = &ds
	}
	doneStr := now.Format(time.RFC3339)
	_, err = s.db.Exec(
		`UPDATE tasks SET status=?,done=?,modified=?,urgency=?,tags=?,annotations=?,due=? WHERE id=?`,
		t.Status, doneStr, t.Modified.Format(time.RFC3339), t.Urgency, string(tagsJSON), string(annJSON), dueStr, t.ID,
	)
	if err != nil {
		return model.Task{}, fmt.Errorf("mark done: %w", err)
	}
	return t, nil
}

func (s *Store) DeleteTask(id int64) (model.Task, error) {
	t, err := s.GetTask(id)
	if err != nil {
		return model.Task{}, err
	}
	_, err = s.db.Exec("DELETE FROM tasks WHERE id=?", id)
	if err != nil {
		return model.Task{}, fmt.Errorf("delete task: %w", err)
	}
	return t, nil
}

func (s *Store) CreateWebhook(req model.CreateWebhookRequest) (model.Webhook, error) {
	eventsJSON, _ := json.Marshal(req.Events)
	res, err := s.db.Exec("INSERT INTO webhooks (url,events) VALUES (?,?)", req.URL, string(eventsJSON))
	if err != nil {
		return model.Webhook{}, fmt.Errorf("insert webhook: %w", err)
	}
	id, _ := res.LastInsertId()
	return model.Webhook{ID: id, URL: req.URL, Events: req.Events}, nil
}

func (s *Store) ListWebhooks() ([]model.Webhook, error) {
	rows, err := s.db.Query("SELECT id,url,events FROM webhooks ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()
	var whs []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var evStr string
		if err := rows.Scan(&w.ID, &w.URL, &evStr); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		json.Unmarshal([]byte(evStr), &w.Events)
		whs = append(whs, w)
	}
	if whs == nil {
		whs = []model.Webhook{}
	}
	return whs, rows.Err()
}

func (s *Store) DeleteWebhook(id int64) error {
	res, err := s.db.Exec("DELETE FROM webhooks WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("webhook %d not found", id)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row scanner) (model.Task, error) {
	var t model.Task
	var tagsStr, annStr, entryStr, modStr string
	var dueStr, doneStr *string
	err := row.Scan(&t.ID, &t.UUID, &t.Description, &t.Project, &tagsStr,
		&t.Status, &t.Priority, &annStr, &entryStr, &modStr, &dueStr, &doneStr, &t.Urgency)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return model.Task{}, fmt.Errorf("task not found")
		}
		return model.Task{}, fmt.Errorf("scan task: %w", err)
	}
	json.Unmarshal([]byte(tagsStr), &t.Tags)
	json.Unmarshal([]byte(annStr), &t.Annotations)
	if t.Tags == nil {
		t.Tags = []string{}
	}
	if t.Annotations == nil {
		t.Annotations = []model.Annotation{}
	}
	t.Entry, _ = time.Parse(time.RFC3339, entryStr)
	t.Modified, _ = time.Parse(time.RFC3339, modStr)
	if dueStr != nil {
		d, _ := time.Parse(time.RFC3339, *dueStr)
		t.Due = &d
	}
	if doneStr != nil {
		d, _ := time.Parse(time.RFC3339, *doneStr)
		t.Done = &d
	}
	return t, nil
}

func scanTaskRows(rows *sql.Rows) (model.Task, error) {
	return scanTask(rows)
}
