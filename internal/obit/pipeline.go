package obit

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/nazboyko/here-lies/internal/ollama"
)

// Model is the part of the Ollama client the pipeline needs.
type Model interface {
	JSON(ctx context.Context, r ollama.Request, v any) error
}

// Step names sent to the page while it waits.
const (
	StepLook  = "look"
	StepWrite = "write"
)

// Result is the whole answer for one photo.
type Result struct {
	Kind     string        `json:"kind"` // "obituary" or "notice"
	Object   string        `json:"object,omitempty"`
	Obituary *Obituary     `json:"obituary,omitempty"`
	Notice   *Notice       `json:"notice,omitempty"`
	Details  []DetailOut   `json:"details"`
	Checks   *Checks       `json:"checks,omitempty"`
	Timings  Timings       `json:"timings"`
	Look     Look          `json:"-"`
	Drafts   []Draft       `json:"-"`
	Reports  []Report      `json:"-"`
	Started  time.Time     `json:"-"`
	Elapsed  time.Duration `json:"-"`
}

// DetailOut is a detail with the places on the card that use it.
type DetailOut struct {
	Detail
	UsedIn []string `json:"used_in"`
}

// Checks tells the page and the eval what CHECK did.
type Checks struct {
	FirstTryPass   bool     `json:"first_try_pass"`
	Retried        bool     `json:"retried"`
	FallbackUsed   bool     `json:"fallback_used"`
	SoftMismatches int      `json:"soft_mismatches"`
	Repaired       []string `json:"repaired"`
	FirstProblems  []string `json:"first_problems,omitempty"`
}

// Timings in milliseconds.
type Timings struct {
	LookMS  int64 `json:"look_ms"`
	WriteMS int64 `json:"write_ms"`
	TotalMS int64 `json:"total_ms"`
}

// Pipeline runs LOOK, GATE, WRITE and CHECK.
type Pipeline struct {
	Model Model
	Now   func() time.Time // time.Now when nil
}

// Run turns one prepared JPEG into a Result. progress, when not nil, is
// called as each model step starts.
func (p *Pipeline) Run(ctx context.Context, jpg []byte, progress func(step string)) (*Result, error) {
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	if progress == nil {
		progress = func(string) {}
	}
	start := time.Now()
	res := &Result{Started: start}

	progress(StepLook)
	var raw lookAnswer
	err := p.Model.JSON(ctx, ollama.Request{
		Prompt:      lookPrompt,
		Images:      [][]byte{jpg},
		Schema:      lookSchema,
		Temperature: 0.2,
	}, &raw)
	res.Timings.LookMS = time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	look := raw.look()
	res.Look, res.Object = look, look.Object

	if n := Gate(look); n != nil {
		res.Kind, res.Notice = "notice", n
		res.Object = ""
		res.Details = []DetailOut{}
		res.Timings.TotalMS = time.Since(start).Milliseconds()
		return res, nil
	}

	progress(StepWrite)
	writeStart := time.Now()
	checks := &Checks{Repaired: []string{}}
	draft, err := p.write(ctx, look, nil)
	if err != nil {
		return nil, err
	}
	report := Check(draft, look)
	res.Drafts, res.Reports = append(res.Drafts, draft), append(res.Reports, report)
	checks.FirstTryPass = report.OK()
	checks.FirstProblems = report.Problems
	if !report.OK() {
		checks.Retried = true
		second, err := p.write(ctx, look, report.Problems)
		switch {
		case err == nil:
			draft = second
			report = Check(draft, look)
			res.Drafts, res.Reports = append(res.Drafts, draft), append(res.Reports, report)
		case errors.Is(err, context.Canceled):
			return nil, err
		}
		// A failed retry keeps the first draft; finalize repairs it.
		checks.FallbackUsed = !report.OK()
	}
	obit, repaired := finalize(draft, look, now())
	checks.Repaired = append(checks.Repaired, repaired...)
	checks.SoftMismatches = report.SoftMismatches
	res.Timings.WriteMS = time.Since(writeStart).Milliseconds()

	res.Kind, res.Obituary, res.Checks = "obituary", &obit, checks
	res.Details = usage(look, obit)
	res.Timings.TotalMS = time.Since(start).Milliseconds()
	return res, nil
}

func (p *Pipeline) write(ctx context.Context, look Look, problems []string) (Draft, error) {
	var d Draft
	err := p.Model.JSON(ctx, ollama.Request{
		System:      writeSystem,
		Prompt:      writePrompt(look, problems),
		Schema:      writeSchema,
		Temperature: 0.9,
	}, &d)
	return d, err
}

// usage lists every detail with the parts of the card that cite it.
func usage(l Look, o Obituary) []DetailOut {
	out := make([]DetailOut, 0, len(l.Details))
	for _, d := range l.Details {
		used := []string{}
		if o.Cause.DetailID == d.ID {
			used = append(used, "cause of death")
		}
		for i, s := range o.Life {
			if slices.Contains(s.DetailIDs, d.ID) {
				used = append(used, "life, sentence "+strconv.Itoa(i+1))
			}
		}
		out = append(out, DetailOut{Detail: d, UsedIn: used})
	}
	return out
}
