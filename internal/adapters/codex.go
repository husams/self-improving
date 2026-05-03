package adapters

import (
	"encoding/json"
	"strings"

	"skillbench/internal/model"
)

func parseCodex(data []byte) ([]model.Event, error) {
	return scanJSONL(data, func(raw json.RawMessage) []model.Event {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		ts := parseTime(obj["timestamp"])
		payload, _ := obj["payload"].(map[string]any)
		if payload == nil {
			payload = obj
		}
		item, _ := payload["item"].(map[string]any)
		if item == nil {
			item = payload
		}
		t, _ := item["type"].(string)
		if t == "" {
			t, _ = payload["type"].(string)
		}
		switch t {
		case "message":
			role, _ := item["role"].(string)
			evType := model.EventAssistant
			if role == "user" {
				evType = model.EventUserMessage
			}
			return []model.Event{{Type: evType, Role: role, Text: textFrom(item["content"]), Timestamp: ts, Raw: raw}}
		case "function_call", "tool_search_call", "web_search_call", "custom_tool_call":
			name, _ := item["name"].(string)
			if name == "" {
				name, _ = item["namespace"].(string)
			}
			ev := model.Event{Type: model.EventToolCall, Name: name, Text: jsonText(item["arguments"]), Timestamp: ts, Raw: raw}
			if call, _ := item["call_id"].(string); call != "" {
				ev.CallID = call
			}
			if strings.EqualFold(name, "Skill") || strings.Contains(strings.ToLower(name), "skill") {
				ev.Type = model.EventSkillInvocation
			}
			return []model.Event{ev}
		case "function_call_output", "custom_tool_call_output", "tool_search_output":
			return []model.Event{{Type: model.EventToolResult, CallID: stringValue(item["call_id"]), Text: textFrom(item["output"]), Timestamp: ts, Raw: raw}}
		case "reasoning":
			return nil
		default:
			if msg, _ := payload["message"].(string); msg != "" && strings.Contains(strings.ToLower(msg), "error") {
				return []model.Event{{Type: model.EventError, Text: msg, Timestamp: ts, Raw: raw}}
			}
		}
		return nil
	})
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
