package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nazboyko/here-lies/internal/obit"
	"github.com/nazboyko/here-lies/internal/ollama"
)

type cannedModel struct{ answers []string }

func (c *cannedModel) JSON(_ context.Context, _ ollama.Request, v any) error {
	if len(c.answers) == 0 {
		return ollama.ErrDown
	}
	a := c.answers[0]
	c.answers = c.answers[1:]
	return json.Unmarshal([]byte(a), v)
}

type fixedHealth struct{ h ollama.Health }

func (f fixedHealth) Health(context.Context) ollama.Health { return f.h }

const lookJSON = `{"subject":"thing","object":"broken umbrella","damage":["bent spoke on the left"],"wear":[],"features":["black nylon canopy"]}`
const draftJSON = `{"name":"Umberto Brolly","born":"est. 2020","headline":"Kept a city dry","cause_of_death":{"text":"A bent spoke on the left gave out in the wind.","detail_id":"d1"},"life":[{"text":"His black nylon canopy met every storm.","detail_ids":["d2"]}],"survived_by":"a raincoat","in_lieu_of_flowers":"stay indoors","epitaph":"He took the weather personally."}`

func photoForm(t *testing.T, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "x.png")
	fw.Write(data)
	mw.Close()
	return &body, mw.FormDataContentType()
}

func tinyPNG() []byte {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 30)))
	return buf.Bytes()
}

func newServer(answers ...string) *Server {
	return &Server{
		Pipeline: &obit.Pipeline{Model: &cannedModel{answers: answers}},
		Health:   fixedHealth{ollama.Health{Reachable: true, ModelPresent: true, Model: "gemma4:e4b"}},
		Static:   fstest.MapFS{"index.html": {Data: []byte("<h1>Here Lies</h1>")}},
	}
}

func TestObituaryJSON(t *testing.T) {
	body, ct := photoForm(t, tinyPNG())
	req := httptest.NewRequest("POST", "/api/obituary", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	newServer(lookJSON, draftJSON).Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res obit.Result
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Kind != "obituary" || res.Obituary.Name != "Umberto Brolly" || len(res.Details) != 2 {
		t.Errorf("result = %+v", res)
	}
}

func TestObituaryStream(t *testing.T) {
	body, ct := photoForm(t, tinyPNG())
	req := httptest.NewRequest("POST", "/api/obituary", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Accept", "application/x-ndjson")
	rec := httptest.NewRecorder()
	newServer(lookJSON, draftJSON).Handler().ServeHTTP(rec, req)

	var lines []map[string]json.RawMessage
	sc := bufio.NewScanner(rec.Body)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var m map[string]json.RawMessage
		json.Unmarshal(sc.Bytes(), &m)
		lines = append(lines, m)
	}
	if len(lines) != 3 || string(lines[0]["step"]) != `"look"` || string(lines[1]["step"]) != `"write"` || lines[2]["result"] == nil {
		t.Errorf("stream = %s", rec.Body)
	}
}

func TestObituaryErrors(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		model  []string
		status int
		want   string
	}{
		{"not a photo", []byte("hello"), nil, http.StatusUnsupportedMediaType, "isn't a photo"},
		{"ollama down", tinyPNG(), nil, http.StatusServiceUnavailable, "ollama serve"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, ct := photoForm(t, c.data)
			req := httptest.NewRequest("POST", "/api/obituary", body)
			req.Header.Set("Content-Type", ct)
			rec := httptest.NewRecorder()
			newServer(c.model...).Handler().ServeHTTP(rec, req)
			if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("status %d body %s", rec.Code, rec.Body)
			}
		})
	}

	req := httptest.NewRequest("POST", "/api/obituary", strings.NewReader("x"))
	rec := httptest.NewRecorder()
	newServer().Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "No photo arrived") {
		t.Errorf("no photo: status %d body %s", rec.Code, rec.Body)
	}
}

func TestHealthAndStatic(t *testing.T) {
	h := newServer().Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))
	if !strings.Contains(rec.Body.String(), `"model_present":true`) {
		t.Errorf("health = %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/some/page", nil))
	if !strings.Contains(rec.Body.String(), "Here Lies") || rec.Header().Get("Content-Security-Policy") == "" {
		t.Errorf("fallback page = %d %s", rec.Code, rec.Body)
	}
}
