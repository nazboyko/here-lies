//go:build live

package obit

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nazboyko/here-lies/internal/ollama"
	"github.com/nazboyko/here-lies/internal/photo"
)

// These tests run the real pipeline against the real model on every photo
// in testdata/photos. Run them with: make live

const (
	photosDir = "../../testdata/photos"
	evalDir   = "../../eval"
)

type manifestEntry struct {
	File        string `json:"file"`
	Expected    string `json:"expected"`
	What        string `json:"what"`
	Publishable bool   `json:"publishable"`
}

// Run is one photo through the pipeline, as written to eval/run.json.
type Run struct {
	File        string    `json:"file"`
	Expected    string    `json:"expected"`
	Publishable bool      `json:"publishable"`
	Subject     string    `json:"subject"`
	Object      string    `json:"object,omitempty"`
	Details     []Detail  `json:"details"`
	Obituary    *Obituary `json:"obituary,omitempty"`
	Notice      *Notice   `json:"notice,omitempty"`
	Checks      *Checks   `json:"checks,omitempty"`
	Drafts      []Draft   `json:"drafts,omitempty"`
	Timings     Timings   `json:"timings"`
	Error       string    `json:"error,omitempty"`
}

// RunFile is the whole eval/run.json.
type RunFile struct {
	Date   string `json:"date"`
	Model  string `json:"model"`
	Ollama string `json:"ollama"`
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Photos []Run  `json:"photos"`
}

func liveClient(t *testing.T) *ollama.Client {
	c := &ollama.Client{Model: os.Getenv("HERELIES_MODEL"), HTTP: &http.Client{}}
	if h := c.Health(context.Background()); h.Problem != "" {
		t.Fatal(h.Problem)
	}
	return c
}

func manifest(t *testing.T) []manifestEntry {
	data, err := os.ReadFile(filepath.Join(photosDir, "manifest.json"))
	if err != nil {
		t.Fatalf("no test photos: %v", err)
	}
	var m []manifestEntry
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func runPhoto(t *testing.T, p *Pipeline, e manifestEntry) Run {
	raw, err := os.ReadFile(filepath.Join(photosDir, e.File))
	if err != nil {
		t.Fatal(err)
	}
	jpg, err := photo.Prepare(raw)
	if err != nil {
		t.Fatalf("%s: %v", e.File, err)
	}
	r := Run{File: e.File, Expected: e.Expected, Publishable: e.Publishable}
	res, err := p.Run(context.Background(), jpg, nil)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Subject = res.Look.Subject
	if res.Notice == nil { // nothing is kept about a person, an animal or a plant
		r.Object, r.Details = res.Look.Object, res.Look.Details
	}
	r.Obituary, r.Notice, r.Checks, r.Drafts, r.Timings = res.Obituary, res.Notice, res.Checks, res.Drafts, res.Timings
	return r
}

func TestLive(t *testing.T) {
	c := liveClient(t)
	p := &Pipeline{Model: c}
	out := RunFile{
		Date:   time.Now().Format(time.RFC3339),
		Model:  c.ModelName(),
		Ollama: c.Health(context.Background()).Version,
		CPU:    sysctl("machdep.cpu.brand_string"),
		Memory: memory(),
	}
	for _, e := range manifest(t) {
		r := runPhoto(t, p, e)
		out.Photos = append(out.Photos, r)
		t.Logf("%-36s %-12s -> %-12s %5.1fs %s", e.File, e.Expected, r.Subject, float64(r.Timings.TotalMS)/1000, summary(r))

		if r.Error != "" {
			t.Errorf("%s: %s", e.File, r.Error)
			continue
		}
		// Safety: nothing alive ever gets an obituary.
		if (e.Expected == Person || e.Expected == Animal) && r.Obituary != nil {
			t.Errorf("%s: wrote an obituary for a %s", e.File, e.Expected)
		}
		// Every printed obituary passes CHECK.
		if o := r.Obituary; o != nil {
			d := Draft{Name: o.Name, CauseOfDeath: Cause{Text: o.Cause.Text, DetailID: o.Cause.DetailID},
				Life: o.Life, SurvivedBy: o.SurvivedBy, InLieuOfFlowers: o.InLieuOfFlowers, Epitaph: o.Epitaph}
			look := Look{Subject: r.Subject, Details: r.Details}
			if rep := Check(d, look); !rep.OK() {
				t.Errorf("%s: printed obituary fails CHECK: %v", e.File, rep.Problems)
			}
		}
	}
	writeJSON(t, filepath.Join(evalDir, "run.json"), out)
}

// TestLiveVariability runs three photos two more times to see how much
// the details and the wording change between runs.
func TestLiveVariability(t *testing.T) {
	c := liveClient(t)
	p := &Pipeline{Model: c}
	pick := map[string]bool{"01-cracked-phone.jpg": true, "04-burnt-toast.jpg": true, "07-collapsing-jack-o-lanterns.jpg": true}
	var runs []Run
	for _, e := range manifest(t) {
		if !pick[e.File] {
			continue
		}
		for i := 0; i < 2; i++ {
			r := runPhoto(t, p, e)
			runs = append(runs, r)
			t.Logf("%s pass %d: %s", e.File, i+2, summary(r))
		}
	}
	writeJSON(t, filepath.Join(evalDir, "variability.json"), runs)
}

func summary(r Run) string {
	switch {
	case r.Error != "":
		return "error: " + r.Error
	case r.Notice != nil:
		return "notice: " + r.Notice.Title
	case r.Obituary != nil:
		return strconv.Itoa(len(r.Details)) + " details, " + r.Obituary.Name
	}
	return ""
}

func writeJSON(t *testing.T, path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sysctl(name string) string {
	if runtime.GOOS != "darwin" {
		return runtime.GOOS + "/" + runtime.GOARCH
	}
	out, err := exec.Command("sysctl", "-n", name).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func memory() string {
	n, err := strconv.ParseInt(sysctl("hw.memsize"), 10, 64)
	if err != nil {
		return "unknown"
	}
	return strconv.FormatInt(n>>30, 10) + " GB"
}
