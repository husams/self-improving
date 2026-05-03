package adapters

import (
	"encoding/json"
	"strings"

	"skillbench/internal/model"
)

func parseCopilot(data []byte) ([]model.Event, error) {
	return scanJSONL(data, func(raw json.RawMessage) []model.Event {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		ts := parseTime(obj["timestamp"])
		typ, _ := obj["type"].(string)
		data, _ := obj["data"].(map[string]any)
		if data == nil {
			data = obj
		}
		switch typ {
		case "user.message":
			text := textFrom(data["message"])
			if text == "" {
				text = textFrom(data["content"])
			}
			return []model.Event{{Type: model.EventUserMessage, Role: "user", Text: text, Timestamp: ts, Raw: raw}}
		case "assistant.message":
			text := textFrom(data["message"])
			if text == "" {
				text = textFrom(data["content"])
			}
			return []model.Event{{Type: model.EventAssistant, Role: "assistant", Text: text, Timestamp: ts, Raw: raw}}
		case "tool.execution_start":
			name := stringValue(data["toolName"])
			if name == "" {
				name = stringValue(data["name"])
			}
			return []model.Event{{Type: model.EventToolCall, Name: name, CallID: stringValue(obj["id"]), Timestamp: ts, Raw: raw}}
		case "tool.execution_complete":
			return []model.Event{{Type: model.EventToolResult, CallID: stringValue(obj["parentId"]), Text: textFrom(data["result"]), Timestamp: ts, Raw: raw}}
		case "error", "abort":
			return []model.Event{{Type: model.EventError, Text: textFrom(data), Timestamp: ts, Raw: raw}}
		default:
			if strings.Contains(typ, "message") && textFrom(data["message"]) != "" {
				return []model.Event{{Type: model.EventAssistant, Text: textFrom(data["message"]), Timestamp: ts, Raw: raw}}
			}
		}
		return nil
	})
}
