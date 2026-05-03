package adapters

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillbench/internal/model"
)

func Parse(agent model.Agent, data []byte) ([]model.Event, error) {
	switch agent {
	case model.AgentCodex:
		return parseCodex(data)
	case model.AgentClaude:
		return parseClaude(data)
	case model.AgentCopilot:
		return parseCopilot(data)
	default:
		return nil, fmt.Errorf("unsupported agent %q", agent)
	}
}

func ParseSessionFile(agent model.Agent, path string) ([]model.Event, error) {
	if path == "" {
		return nil, fmt.Errorf("session path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(agent, b)
}

func LatestSessionPath(agent model.Agent) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var matches []string
	switch agent {
	case model.AgentCodex:
		matches, _ = filepath.Glob(filepath.Join(home, ".codex", "sessions", "*", "*", "*", "*.jsonl"))
	case model.AgentClaude:
		matches, _ = filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
	case model.AgentCopilot:
		matches, _ = filepath.Glob(filepath.Join(home, ".copilot", "session-state", "*", "events.jsonl"))
		if len(matches) == 0 {
			matches, _ = filepath.Glob(filepath.Join(home, ".copilot", "session-state", "*", "*.jsonl"))
		}
	default:
		return "", fmt.Errorf("unsupported agent %q", agent)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no sessions found for %s", agent)
	}
	sort.Slice(matches, func(i, j int) bool {
		ai, _ := os.Stat(matches[i])
		aj, _ := os.Stat(matches[j])
		if ai == nil || aj == nil {
			return matches[i] < matches[j]
		}
		return ai.ModTime().Before(aj.ModTime())
	})
	return matches[len(matches)-1], nil
}

func scanJSONL(data []byte, fn func(json.RawMessage) []model.Event) ([]model.Event, error) {
	var events []model.Event
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("{")) {
			continue
		}
		raw := append(json.RawMessage(nil), line...)
		events = append(events, fn(raw)...)
	}
	return events, sc.Err()
}

func parseTime(v any) time.Time {
	switch t := v.(type) {
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.000"} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

func textFrom(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var parts []string
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					parts = append(parts, s)
				}
				if s, ok := m["content"].(string); ok {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if s, ok := x["text"].(string); ok {
			return s
		}
		if s, ok := x["content"].(string); ok {
			return s
		}
	}
	return ""
}

func jsonText(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
