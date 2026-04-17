package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/harsh-batheja/taskhouse/internal/model"
	"github.com/harsh-batheja/taskhouse/internal/store"
	"github.com/harsh-batheja/taskhouse/internal/webhook"
)

type Server struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
	authToken  string
	mux        *http.ServeMux
}

func New(s *store.Store, d *webhook.Dispatcher, authToken string) *Server {
	srv := &Server{
		store:      s,
		dispatcher: d,
		authToken:  authToken,
		mux:        http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/tasks", s.auth(s.handleCreateTask))
	s.mux.HandleFunc("GET /api/v1/tasks", s.auth(s.handleListTasks))
	s.mux.HandleFunc("GET /api/v1/tasks/{id}", s.auth(s.handleGetTask))
	s.mux.HandleFunc("PUT /api/v1/tasks/{id}", s.auth(s.handleUpdateTask))
	s.mux.HandleFunc("DELETE /api/v1/tasks/{id}", s.auth(s.handleDeleteTask))
	s.mux.HandleFunc("POST /api/v1/tasks/{id}/done", s.auth(s.handleDoneTask))
	s.mux.HandleFunc("POST /api/v1/webhooks", s.auth(s.handleCreateWebhook))
	s.mux.HandleFunc("GET /api/v1/webhooks", s.auth(s.handleListWebhooks))
	s.mux.HandleFunc("DELETE /api/v1/webhooks/{id}", s.auth(s.handleDeleteWebhook))
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken == "" {
			next(w, r)
			return
		}
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") || strings.TrimPrefix(h, "Bearer ") != s.authToken {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}
	task, err := s.store.CreateTask(req)
	if err != nil {
		log.Printf("create task error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}
	s.dispatcher.Dispatch("create", task)
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	status := r.URL.Query().Get("status")
	tag := r.URL.Query().Get("tag")
	if status == "" {
		status = "pending"
	}
	tasks, err := s.store.ListTasks(project, status, tag)
	if err != nil {
		log.Printf("list tasks error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task ID")
		return
	}
	task, err := s.store.GetTask(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task ID")
		return
	}
	var req model.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	task, err := s.store.UpdateTask(id, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		log.Printf("update task error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}
	s.dispatcher.Dispatch("update", task)
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task ID")
		return
	}
	task, err := s.store.DeleteTask(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		log.Printf("delete task error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	s.dispatcher.Dispatch("delete", task)
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleDoneTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task ID")
		return
	}
	task, err := s.store.MarkDone(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		log.Printf("done task error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to mark task done")
		return
	}
	s.dispatcher.Dispatch("done", task)
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req model.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "events is required")
		return
	}
	wh, err := s.store.CreateWebhook(req)
	if err != nil {
		log.Printf("create webhook error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	whs, err := s.store.ListWebhooks()
	if err != nil {
		log.Printf("list webhooks error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}
	writeJSON(w, http.StatusOK, whs)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}
	if err := s.store.DeleteWebhook(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "webhook not found")
			return
		}
		log.Printf("delete webhook error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id: %w", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("id must be positive")
	}
	return id, nil
}
