package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harsh-batheja/taskhouse/internal/cli"
)

func main() {
	serverURL, token := loadConfig()

	if v := os.Getenv("TASKHOUSE_URL"); v != "" {
		serverURL = v
	}
	if v := os.Getenv("TASKHOUSE_TOKEN"); v != "" {
		token = v
	}

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	jsonOutput := false
	var filtered []string
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	rest := args[1:]

	var err error
	switch cmd {
	case "add":
		err = cli.RunAdd(serverURL, token, rest, jsonOutput)
	case "list", "ls":
		err = cli.RunList(serverURL, token, rest, jsonOutput)
	case "done":
		err = cli.RunDone(serverURL, token, rest, jsonOutput)
	case "modify", "mod":
		err = cli.RunModify(serverURL, token, rest, jsonOutput)
	case "info":
		err = cli.RunInfo(serverURL, token, rest, jsonOutput)
	case "delete", "del":
		err = cli.RunDelete(serverURL, token, rest, jsonOutput)
	case "webhook":
		err = handleWebhook(serverURL, token, rest, jsonOutput)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func handleWebhook(serverURL, token string, args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: task webhook <add|list|delete> ...")
	}
	switch args[0] {
	case "add":
		return cli.RunWebhookAdd(serverURL, token, args[1:], jsonOutput)
	case "list", "ls":
		return cli.RunWebhookList(serverURL, token, jsonOutput)
	case "delete", "del":
		return cli.RunWebhookDelete(serverURL, token, args[1:], jsonOutput)
	default:
		return fmt.Errorf("unknown webhook subcommand: %s", args[0])
	}
}

func loadConfig() (string, string) {
	serverURL := "http://localhost:8080"
	token := ""

	home, err := os.UserHomeDir()
	if err != nil {
		return serverURL, token
	}
	configPath := filepath.Join(home, ".config", "taskhouse", "config.toml")
	f, err := os.Open(configPath)
	if err != nil {
		return serverURL, token
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")
		switch key {
		case "server":
			serverURL = val
		case "token":
			token = val
		}
	}
	return serverURL, token
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: task <command> [options]

Commands:
  add <description> [project:X] [+tag] [priority:H|M|L]
  list [project:X] [+tag] [status:pending|done|all]
  done <id>
  modify <id> [key:value ...]
  info <id>
  delete <id>
  webhook add <url> [--events create,update,delete,done]
  webhook list
  webhook delete <id>

Flags:
  --json    Output JSON instead of human-readable text`)
}
