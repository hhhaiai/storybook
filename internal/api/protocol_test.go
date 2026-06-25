package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIAdapterParsesContentArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"text","text":"你好"},{"type":"text","text":"世界"}]}}]}`))
	}))
	defer ts.Close()

	adapter := NewAdapter(ProtocolOpenAI, ts.URL, "test-key", time.Second)
	got, err := adapter.ChatCompletion(context.Background(), "test-model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "你好世界" {
		t.Fatalf("unexpected text: %q", got)
	}
}

func TestResponsesAdapterParsesOutputTextArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"第一页"},{"type":"output_text","text":"第二页"}]}]}`))
	}))
	defer ts.Close()

	adapter := NewAdapter(ProtocolResponses, ts.URL, "test-key", time.Second)
	got, err := adapter.ChatCompletion(context.Background(), "test-model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "第一页第二页" {
		t.Fatalf("unexpected text: %q", got)
	}
}

func TestOpenAIAdapterImageAcceptsB64JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer ts.Close()

	adapter := NewAdapter(ProtocolOpenAI, ts.URL, "test-key", time.Second)
	got, err := adapter.ImageGeneration(context.Background(), "test-image", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,aGVsbG8=") {
		t.Fatalf("unexpected data url: %q", got)
	}
}
