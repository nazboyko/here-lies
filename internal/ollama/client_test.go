package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func chatReply(w http.ResponseWriter, content string) {
	json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"role": "assistant", "content": content}})
}

func TestJSONRetriesBadJSON(t *testing.T) {
	calls := 0
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewDecoder(r.Body).Decode(&got)
		if calls == 1 {
			chatReply(w, "{not json")
			return
		}
		chatReply(w, `{"subject":"thing"}`)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	var v struct{ Subject string }
	err := c.JSON(context.Background(), Request{Prompt: "look", Images: [][]byte{[]byte("jpg")}, Schema: json.RawMessage(`{"type":"object"}`), Temperature: 0.2}, &v)
	if err != nil || v.Subject != "thing" || calls != 2 {
		t.Fatalf("err %v subject %q calls %d", err, v.Subject, calls)
	}
	if got.Model != DefaultModel || got.Stream || got.Think || got.Options["temperature"] != 0.2 {
		t.Errorf("request = %+v", got)
	}
	if len(got.Messages) != 1 || string(got.Messages[0].Images[0]) != "jpg" {
		t.Errorf("image not sent: %+v", got.Messages)
	}
}

func TestJSONGivesUpAfterTwoBadAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { chatReply(w, "nope") }))
	defer srv.Close()
	var v map[string]any
	err := (&Client{BaseURL: srv.URL}).JSON(context.Background(), Request{}, &v)
	if !errors.Is(err, ErrGarbled) {
		t.Fatalf("err = %v, want ErrGarbled", err)
	}
}

func TestJSONMissingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"model \"missing-model\" not found, try pulling it first"}`))
	}))
	defer srv.Close()
	var v map[string]any
	err := (&Client{BaseURL: srv.URL, Model: "missing-model"}).JSON(context.Background(), Request{}, &v)
	var m MissingModel
	if !errors.As(err, &m) || Explain(err) != "The model is not on this computer yet. Get it with: ollama pull missing-model" {
		t.Fatalf("err = %v", err)
	}
}

func TestJSONOllamaDown(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	var v map[string]any
	err := (&Client{BaseURL: url}).JSON(context.Background(), Request{}, &v)
	if !errors.Is(err, ErrDown) || Explain(err) != string(ErrDown) {
		t.Fatalf("err = %v", err)
	}
}

func TestJSONTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	var v map[string]any
	err := (&Client{BaseURL: srv.URL, Timeout: 50 * time.Millisecond}).JSON(context.Background(), Request{}, &v)
	if !errors.Is(err, ErrSlow) {
		t.Fatalf("err = %v, want ErrSlow", err)
	}
}

func TestJSONServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"out of memory"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	var v map[string]any
	err := (&Client{BaseURL: srv.URL}).JSON(context.Background(), Request{}, &v)
	if !errors.Is(err, ErrStopped) || !strings.Contains(err.Error(), "out of memory") {
		t.Fatalf("err = %v", err)
	}
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.Write([]byte(`{"version":"0.35.1"}`))
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"gemma4:e4b"},{"name":"llava:latest"}]}`))
		}
	}))
	defer srv.Close()

	h := (&Client{BaseURL: srv.URL}).Health(context.Background())
	if !h.Reachable || !h.ModelPresent || h.Problem != "" || h.Version != "0.35.1" {
		t.Errorf("ready health = %+v", h)
	}
	if h := (&Client{BaseURL: srv.URL, Model: "llava"}).Health(context.Background()); !h.ModelPresent {
		t.Errorf("model without tag not matched: %+v", h)
	}
	h = (&Client{BaseURL: srv.URL, Model: "missing-model"}).Health(context.Background())
	if h.ModelPresent || !strings.Contains(h.Problem, "ollama pull missing-model") {
		t.Errorf("missing health = %+v", h)
	}
	h = (&Client{BaseURL: "http://127.0.0.1:1"}).Health(context.Background())
	if h.Reachable || h.Problem != string(ErrDown) {
		t.Errorf("down health = %+v", h)
	}
}
