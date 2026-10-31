// Package obit turns a photo of a dead household thing into an obituary:
// LOOK at the photo, GATE out the living, WRITE from the details only, and
// CHECK every detail the writing cites in plain code.
package obit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Subjects LOOK can report.
const (
	Thing       = "thing"
	Person      = "person"
	Animal      = "animal"
	LivingPlant = "living_plant"
	Unclear     = "unclear"
)

// Detail kinds.
const (
	Damage  = "damage"
	Wear    = "wear"
	Feature = "feature"
)

const maxDetails = 6

// Detail is one thing the model says it saw in the photo.
type Detail struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Kind string `json:"kind"`
}

// Look is what the model saw.
type Look struct {
	Subject string   `json:"subject"`
	Object  string   `json:"object"`
	Details []Detail `json:"details"`
}

// lookAnswer is what the model returns. Asking for damage, wear and
// features as separate lists makes the model look for injuries first;
// code then merges them into one numbered list of details.
type lookAnswer struct {
	Subject  string   `json:"subject"`
	Object   string   `json:"object"`
	Damage   []string `json:"damage"`
	Wear     []string `json:"wear"`
	Features []string `json:"features"`
}

func (a lookAnswer) look() Look {
	l := Look{Subject: a.Subject, Object: a.Object}
	for _, list := range []struct {
		kind  string
		texts []string
	}{{Damage, a.Damage}, {Wear, a.Wear}, {Feature, a.Features}} {
		for _, t := range list.texts {
			l.Details = append(l.Details, Detail{Text: t, Kind: list.kind})
		}
	}
	return l.normalize()
}

var lookSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "subject": {"type": "string", "enum": ["thing", "person", "animal", "living_plant", "unclear"]},
    "object": {"type": "string"},
    "damage": {"type": "array", "items": {"type": "string"}},
    "wear": {"type": "array", "items": {"type": "string"}},
    "features": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["subject", "object", "damage", "wear", "features"]
}`)

const lookPrompt = `You are the examiner for The Household Gazette, a newspaper that prints obituaries for household objects.
People send photos of things that broke, wore out, burnt, rotted or died. Report only what is visible in this photo.

subject:
- "person" if any person, face, hand or body part is visible anywhere.
- "animal" if a living animal is the main subject.
- "living_plant" only if the main subject is a plant that is green, firm and healthy.
- "thing" for any object or food. A plant with brown, black, wilted, mushy, translucent or rotting leaves is dead: that is a "thing".
- "unclear" if the photo is out of focus, very dark or far away: if no edge in the photo is sharp, or you cannot name the object with confidence, it is "unclear".

object: a short noun phrase naming the main object as precisely as you can, e.g. "laptop charger cable", "white coffee mug", "slice of toast". No brand unless it is readable in the photo.

Then examine the main object closely, injuries first:
- damage: what is broken, cracked, shattered, torn, split, burnt, charred, melted, warped, bent, collapsed, rotting or missing. Empty if nothing is damaged.
- wear: scuffs, stains, dirt, fading, fraying, discoloration, wrinkles. Empty if it looks new.
- features: color, material, shape, parts that are simply there.
Give 2 to 6 observations in total. Each is a short noun phrase under 10 words, not a sentence, specific enough that someone could point at it in the photo: "crack across the top of the screen", "blackened crust", "wires exposed where the cable split".
Do not guess age, price, owner or history. Do not describe the background. Never invent damage you cannot see.
If nothing is damaged, leave damage as an empty list; never write "no damage" or "none" as an entry.
For person, animal, living_plant and unclear, the three lists may be empty.`

// normalize cleans what the model returned: known subject and kinds,
// trimmed texts, no empties or duplicates, at most six details, ids d1..dn.
func (l Look) normalize() Look {
	out := Look{Subject: strings.ToLower(strings.TrimSpace(l.Subject)), Object: clean(l.Object)}
	switch out.Subject {
	case Thing, Person, Animal, LivingPlant, Unclear:
	default:
		out.Subject = Unclear
	}
	seen := map[string]bool{}
	for _, d := range l.Details {
		text := clean(d.Text)
		key := strings.ToLower(text)
		if text == "" || seen[key] || saysNothing(key) {
			continue
		}
		seen[key] = true
		kind := strings.ToLower(strings.TrimSpace(d.Kind))
		switch kind {
		case Damage, Wear, Feature:
		default:
			kind = Feature
		}
		out.Details = append(out.Details, Detail{Text: text, Kind: kind})
		if len(out.Details) == maxDetails {
			break
		}
	}
	for i := range out.Details {
		out.Details[i].ID = fmt.Sprintf("d%d", i+1)
	}
	// A thing with nothing to say about it cannot get an honest obituary.
	if out.Subject == Thing && len(out.Details) == 0 {
		out.Subject = Unclear
	}
	return out
}

// saysNothing spots entries like "no visible damage" that a small model
// writes into a list instead of leaving it empty.
// "no handle" is a real injury, so only empty claims about damage or wear
// count.
func saysNothing(lower string) bool {
	switch lower {
	case "none", "nothing", "n/a":
		return true
	}
	if !strings.HasPrefix(lower, "no ") && !strings.HasPrefix(lower, "nothing ") {
		return false
	}
	for _, w := range []string{"damage", "wear", "visible", "signs", "issues", "defects"} {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// clean trims whitespace, collapses runs of spaces and drops a final period.
func clean(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimRight(s, ".")
}

func (l Look) byID() map[string]Detail {
	m := make(map[string]Detail, len(l.Details))
	for _, d := range l.Details {
		m[d.ID] = d
	}
	return m
}

func (l Look) injuries() []Detail {
	var out []Detail
	for _, d := range l.Details {
		if d.Kind == Damage || d.Kind == Wear {
			out = append(out, d)
		}
	}
	return out
}
