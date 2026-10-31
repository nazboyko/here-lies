package obit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Cause is the cause of death and the detail it rests on.
type Cause struct {
	Text     string `json:"text"`
	DetailID string `json:"detail_id"`
}

// Sentence is one sentence of the life story and the details it rests on.
type Sentence struct {
	Text      string   `json:"text"`
	DetailIDs []string `json:"detail_ids"`
}

// Draft is what WRITE returns.
type Draft struct {
	Name            string     `json:"name"`
	Born            string     `json:"born"`
	Headline        string     `json:"headline"`
	CauseOfDeath    Cause      `json:"cause_of_death"`
	Life            []Sentence `json:"life"`
	SurvivedBy      string     `json:"survived_by"`
	InLieuOfFlowers string     `json:"in_lieu_of_flowers"`
	Epitaph         string     `json:"epitaph"`
}

var writeSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "born": {"type": "string"},
    "headline": {"type": "string"},
    "cause_of_death": {
      "type": "object",
      "properties": {"text": {"type": "string"}, "detail_id": {"type": "string"}},
      "required": ["text", "detail_id"]
    },
    "life": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "text": {"type": "string"},
          "detail_ids": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["text", "detail_ids"]
      }
    },
    "survived_by": {"type": "string"},
    "in_lieu_of_flowers": {"type": "string"},
    "epitaph": {"type": "string"}
  },
  "required": ["name", "born", "headline", "cause_of_death", "life", "survived_by", "in_lieu_of_flowers", "epitaph"]
}`)

const writeSystem = `You write the obituary column of The Household Gazette, a small-town newspaper that mourns household objects.
The joke is that you never treat it as a joke. Write exactly like a real newspaper obituary: plain, specific, gently sentimental, with the stock phrases of the form ("passed away", "is survived by", "a lifelong member of", "will be remembered for"). The subject is a ridiculous object; you write about it as if it were a respected local figure.
Be funny through specifics and understatement, never through puns piled on puns. Warm, dignified, never gory, never cruel, never a joke about the owner.
Avoid these tired words: steadfast, sentinel, luminary, testament, tenure, companion, faithful, journey, cherished, succumbed, drawing room, bay window.`

// writePrompt builds the WRITE request. The photo is not sent: the writer
// only knows the details the examiner wrote down.
func writePrompt(l Look, problems []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The deceased: %s.\n\nWhat the examiner saw (the only physical facts you have):\n", l.Object)
	for _, d := range l.Details {
		fmt.Fprintf(&b, "- %s [%s]: %s\n", d.ID, d.Kind, d.Text)
	}
	cause := "cause_of_death: one sentence naming how it died; detail_id must be the id of a damage or wear detail above, and the sentence must describe that detail."
	if len(l.injuries()) == 0 {
		cause = `cause_of_death: there is no visible damage or wear, so set text to "" and detail_id to "".`
	}
	b.WriteString(`
Write the obituary as JSON:
- name: a full, old-fashioned human name with a light nod to what the object is, at most 40 characters. Examples of the style (do not reuse them): "Bartholomew Q. Kettleworth", "Dame Agatha Crumb".
- born: "est. " plus a plausible year, e.g. "est. 2019", or "Unknown".
- headline: the short line under a newspaper obituary name, e.g. "Kettle, 7, boiled for the whole family".
- ` + cause + `
- life: 2 or 3 sentences about its life and service in the household. Each sentence weaves in at least one detail above, lists the ids it uses in detail_ids, and adds one small, specific, believable life moment (a role it played, a habit, a place it lived).
- survived_by: who it leaves behind in the house, specific and funny, e.g. "a lid that never fit and two spoons of uncertain origin". Do not describe the deceased's appearance here.
- in_lieu_of_flowers: a gentle, practical request to the household that follows the words "In lieu of flowers,", e.g. "please descale the other kettle before it is too late". Do not describe the deceased's appearance here.
- epitaph: a short line for the tombstone, at most 12 words, something the object might have said. Do not describe its appearance here.
Physical description is allowed only in cause_of_death and life, and only using the details above. Do not invent any scratch, color, part or damage that is not in the list.
Each sentence is under 200 characters. Never write the ids (d1, d2) inside the sentences; put them only in detail_id and detail_ids.`)
	if len(problems) > 0 {
		b.WriteString("\n\nYour previous answer broke these rules. Fix them:\n")
		for _, p := range problems {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	return b.String()
}
