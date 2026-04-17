package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/harsh-batheja/taskhouse/internal/model"
)

var client = &http.Client{Timeout: 30 * time.Second}

func doRequest(method, url, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

func RunAdd(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: task add <description> [project:X] [+tag] [priority:H|M|L]")
	}
	var desc []string
	var project string
	var tags []string
	priority := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "project:") {
			project = strings.TrimPrefix(arg, "project:")
		} else if strings.HasPrefix(arg, "+") {
			tags = append(tags, strings.TrimPrefix(arg, "+"))
		} else if strings.HasPrefix(arg, "priority:") {
			priority = strings.TrimPrefix(arg, "priority:")
		} else {
			desc = append(desc, arg)
		}
	}
	reqBody := model.CreateTaskRequest{
		Description: strings.Join(desc, " "),
		Project:     project,
		Tags:        tags,
		Priority:    priority,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := doRequest("POST", serverURL+"/api/v1/tasks", token, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return readError(resp)
	}
	var task model.Task
	json.NewDecoder(resp.Body).Decode(&task)
	if jsonOutput {
		return printJSON(task)
	}
	fmt.Printf("Created task %d: %s\n", task.ID, task.Description)
	return nil
}

func RunList(serverURL, token string, args []string, jsonOutput bool) error {
	var project, status, tag string
	status = "pending"
	for _, arg := range args {
		if strings.HasPrefix(arg, "project:") {
			project = strings.TrimPrefix(arg, "project:")
		} else if strings.HasPrefix(arg, "status:") {
			status = strings.TrimPrefix(arg, "status:")
		} else if strings.HasPrefix(arg, "+") {
			tag = strings.TrimPrefix(arg, "+")
		}
	}
	url := serverURL + "/api/v1/tasks?status=" + status
	if project != "" {
		url += "&project=" + project
	}
	if tag != "" {
		url += "&tag=" + tag
	}
	resp, err := doRequest("GET", url, token, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	var tasks []model.Task
	json.NewDecoder(resp.Body).Decode(&tasks)
	if jsonOutput {
		return printJSON(tasks)
	}
	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPri\tProject\tTags\tDescription")
	fmt.Fprintln(w, "--\t---\t-------\t----\t-----------")
	for _, t := range tasks {
		tagStr := ""
		if len(t.Tags) > 0 {
			tagStr = "+" + strings.Join(t.Tags, " +")
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", t.ID, t.Priority, t.Project, tagStr, t.Description)
	}
	w.Flush()
	return nil
}

func RunDone(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: task done <id>")
	}
	id := args[0]
	resp, err := doRequest("POST", serverURL+"/api/v1/tasks/"+id+"/done", token, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	var task model.Task
	json.NewDecoder(resp.Body).Decode(&task)
	if jsonOutput {
		return printJSON(task)
	}
	fmt.Printf("Completed task %d: %s\n", task.ID, task.Description)
	return nil
}

func RunModify(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: task modify <id> [key:value ...]")
	}
	id := args[0]
	req := make(map[string]any)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "description":
			req["description"] = val
		case "project":
			req["project"] = val
		case "priority":
			req["priority"] = val
		case "status":
			req["status"] = val
		case "tags":
			req["tags"] = strings.Split(val, ",")
		}
	}
	data, _ := json.Marshal(req)
	resp, err := doRequest("PUT", serverURL+"/api/v1/tasks/"+id, token, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	var task model.Task
	json.NewDecoder(resp.Body).Decode(&task)
	if jsonOutput {
		return printJSON(task)
	}
	fmt.Printf("Modified task %d: %s\n", task.ID, task.Description)
	return nil
}

func RunInfo(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: task info <id>")
	}
	id := args[0]
	resp, err := doRequest("GET", serverURL+"/api/v1/tasks/"+id, token, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	var task model.Task
	json.NewDecoder(resp.Body).Decode(&task)
	if jsonOutput {
		return printJSON(task)
	}
	fmt.Printf("ID:          %d\n", task.ID)
	fmt.Printf("UUID:        %s\n", task.UUID)
	fmt.Printf("Description: %s\n", task.Description)
	fmt.Printf("Project:     %s\n", task.Project)
	fmt.Printf("Tags:        %s\n", strings.Join(task.Tags, ", "))
	fmt.Printf("Status:      %s\n", task.Status)
	fmt.Printf("Priority:    %s\n", task.Priority)
	fmt.Printf("Urgency:     %.1f\n", task.Urgency)
	fmt.Printf("Entry:       %s\n", task.Entry.Format(time.RFC3339))
	fmt.Printf("Modified:    %s\n", task.Modified.Format(time.RFC3339))
	if task.Due != nil {
		fmt.Printf("Due:         %s\n", task.Due.Format(time.RFC3339))
	}
	if task.Done != nil {
		fmt.Printf("Done:        %s\n", task.Done.Format(time.RFC3339))
	}
	return nil
}

func RunDelete(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: task delete <id>")
	}
	id := args[0]
	resp, err := doRequest("DELETE", serverURL+"/api/v1/tasks/"+id, token, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	var task model.Task
	json.NewDecoder(resp.Body).Decode(&task)
	if jsonOutput {
		return printJSON(task)
	}
	fmt.Printf("Deleted task %d: %s\n", task.ID, task.Description)
	return nil
}

func RunWebhookAdd(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: task webhook add <url> [--events create,update,delete,done]")
	}
	whURL := args[0]
	events := []string{"create", "update", "delete", "done"}
	for i, arg := range args {
		if arg == "--events" && i+1 < len(args) {
			events = strings.Split(args[i+1], ",")
			break
		}
	}
	reqBody := model.CreateWebhookRequest{URL: whURL, Events: events}
	data, _ := json.Marshal(reqBody)
	resp, err := doRequest("POST", serverURL+"/api/v1/webhooks", token, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return readError(resp)
	}
	var wh model.Webhook
	json.NewDecoder(resp.Body).Decode(&wh)
	if jsonOutput {
		return printJSON(wh)
	}
	fmt.Printf("Created webhook %d for %s (events: %s)\n", wh.ID, wh.URL, strings.Join(wh.Events, ","))
	return nil
}

func RunWebhookList(serverURL, token string, jsonOutput bool) error {
	resp, err := doRequest("GET", serverURL+"/api/v1/webhooks", token, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readError(resp)
	}
	var whs []model.Webhook
	json.NewDecoder(resp.Body).Decode(&whs)
	if jsonOutput {
		return printJSON(whs)
	}
	if len(whs) == 0 {
		fmt.Println("No webhooks registered.")
		return nil
	}
	for _, wh := range whs {
		fmt.Printf("%d: %s (events: %s)\n", wh.ID, wh.URL, strings.Join(wh.Events, ","))
	}
	return nil
}

func RunWebhookDelete(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: task webhook delete <id>")
	}
	id := args[0]
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return fmt.Errorf("invalid webhook ID: %s", id)
	}
	resp, err := doRequest("DELETE", serverURL+"/api/v1/webhooks/"+id, token, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return readError(resp)
	}
	if !jsonOutput {
		fmt.Printf("Deleted webhook %s\n", id)
	}
	return nil
}

func readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
