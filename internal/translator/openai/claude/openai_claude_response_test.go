package claude

import (
	"bytes"
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIResponseToClaude_StreamIgnoresNullToolNameDelta(t *testing.T) {
	originalRequest := []byte(`{"stream":true}`)
	var param any

	firstChunks := ConvertOpenAIResponseToClaude(
		context.Background(),
		"test-model",
		originalRequest,
		nil,
		[]byte(`data: {"id":"chatcmpl_1","model":"test-model","created":1,"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":null}]}`),
		&param,
	)
	firstOutput := bytes.Join(firstChunks, nil)
	if !bytes.Contains(firstOutput, []byte(`"name":"read_file"`)) {
		t.Fatalf("expected first chunk to start read_file tool block, got %s", string(firstOutput))
	}

	secondChunks := ConvertOpenAIResponseToClaude(
		context.Background(),
		"test-model",
		originalRequest,
		nil,
		[]byte(`data: {"id":"chatcmpl_1","model":"test-model","created":1,"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":null,"arguments":"{\"path\":\"/tmp/a\"}"}}]},"finish_reason":null}]}`),
		&param,
	)
	secondOutput := bytes.Join(secondChunks, nil)
	if bytes.Contains(secondOutput, []byte(`content_block_start`)) {
		t.Fatalf("did not expect null tool name delta to start a new content block, got %s", string(secondOutput))
	}
	if bytes.Contains(secondOutput, []byte(`"name":""`)) {
		t.Fatalf("did not expect null tool name delta to emit an empty tool name, got %s", string(secondOutput))
	}
}

// TestConvertOpenAIResponseToClaude_ReasoningFallback exercises the ampeco fork patch
// that falls back from `reasoning_content` to `reasoning` so OpenRouter-emitted reasoning
// surfaces as Anthropic `thinking` blocks. Three call sites are covered:
//
//  1. Streaming (convertOpenAIStreamingChunkToAnthropic)
//  2. Non-streaming choices[].message.reasoning (ConvertOpenAIResponseToClaudeNonStream first path)
//  3. Non-streaming alternate non-stream path (ConvertOpenAIResponseToClaudeNonStream second path)
//
// Each subtest also verifies the strict-additive property: when `reasoning_content` is
// present and non-empty, the fallback MUST NOT override it (negative test for AC #7b).
func TestConvertOpenAIResponseToClaude_ReasoningFallback(t *testing.T) {
	t.Run("streaming: reasoning emits thinking_delta when reasoning_content absent", func(t *testing.T) {
		originalRequest := []byte(`{"stream":true}`)
		var param any
		chunks := ConvertOpenAIResponseToClaude(
			context.Background(),
			"openrouter-kimi",
			originalRequest,
			nil,
			[]byte(`data: {"id":"chatcmpl_r1","model":"openrouter-kimi","created":1,"choices":[{"index":0,"delta":{"role":"assistant","reasoning":"Let me think about this carefully."},"finish_reason":null}]}`),
			&param,
		)
		output := bytes.Join(chunks, nil)
		if !bytes.Contains(output, []byte(`"type":"thinking_delta"`)) {
			t.Fatalf("expected fallback `reasoning` to emit thinking_delta, got %s", string(output))
		}
		if !bytes.Contains(output, []byte(`"thinking":"Let me think about this carefully."`)) {
			t.Fatalf("expected thinking_delta to carry the reasoning text, got %s", string(output))
		}
	})

	t.Run("streaming: reasoning_content wins when both fields present", func(t *testing.T) {
		originalRequest := []byte(`{"stream":true}`)
		var param any
		chunks := ConvertOpenAIResponseToClaude(
			context.Background(),
			"deepseek-direct",
			originalRequest,
			nil,
			[]byte(`data: {"id":"chatcmpl_r2","model":"deepseek-direct","created":1,"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"primary","reasoning":"fallback"},"finish_reason":null}]}`),
			&param,
		)
		output := bytes.Join(chunks, nil)
		if !bytes.Contains(output, []byte(`"thinking":"primary"`)) {
			t.Fatalf("expected reasoning_content to win over reasoning when both present, got %s", string(output))
		}
		if bytes.Contains(output, []byte(`"thinking":"fallback"`)) {
			t.Fatalf("did not expect fallback `reasoning` to leak when reasoning_content was present, got %s", string(output))
		}
	})

	t.Run("non-streaming via convertOpenAINonStreamingToAnthropic: reasoning emits thinking block", func(t *testing.T) {
		// stream:false on the original request routes ConvertOpenAIResponseToClaude through
		// convertOpenAINonStreamingToAnthropic, hitting the 2nd reasoning_content site.
		originalRequest := []byte(`{"stream":false}`)
		var param any
		chunks := ConvertOpenAIResponseToClaude(
			context.Background(),
			"openrouter-qwen",
			originalRequest,
			nil,
			[]byte(`data: {"id":"chatcmpl_r5","model":"openrouter-qwen","choices":[{"index":0,"message":{"role":"assistant","reasoning":"Internal Qwen reasoning.","content":"Qwen answer"},"finish_reason":"stop"}]}`),
			&param,
		)
		body := bytes.Join(chunks, nil)
		parsed := gjson.ParseBytes(body)
		thinkingFound := false
		parsed.Get("content").ForEach(func(_, block gjson.Result) bool {
			if block.Get("type").String() == "thinking" && block.Get("thinking").String() == "Internal Qwen reasoning." {
				thinkingFound = true
				return false
			}
			return true
		})
		if !thinkingFound {
			t.Fatalf("expected a thinking content block from fallback `reasoning` (non-stream path 1), got %s", body)
		}
	})

	t.Run("non-streaming: choice.message.reasoning emits thinking content block", func(t *testing.T) {
		originalRequest := []byte(`{"stream":false}`)
		var param any
		// Non-streaming OpenAI response with `reasoning` (no `reasoning_content`)
		body := ConvertOpenAIResponseToClaudeNonStream(
			context.Background(),
			"openrouter-glm",
			originalRequest,
			nil,
			[]byte(`{"id":"chatcmpl_r3","model":"openrouter-glm","choices":[{"index":0,"message":{"role":"assistant","reasoning":"Step-by-step reasoning here.","content":"final answer"},"finish_reason":"stop"}]}`),
			&param,
		)
		parsed := gjson.ParseBytes(body)
		thinkingFound := false
		parsed.Get("content").ForEach(func(_, block gjson.Result) bool {
			if block.Get("type").String() == "thinking" && block.Get("thinking").String() == "Step-by-step reasoning here." {
				thinkingFound = true
				return false
			}
			return true
		})
		if !thinkingFound {
			t.Fatalf("expected a thinking content block from fallback `reasoning`, got %s", body)
		}
	})

	t.Run("non-streaming: reasoning_content wins when both fields present", func(t *testing.T) {
		originalRequest := []byte(`{"stream":false}`)
		var param any
		body := ConvertOpenAIResponseToClaudeNonStream(
			context.Background(),
			"deepseek-direct",
			originalRequest,
			nil,
			[]byte(`{"id":"chatcmpl_r4","model":"deepseek-direct","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"primary thought","reasoning":"never used","content":"answer"},"finish_reason":"stop"}]}`),
			&param,
		)
		parsed := gjson.ParseBytes(body)
		var thinkingTexts []string
		parsed.Get("content").ForEach(func(_, block gjson.Result) bool {
			if block.Get("type").String() == "thinking" {
				thinkingTexts = append(thinkingTexts, block.Get("thinking").String())
			}
			return true
		})
		if len(thinkingTexts) != 1 || thinkingTexts[0] != "primary thought" {
			t.Fatalf("expected exactly one thinking block with reasoning_content text, got %v (body=%s)", thinkingTexts, body)
		}
	})
}
