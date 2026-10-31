package obit

import (
	"strings"
	"testing"
	"time"
)

var earbuds = Look{
	Subject: Thing,
	Object:  "white wired earbuds",
	Details: []Detail{
		{ID: "d1", Text: "cable frayed near the plug", Kind: Damage},
		{ID: "d2", Text: "yellowed ear tips", Kind: Wear},
		{ID: "d3", Text: "white plastic case", Kind: Feature},
	},
}

func goodDraft() Draft {
	return Draft{
		Name:         "Sir Reginald Earbud",
		Born:         "est. 2023",
		Headline:     "Beloved earbud, listener of one side",
		CauseOfDeath: Cause{Text: "A frayed cable near the plug, the injury he never spoke about.", DetailID: "d1"},
		Life: []Sentence{
			{Text: "His tips yellowed in loyal service, ear after ear.", DetailIDs: []string{"d2"}},
			{Text: "His white case went everywhere with him.", DetailIDs: []string{"d3"}},
		},
		SurvivedBy:      "three chargers nobody can identify",
		InLieuOfFlowers: "please stop wrapping cables around your hand",
		Epitaph:         "He was always there for you. Mostly the left half.",
	}
}

func TestCheckValid(t *testing.T) {
	r := Check(goodDraft(), earbuds)
	if !r.OK() {
		t.Fatalf("problems on a valid draft: %v", r.Problems)
	}
	if r.SoftMismatches != 0 {
		t.Errorf("soft mismatches = %d, want 0", r.SoftMismatches)
	}
}

func TestCheckProblems(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Draft)
		want string
	}{
		{"unknown cause id", func(d *Draft) { d.CauseOfDeath.DetailID = "d9" }, `cause_of_death cites "d9"`},
		{"cause cites feature", func(d *Draft) { d.CauseOfDeath.DetailID = "d3" }, "which is a feature"},
		{"empty cause", func(d *Draft) { d.CauseOfDeath.Text = " " }, "cause_of_death.text is empty"},
		{"empty life", func(d *Draft) { d.Life = nil }, "life is empty"},
		{"too much life", func(d *Draft) {
			s := d.Life[0]
			d.Life = []Sentence{s, s, s, s}
		}, "life has 4 sentences"},
		{"unknown life id", func(d *Draft) { d.Life[1].DetailIDs = []string{"d7"} }, `life sentence 2 cites "d7"`},
		{"life without ids", func(d *Draft) { d.Life[0].DetailIDs = nil }, "life sentence 1 cites no detail"},
		{"long name", func(d *Draft) { d.Name = strings.Repeat("x", 41) }, "name is 41 characters"},
		{"empty name", func(d *Draft) { d.Name = "" }, "name is empty"},
		{"long sentence", func(d *Draft) { d.Life[0].Text = strings.Repeat("tip ", 60) }, "life sentence 1 is 240 characters"},
		{"long epitaph", func(d *Draft) { d.Epitaph = "one two three four five six seven eight nine ten eleven twelve thirteen" }, "epitaph is 13 words"},
		{"empty joke", func(d *Draft) { d.SurvivedBy = "" }, "survived_by is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := goodDraft()
			c.edit(&d)
			r := Check(d, earbuds)
			if r.OK() {
				t.Fatal("draft passed, want a problem")
			}
			if !strings.Contains(strings.Join(r.Problems, "\n"), c.want) {
				t.Errorf("problems %q do not mention %q", r.Problems, c.want)
			}
		})
	}
}

func TestCheckNaturalCauses(t *testing.T) {
	intact := Look{Subject: Thing, Object: "new mug", Details: []Detail{
		{ID: "d1", Text: "glossy blue glaze", Kind: Feature},
		{ID: "d2", Text: "round handle", Kind: Feature},
	}}
	d := goodDraft()
	d.CauseOfDeath = Cause{}
	d.Life = []Sentence{{Text: "Its glossy blue glaze greeted every morning.", DetailIDs: []string{"d1"}}}
	if r := Check(d, intact); !r.OK() {
		t.Fatalf("natural causes draft rejected: %v", r.Problems)
	}
	o, repaired := finalize(d, intact, time.Date(2026, 10, 31, 12, 0, 0, 0, time.Local))
	if !o.Cause.Natural || o.Cause.Text != naturalCause {
		t.Errorf("cause = %+v, want natural causes", o.Cause)
	}
	if len(repaired) != 0 {
		t.Errorf("repaired = %v, want none", repaired)
	}
}

func TestCheckSoftMismatchCounted(t *testing.T) {
	d := goodDraft()
	d.Life[0] = Sentence{Text: "He loved jazz on Sunday mornings.", DetailIDs: []string{"d2"}}
	r := Check(d, earbuds)
	if !r.OK() {
		t.Fatalf("soft mismatch must not block: %v", r.Problems)
	}
	if r.SoftMismatches != 1 {
		t.Errorf("soft mismatches = %d, want 1", r.SoftMismatches)
	}
}

func TestContentWords(t *testing.T) {
	got := strings.Join(contentWords("His tips yellowed, fraying near the plug."), " ")
	if got != "tip yellow fray plug" {
		t.Errorf("contentWords = %q", got)
	}
	if !sameWord("shoe", "shoes") || sameWord("cat", "catalog") {
		t.Error("sameWord prefix rule is wrong")
	}
}

func TestFinalizeRepairs(t *testing.T) {
	d := goodDraft()
	d.CauseOfDeath.DetailID = "d3"                                  // a feature
	d.Life[0].DetailIDs = []string{"d8"}                            // unknown
	d.Name = "Lord Bartholomew Wiresworth the Third of Pocketshire" // too long
	d.Epitaph = strings.Repeat("word ", 20)
	d.SurvivedBy = ""
	o, repaired := finalize(d, earbuds, time.Now())

	if o.Cause.DetailID != "d1" || o.Cause.Text != "Cable frayed near the plug." {
		t.Errorf("cause = %+v, want the first injury's text", o.Cause)
	}
	if len(o.Life) != 1 || o.Life[0].DetailIDs[0] != "d3" {
		t.Errorf("life = %+v, want only the valid sentence", o.Life)
	}
	if len([]rune(o.Name)) > maxName {
		t.Errorf("name %q is too long", o.Name)
	}
	if o.Epitaph != "Rest in pieces." || o.SurvivedBy == "" {
		t.Errorf("epitaph %q, survived by %q", o.Epitaph, o.SurvivedBy)
	}
	for _, want := range []string{repairedCause, repairedLife, repairedName, repairedEpitaph, repairedJoke} {
		if !strings.Contains(strings.Join(repaired, "\n"), want) {
			t.Errorf("repaired %v misses %q", repaired, want)
		}
	}
	if r := Check(Draft{
		Name: o.Name, CauseOfDeath: Cause{Text: o.Cause.Text, DetailID: o.Cause.DetailID},
		Life: o.Life, SurvivedBy: o.SurvivedBy, InLieuOfFlowers: o.InLieuOfFlowers, Epitaph: o.Epitaph,
	}, earbuds); !r.OK() {
		t.Errorf("finalized obituary fails CHECK: %v", r.Problems)
	}
}

func TestFinalizeTemplateLife(t *testing.T) {
	d := goodDraft()
	d.Life = []Sentence{{Text: "Nothing cited.", DetailIDs: nil}}
	o, repaired := finalize(d, earbuds, time.Now())
	if len(o.Life) != 1 || o.Life[0].Text != "Those who knew it will remember it for this: yellowed ear tips." {
		t.Errorf("life = %+v", o.Life)
	}
	if len(repaired) != 1 || repaired[0] != repairedNoLife {
		t.Errorf("repaired = %v", repaired)
	}
}

func TestNormalizeBorn(t *testing.T) {
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	for in, want := range map[string]string{
		"est. 2023":                "est. 2023",
		"Est 2019":                 "est. 2019",
		"est. the summer of sales": "est. the summer of sales",
		"2021":                     "est. 2021",
		"born 2031":                "Unknown",
		"":                         "Unknown",
		"a long time ago":          "Unknown",
		"unknown":                  "Unknown",
	} {
		if got := normalizeBorn(in, now); got != want {
			t.Errorf("normalizeBorn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFinalizeCleansProse(t *testing.T) {
	d := goodDraft()
	d.CauseOfDeath.Text = "A frayed cable near the plug (d1)."
	d.Life[0].Text = "His tips yellowed [d2, d3] in service. (detail_ids: [d2,"
	d.SurvivedBy = "Survived by: Three chargers nobody can identify"
	d.InLieuOfFlowers = "Please stop wrapping cables"
	o, _ := finalize(d, earbuds, time.Now())
	if o.Cause.Text != "A frayed cable near the plug." || o.Life[0].Text != "His tips yellowed in service." {
		t.Errorf("ids left in prose: %q / %q", o.Cause.Text, o.Life[0].Text)
	}
	if o.SurvivedBy != "three chargers nobody can identify" || o.InLieuOfFlowers != "please stop wrapping cables" {
		t.Errorf("labels: %q / %q", o.SurvivedBy, o.InLieuOfFlowers)
	}
}
