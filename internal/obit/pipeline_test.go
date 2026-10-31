package obit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nazboyko/here-lies/internal/ollama"
)

// fakeModel answers each call with the next canned value.
type fakeModel struct {
	answers []any
	calls   []ollama.Request
}

func (f *fakeModel) JSON(_ context.Context, r ollama.Request, v any) error {
	f.calls = append(f.calls, r)
	if len(f.answers) == 0 {
		return errors.New("no more answers")
	}
	a := f.answers[0]
	f.answers = f.answers[1:]
	if err, ok := a.(error); ok {
		return err
	}
	b, _ := json.Marshal(a)
	return json.Unmarshal(b, v)
}

var earbudsAnswer = lookAnswer{
	Subject:  Thing,
	Object:   "white wired earbuds",
	Damage:   []string{"cable frayed near the plug"},
	Wear:     []string{"yellowed ear tips"},
	Features: []string{"white plastic case"},
}

var fixedNow = func() time.Time { return time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local) }

func TestPipelineFirstTry(t *testing.T) {
	m := &fakeModel{answers: []any{earbudsAnswer, goodDraft()}}
	var steps []string
	res, err := (&Pipeline{Model: m, Now: fixedNow}).Run(context.Background(), []byte("jpg"), func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "obituary" || !res.Checks.FirstTryPass || res.Checks.Retried {
		t.Fatalf("kind %s checks %+v", res.Kind, res.Checks)
	}
	if strings.Join(steps, ",") != "look,write" {
		t.Errorf("steps = %v", steps)
	}
	if res.Obituary.Died != "October 31, 2026" || res.Obituary.Dateline != "Saturday, October 31, 2026" {
		t.Errorf("dates %q / %q", res.Obituary.Died, res.Obituary.Dateline)
	}
	if len(m.calls) != 2 || len(m.calls[0].Images) != 1 || len(m.calls[1].Images) != 0 {
		t.Errorf("LOOK must send the image and WRITE must not")
	}
	if res.Details[0].UsedIn[0] != "cause of death" || res.Details[1].UsedIn[0] != "life, sentence 1" {
		t.Errorf("details usage = %+v", res.Details)
	}
}

func TestPipelineRetryThenPass(t *testing.T) {
	bad := goodDraft()
	bad.Life[0].DetailIDs = []string{"d9"}
	m := &fakeModel{answers: []any{earbudsAnswer, bad, goodDraft()}}
	res, err := (&Pipeline{Model: m, Now: fixedNow}).Run(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := res.Checks
	if c.FirstTryPass || !c.Retried || c.FallbackUsed || len(c.Repaired) != 0 {
		t.Errorf("checks = %+v", c)
	}
	if !strings.Contains(m.calls[2].Prompt, `cites "d9"`) {
		t.Errorf("retry prompt does not list the problem:\n%s", m.calls[2].Prompt)
	}
}

func TestPipelineFallback(t *testing.T) {
	bad := goodDraft()
	bad.CauseOfDeath.DetailID = "d3"
	m := &fakeModel{answers: []any{earbudsAnswer, bad, bad}}
	res, err := (&Pipeline{Model: m, Now: fixedNow}).Run(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Checks.FallbackUsed || res.Obituary.Cause.DetailID != "d1" {
		t.Errorf("checks %+v cause %+v", res.Checks, res.Obituary.Cause)
	}
	if len(res.Checks.Repaired) != 1 || res.Checks.Repaired[0] != repairedCause {
		t.Errorf("repaired = %v", res.Checks.Repaired)
	}
}

func TestPipelineRetryErrorKeepsFirstDraft(t *testing.T) {
	bad := goodDraft()
	bad.Life[1].DetailIDs = nil
	m := &fakeModel{answers: []any{earbudsAnswer, bad, ollama.ErrGarbled}}
	res, err := (&Pipeline{Model: m, Now: fixedNow}).Run(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Checks.FallbackUsed || len(res.Obituary.Life) != 1 {
		t.Errorf("checks %+v life %+v", res.Checks, res.Obituary.Life)
	}
}

func TestPipelineStillAlive(t *testing.T) {
	m := &fakeModel{answers: []any{lookAnswer{Subject: Animal, Object: "tabby cat"}}}
	res, err := (&Pipeline{Model: m}).Run(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "notice" || res.Notice.Subject != Animal || res.Obituary != nil || len(m.calls) != 1 {
		t.Errorf("res %+v calls %d", res, len(m.calls))
	}
}

func TestPipelineModelError(t *testing.T) {
	m := &fakeModel{answers: []any{ollama.MissingModel{Model: "x"}}}
	_, err := (&Pipeline{Model: m}).Run(context.Background(), nil, nil)
	var missing ollama.MissingModel
	if !errors.As(err, &missing) {
		t.Errorf("err = %v, want MissingModel", err)
	}
}
