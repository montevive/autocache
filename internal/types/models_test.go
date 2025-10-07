package types

import (
	"encoding/json"
	"testing"
)

func TestMessageUnmarshalJSON_StringContent(t *testing.T) {
	// Test string content format (shorthand)
	jsonData := []byte(`{
		"role": "user",
		"content": "Hello, Claude"
	}`)

	var msg Message
	err := json.Unmarshal(jsonData, &msg)
	if err != nil {
		t.Fatalf("Failed to unmarshal string content: %v", err)
	}

	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(msg.Content))
	}

	if msg.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", msg.Content[0].Type)
	}

	if msg.Content[0].Text != "Hello, Claude" {
		t.Errorf("Expected text 'Hello, Claude', got '%s'", msg.Content[0].Text)
	}
}

func TestMessageUnmarshalJSON_ArrayContent(t *testing.T) {
	// Test array content format (explicit)
	jsonData := []byte(`{
		"role": "user",
		"content": [
			{
				"type": "text",
				"text": "Hello, Claude"
			}
		]
	}`)

	var msg Message
	err := json.Unmarshal(jsonData, &msg)
	if err != nil {
		t.Fatalf("Failed to unmarshal array content: %v", err)
	}

	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(msg.Content))
	}

	if msg.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", msg.Content[0].Type)
	}

	if msg.Content[0].Text != "Hello, Claude" {
		t.Errorf("Expected text 'Hello, Claude', got '%s'", msg.Content[0].Text)
	}
}

func TestMessageUnmarshalJSON_MultipleContentBlocks(t *testing.T) {
	// Test multiple content blocks
	jsonData := []byte(`{
		"role": "user",
		"content": [
			{
				"type": "text",
				"text": "First block"
			},
			{
				"type": "text",
				"text": "Second block"
			}
		]
	}`)

	var msg Message
	err := json.Unmarshal(jsonData, &msg)
	if err != nil {
		t.Fatalf("Failed to unmarshal multiple content blocks: %v", err)
	}

	if len(msg.Content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(msg.Content))
	}

	if msg.Content[0].Text != "First block" {
		t.Errorf("Expected first text 'First block', got '%s'", msg.Content[0].Text)
	}

	if msg.Content[1].Text != "Second block" {
		t.Errorf("Expected second text 'Second block', got '%s'", msg.Content[1].Text)
	}
}

func TestMessageUnmarshalJSON_EmptyString(t *testing.T) {
	// Test empty string content
	jsonData := []byte(`{
		"role": "assistant",
		"content": ""
	}`)

	var msg Message
	err := json.Unmarshal(jsonData, &msg)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty string content: %v", err)
	}

	if len(msg.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(msg.Content))
	}

	if msg.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", msg.Content[0].Type)
	}

	if msg.Content[0].Text != "" {
		t.Errorf("Expected empty text, got '%s'", msg.Content[0].Text)
	}
}

func TestAnthropicRequest_WithStringContent(t *testing.T) {
	// Test full request with string content
	jsonData := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"messages": [
			{
				"role": "user",
				"content": "Test message"
			}
		]
	}`)

	var req AnthropicRequest
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal request with string content: %v", err)
	}

	if req.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model 'claude-3-5-sonnet-20241022', got '%s'", req.Model)
	}

	if len(req.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(req.Messages))
	}

	if len(req.Messages[0].Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(req.Messages[0].Content))
	}

	if req.Messages[0].Content[0].Text != "Test message" {
		t.Errorf("Expected text 'Test message', got '%s'", req.Messages[0].Content[0].Text)
	}
}

func TestAnthropicRequest_WithArrayContent(t *testing.T) {
	// Test full request with array content
	jsonData := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "text",
						"text": "Test message"
					}
				]
			}
		]
	}`)

	var req AnthropicRequest
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal request with array content: %v", err)
	}

	if len(req.Messages[0].Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(req.Messages[0].Content))
	}

	if req.Messages[0].Content[0].Text != "Test message" {
		t.Errorf("Expected text 'Test message', got '%s'", req.Messages[0].Content[0].Text)
	}
}

func TestMessageMarshalJSON_OutputsArray(t *testing.T) {
	// Test that marshaling always outputs array format
	msg := Message{
		Role: "user",
		Content: []ContentBlock{
			{
				Type: "text",
				Text: "Hello",
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Unmarshal to check structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Content should be an array, not a string
	content, ok := result["content"].([]interface{})
	if !ok {
		t.Errorf("Expected content to be an array, got %T", result["content"])
	}

	if len(content) != 1 {
		t.Errorf("Expected 1 content block in output, got %d", len(content))
	}
}
