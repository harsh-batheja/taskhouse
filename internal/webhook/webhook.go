package webhook

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/harsh-batheja/taskhouse/internal/model"
	"github.com/harsh-batheja/taskhouse/internal/store"
)

type Dispatcher struct {
	store  *store.Store
	client *http.Client
}

func NewDispatcher(s *store.Store) *Dispatcher {
	return &Dispatcher{
		store:  s,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *Dispatcher) Dispatch(event string, task model.Task) {
	whs, err := d.store.ListWebhooks()
	if err != nil {
		log.Printf("webhook: list error: %v", err)
		return
	}
	payload := model.WebhookPayload{
		Event:     event,
		Task:      task,
		Timestamp: time.Now().UTC(),
	}
	for _, wh := range whs {
		for _, ev := range wh.Events {
			if ev == event {
				go d.deliver(wh.URL, payload)
				break
			}
		}
	}
}

func (d *Dispatcher) deliver(url string, payload model.WebhookPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: marshal error: %v", err)
		return
	}
	backoff := time.Second
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		resp, err := d.client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("webhook: delivery attempt %d to %s failed: %v", attempt+1, url, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		log.Printf("webhook: delivery attempt %d to %s returned %d", attempt+1, url, resp.StatusCode)
	}
	log.Printf("webhook: all 3 delivery attempts to %s failed", url)
}
