// Package ollama talks to a local Ollama server: one structured chat call,
// a health check, and errors a person can act on.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultURL     = "http://127.0.0.1:11434"
	DefaultModel   = "gemma4:e4b"
	DefaultTimeout = 180 * time.Second
)

// A Problem is an error whose message tells a person what to do.
type Problem string

func (p Problem) Error() string { return string(p) }

const (
	ErrDown    Problem = "Can't reach the model. Start Ollama with: ollama serve"
	ErrSlow    Problem = "The model took too long. Try again, or try a smaller photo."
	ErrStopped Problem = "The model stopped with an error. Try again."
	ErrGarbled Problem = "The model gave an answer we couldn't read. Try again."
)

// MissingModel means Ollama runs but the model is not pulled yet.
type MissingModel struct{ Model string }

func (m MissingModel) Error() string {
	return "The model is not on this computer yet. Get it with: ollama pull " + m.Model
}

// failure keeps the plain message for the person and the cause for the log.
type failure struct {
	msg   Problem
	cause error
}

func (f *failure) Error() string   { return string(f.msg) + " (" + f.cause.Error() + ")" }
func (f *failure) Unwrap() []error { return []error{f.msg, f.cause} }

// Explain turns any error from this package into one sentence.
func Explain(err error) string {
	var m MissingModel
	if errors.As(err, &m) {
		return m.Error()
	}
	var p Problem
	if errors.As(err, &p) {
		return string(p)
	}
	return string(ErrStopped)
}

// Client calls the chat endpoint of a local Ollama.
type Client struct {
	BaseURL string        // DefaultURL when empty
	Model   string        // DefaultModel when empty
	Timeout time.Duration // per call, DefaultTimeout when zero
	HTTP    *http.Client  // http.DefaultClient when nil
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

// ModelName is the model this client asks.
func (c *Client) ModelName() string {
	if c.Model == "" {
		return DefaultModel
	}
	return c.Model
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}
	return c.HTTP
}

// Request is one structured call: a prompt, optional images, and the JSON
// schema the answer must follow.
type Request struct {
	System      string
	Prompt      string
	Images      [][]byte // raw JPEG bytes, sent base64
	Schema      json.RawMessage
	Temperature float64
}

type chatRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Format   json.RawMessage `json:"format,omitempty"`
	Options  map[string]any  `json:"options"`
	Messages []chatMessage   `json:"messages"`
}

type chatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  [][]byte `json:"images,omitempty"` // []byte marshals as base64
}

type chatResponse struct {
	Message chatMessage `json:"message"`
	Error   string      `json:"error"`
}

// JSON sends the request and decodes the answer into v. An answer that is
// not valid JSON is asked for once more.
func (c *Client) JSON(ctx context.Context, r Request, v any) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bad error
	for attempt := 0; attempt < 2; attempt++ {
		content, err := c.chat(ctx, r)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(content), v); err != nil {
			bad = err
			continue
		}
		return nil
	}
	return &failure{ErrGarbled, fmt.Errorf("model answer is not JSON: %w", bad)}
}

func (c *Client) chat(ctx context.Context, r Request) (string, error) {
	var msgs []chatMessage
	if r.System != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: r.System})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: r.Prompt, Images: r.Images})
	body, err := json.Marshal(chatRequest{
		Model:    c.ModelName(),
		Format:   r.Schema,
		Options:  map[string]any{"temperature": r.Temperature},
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http().Do(req)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return "", &failure{ErrSlow, err}
		case errors.Is(err, context.Canceled):
			return "", err
		}
		return "", &failure{ErrDown, err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", &failure{ErrSlow, err}
		}
		return "", &failure{ErrStopped, err}
	}
	var out chatResponse
	jsonErr := json.Unmarshal(data, &out)
	switch {
	case resp.StatusCode == http.StatusNotFound && strings.Contains(out.Error, "not found"):
		return "", MissingModel{c.ModelName()}
	case resp.StatusCode != http.StatusOK:
		return "", &failure{ErrStopped, fmt.Errorf("ollama answered %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))}
	case jsonErr != nil:
		return "", &failure{ErrStopped, fmt.Errorf("ollama reply is not JSON: %w", jsonErr)}
	}
	return out.Message.Content, nil
}

// Health is what the page needs to know before it sends a photo.
type Health struct {
	Reachable    bool   `json:"reachable"`
	ModelPresent bool   `json:"model_present"`
	Model        string `json:"model"`
	Version      string `json:"ollama_version,omitempty"`
	Problem      string `json:"problem,omitempty"` // what to do; empty when ready
}

// Health checks that Ollama answers and has the model.
func (c *Client) Health(ctx context.Context) Health {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	h := Health{Model: c.ModelName()}

	var v struct {
		Version string `json:"version"`
	}
	if err := c.getJSON(ctx, "/api/version", &v); err != nil {
		h.Problem = string(ErrDown)
		return h
	}
	h.Reachable, h.Version = true, v.Version

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := c.getJSON(ctx, "/api/tags", &tags); err != nil {
		h.Problem = string(ErrDown)
		return h
	}
	want := h.Model
	if !strings.Contains(want, ":") {
		want += ":latest"
	}
	for _, m := range tags.Models {
		if m.Name == want {
			h.ModelPresent = true
		}
	}
	if !h.ModelPresent {
		h.Problem = MissingModel{h.Model}.Error()
	}
	return h
}

func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", path, resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}
