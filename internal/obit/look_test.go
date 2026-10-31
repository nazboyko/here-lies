package obit

import "testing"

func TestNormalize(t *testing.T) {
	raw := Look{
		Subject: " Thing ",
		Object:  "  chipped   mug. ",
		Details: []Detail{
			{ID: "x", Text: "chip on the rim.", Kind: "DAMAGE"},
			{ID: "y", Text: "  ", Kind: "wear"},
			{ID: "z", Text: "Chip on the rim", Kind: "damage"},
			{ID: "q", Text: "coffee stain inside", Kind: "stain"},
			{Text: "a", Kind: Feature}, {Text: "b", Kind: Feature}, {Text: "c", Kind: Feature},
			{Text: "d", Kind: Feature}, {Text: "e", Kind: Feature},
		},
	}
	l := raw.normalize()
	if l.Subject != Thing || l.Object != "chipped mug" {
		t.Errorf("subject %q object %q", l.Subject, l.Object)
	}
	if len(l.Details) != maxDetails {
		t.Fatalf("got %d details, want %d", len(l.Details), maxDetails)
	}
	if l.Details[0] != (Detail{ID: "d1", Text: "chip on the rim", Kind: Damage}) {
		t.Errorf("first detail = %+v", l.Details[0])
	}
	if l.Details[1] != (Detail{ID: "d2", Text: "coffee stain inside", Kind: Feature}) {
		t.Errorf("unknown kind not mapped to feature: %+v", l.Details[1])
	}
	if l.Details[5].ID != "d6" {
		t.Errorf("ids not renumbered: %+v", l.Details)
	}
}

func TestNormalizeEmptyThingIsUnclear(t *testing.T) {
	if got := (Look{Subject: "thing"}).normalize().Subject; got != Unclear {
		t.Errorf("subject = %q, want unclear", got)
	}
	if got := (Look{Subject: "ghost"}).normalize().Subject; got != Unclear {
		t.Errorf("subject = %q, want unclear", got)
	}
}

func TestGate(t *testing.T) {
	cases := map[string]string{
		Person:      "We don't write obituaries for people.",
		Animal:      "Still alive.",
		LivingPlant: "This plant is still with us.",
		Unclear:     "We couldn't make out the deceased.",
	}
	for subject, title := range cases {
		n := Gate(Look{Subject: subject})
		if n == nil || n.Title != title || n.Subject != subject {
			t.Errorf("Gate(%s) = %+v", subject, n)
		}
	}
	if n := Gate(earbuds); n != nil {
		t.Errorf("Gate(thing) = %+v, want nil", n)
	}
}

func TestLookAnswerOrdersInjuriesFirst(t *testing.T) {
	l := earbudsAnswer.look()
	if len(l.Details) != 3 || l.Details[0] != (Detail{"d1", "cable frayed near the plug", Damage}) ||
		l.Details[1].Kind != Wear || l.Details[2].Kind != Feature {
		t.Errorf("details = %+v", l.Details)
	}
	many := lookAnswer{Subject: Thing, Damage: []string{"a", "b"}, Features: []string{"c", "d", "e", "f", "g"}}
	if l := many.look(); len(l.Details) != maxDetails || l.Details[1].Kind != Damage {
		t.Errorf("cap should drop features first: %+v", l.Details)
	}
}

func TestNormalizeDropsEmptyClaims(t *testing.T) {
	a := lookAnswer{Subject: Thing, Damage: []string{"no visible damage", "None"}, Wear: []string{"No wear"}, Features: []string{"green stem"}}
	l := a.look()
	if len(l.Details) != 1 || l.Details[0].Text != "green stem" || len(l.injuries()) != 0 {
		t.Errorf("details = %+v", l.Details)
	}
	if l := (lookAnswer{Subject: Thing, Damage: []string{"no handle"}}).look(); len(l.injuries()) != 1 {
		t.Errorf("details = %+v", l.Details)
	}
}
