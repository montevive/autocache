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

// TestAnthropicRequest_MarshalJSON_SystemString tests that system string is preserved
func TestAnthropicRequest_MarshalJSON_SystemString(t *testing.T) {
	req := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    "You are a helpful assistant",
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Unmarshal to check structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// System should be a string
	system, ok := result["system"].(string)
	if !ok {
		t.Errorf("Expected system to be a string, got %T", result["system"])
	}

	if system != "You are a helpful assistant" {
		t.Errorf("Expected system text, got %s", system)
	}
}

// TestAnthropicRequest_MarshalJSON_SystemBlocks tests that SystemBlocks are serialized as system array
func TestAnthropicRequest_MarshalJSON_SystemBlocks(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		SystemBlocks: []ContentBlock{
			{
				Type: "text",
				Text: "You are a helpful assistant",
				CacheControl: &CacheControl{
					Type: "ephemeral",
					TTL:  "1h",
				},
			},
		},
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Unmarshal to check structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// System should be an array (not a string)
	systemArray, ok := result["system"].([]interface{})
	if !ok {
		t.Fatalf("Expected system to be an array when SystemBlocks is set, got %T", result["system"])
	}

	if len(systemArray) != 1 {
		t.Fatalf("Expected 1 system block, got %d", len(systemArray))
	}

	// Check the first block
	block, ok := systemArray[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected system block to be an object, got %T", systemArray[0])
	}

	if block["type"] != "text" {
		t.Errorf("Expected type 'text', got %v", block["type"])
	}

	if block["text"] != "You are a helpful assistant" {
		t.Errorf("Expected text 'You are a helpful assistant', got %v", block["text"])
	}

	// Check cache_control exists
	cacheControl, ok := block["cache_control"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected cache_control to exist, got %T", block["cache_control"])
	}

	if cacheControl["type"] != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got %v", cacheControl["type"])
	}

	if cacheControl["ttl"] != "1h" {
		t.Errorf("Expected cache_control ttl '1h', got %v", cacheControl["ttl"])
	}
}

// TestAnthropicRequest_MarshalJSON_SystemBlocksPriority tests SystemBlocks takes priority over System
func TestAnthropicRequest_MarshalJSON_SystemBlocksPriority(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    "This should be ignored",
		SystemBlocks: []ContentBlock{
			{
				Type: "text",
				Text: "This should be used",
			},
		},
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Unmarshal to check structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// System should be an array (SystemBlocks takes priority)
	systemArray, ok := result["system"].([]interface{})
	if !ok {
		t.Fatalf("Expected system to be an array when SystemBlocks is set, got %T", result["system"])
	}

	block, ok := systemArray[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected system block to be an object, got %T", systemArray[0])
	}

	if block["text"] != "This should be used" {
		t.Errorf("Expected SystemBlocks to take priority, got text: %v", block["text"])
	}
}

// TestAnthropicRequest_UnmarshalJSON_SystemString tests unmarshaling system as string
func TestAnthropicRequest_UnmarshalJSON_SystemString(t *testing.T) {
	jsonData := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"system": "You are a helpful assistant",
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
	}`)

	var req AnthropicRequest
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.System != "You are a helpful assistant" {
		t.Errorf("Expected System string, got %s", req.System)
	}

	if len(req.SystemBlocks) != 0 {
		t.Error("SystemBlocks should be empty when system is a string")
	}
}

// TestAnthropicRequest_UnmarshalJSON_SystemBlocks tests unmarshaling system as array
func TestAnthropicRequest_UnmarshalJSON_SystemBlocks(t *testing.T) {
	jsonData := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"system": [
			{
				"type": "text",
				"text": "You are a helpful assistant",
				"cache_control": {"type": "ephemeral", "ttl": "1h"}
			}
		],
		"messages": [{"role": "user", "content": [{"type": "text", "text": "Hello"}]}]
	}`)

	var req AnthropicRequest
	err := json.Unmarshal(jsonData, &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.System != "" {
		t.Error("System string should be empty when system is an array")
	}

	if len(req.SystemBlocks) != 1 {
		t.Fatalf("Expected 1 SystemBlock, got %d", len(req.SystemBlocks))
	}

	if req.SystemBlocks[0].Text != "You are a helpful assistant" {
		t.Errorf("Expected text in SystemBlock, got %s", req.SystemBlocks[0].Text)
	}

	if req.SystemBlocks[0].CacheControl == nil {
		t.Fatal("Expected cache_control in SystemBlock")
	}

	if req.SystemBlocks[0].CacheControl.Type != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got %s", req.SystemBlocks[0].CacheControl.Type)
	}
}

// TestAnthropicRequest_RoundTrip tests marshal -> unmarshal round trip
func TestAnthropicRequest_RoundTrip(t *testing.T) {
	original := &AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		SystemBlocks: []ContentBlock{
			{
				Type: "text",
				Text: "System prompt",
				CacheControl: &CacheControl{
					Type: "ephemeral",
					TTL:  "1h",
				},
			},
		},
		Messages: []Message{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var decoded AnthropicRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if len(decoded.SystemBlocks) != 1 {
		t.Fatalf("Expected 1 SystemBlock after round trip, got %d", len(decoded.SystemBlocks))
	}

	if decoded.SystemBlocks[0].Text != "System prompt" {
		t.Errorf("Text mismatch after round trip: got %s", decoded.SystemBlocks[0].Text)
	}

	if decoded.SystemBlocks[0].CacheControl == nil {
		t.Fatal("CacheControl lost after round trip")
	}
}
