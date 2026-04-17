package model

import (
	"time"
)

type Annotation struct {
	Entry       time.Time `json:"entry"`
	Description string    `json:"description"`
}

type Task struct {
	ID          int64        `json:"id"`
	UUID        string       `json:"uuid"`
	Description string       `json:"description"`
	Project     string       `json:"project,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Status      string       `json:"status"`
	Priority    string       `json:"priority"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Entry       time.Time    `json:"entry"`
	Modified    time.Time    `json:"modified"`
	Due         *time.Time   `json:"due"`
	Done        *time.Time   `json:"done"`
	Urgency     float64      `json:"urgency"`
}

type Webhook struct {
	ID     int64    `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type WebhookPayload struct {
	Event     string    `json:"event"`
	Task      Task      `json:"task"`
	Timestamp time.Time `json:"timestamp"`
}

type CreateTaskRequest struct {
	Description string       `json:"description"`
	Project     string       `json:"project,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Priority    string       `json:"priority,omitempty"`
	Due         *time.Time   `json:"due,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
}

type UpdateTaskRequest struct {
	Description *string    `json:"description,omitempty"`
	Project     *string    `json:"project,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Priority    *string    `json:"priority,omitempty"`
	Status      *string    `json:"status,omitempty"`
	Due         *time.Time `json:"due,omitempty"`
}

type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// CalcUrgency computes a simple urgency score for a task.
func CalcUrgency(t *Task) float64 {
	var u float64
	switch t.Priority {
	case "H":
		u += 6
	case "M":
		u += 4
	case "L":
		u += 2
	}
	if t.Due != nil {
		daysUntil := time.Until(*t.Due).Hours() / 24
		if daysUntil < 0 {
			u += 8
		} else if daysUntil < 7 {
			u += 4
		} else if daysUntil < 14 {
			u += 2
		}
	}
	if t.Status == "pending" {
		age := time.Since(t.Entry).Hours() / 24
		u += age * 0.01
	}
	return u
}
