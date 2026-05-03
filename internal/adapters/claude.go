package adapters

import (
	"encoding/json"
	"strings"

	"skillbench/internal/model"
)

func parseClaude(data []byte) ([]model.Event, error) {
	return scanJSONL(data, func(raw json.RawMessage) []model.Event {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		ts := parseTime(obj["timestamp"])
		typ, _ := obj["type"].(string)
		msg, _ := obj["message"].(map[string]any)
		if msg == nil {
			msg = obj
		}
		role, _ := msg["role"].(string)
		var out []model.Event
		if role == "user" || typ == "user" {
			out = append(out, model.Event{Type: model.EventUserMessage, Role: "user", Text: textFrom(msg["content"]), Timestamp: ts, Raw: raw})
		}
		if role == "assistant" || typ == "assistant" {
			content, _ := msg["content"].([]any)
			if len(content) == 0 {
				out = append(out, model.Event{Type: model.EventAssistant, Role: "assistant", Text: textFrom(msg["content"]), Timestamp: ts, Raw: raw})
			}
			for _, c := range content {
				part, _ := c.(map[string]any)
				switch part["type"] {
				case "text":
					out = append(out, model.Event{Type: model.EventAssistant, Role: "assistant", Text: textFrom(part["text"]), Timestamp: ts, Raw: raw})
				case "tool_use":
					name := stringValue(part["name"])
					evType := model.EventToolCall
					if strings.EqualFold(name, "Skill") || strings.Contains(strings.ToLower(name), "skill") {
						evType = model.EventSkillInvocation
					}
					out = append(out, model.Event{Type: evType, Name: name, Text: jsonText(part["input"]), CallID: stringValue(part["id"]), Timestamp: ts, Raw: raw})
				case "tool_result":
					out = append(out, model.Event{Type: model.EventToolResult, CallID: stringValue(part["tool_use_id"]), Text: textFrom(part["content"]), Timestamp: ts, Raw: raw})
				}
			}
		}
		if strings.Contains(strings.ToLower(typ), "error") {
			out = append(out, model.Event{Type: model.EventError, Text: textFrom(obj["error"]), Timestamp: ts, Raw: raw})
		}
		return out
	})
}
