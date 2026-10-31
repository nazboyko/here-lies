package obit

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Obituary is what the card prints.
type Obituary struct {
	Name            string     `json:"name"`
	Born            string     `json:"born"`
	Died            string     `json:"died"`
	Dateline        string     `json:"dateline"`
	Headline        string     `json:"headline"`
	Cause           CauseOut   `json:"cause"`
	Life            []Sentence `json:"life"`
	SurvivedBy      string     `json:"survived_by"`
	InLieuOfFlowers string     `json:"in_lieu_of_flowers"`
	Epitaph         string     `json:"epitaph"`
}

// CauseOut is the printed cause of death. Natural is true when LOOK saw no
// damage or wear at all.
type CauseOut struct {
	Text     string `json:"text"`
	DetailID string `json:"detail_id,omitempty"`
	Natural  bool   `json:"natural"`
}

// Messages shown under "How does it know?" when code had to step in.
const (
	repairedCause   = "Cause of death rewritten from the photo: the model pointed at something that isn't a visible injury."
	repairedLife    = "A sentence was removed because it didn't point at anything in the photo."
	repairedNoLife  = "The life story was replaced with a plain line about what the model saw."
	repairedName    = "The name was shortened to fit the column."
	repairedEpitaph = "The epitaph was too long for the stone and was replaced."
	repairedJoke    = "A closing line was missing or too long and was replaced."
)

// finalize turns a draft into the printed obituary. Every field CHECK would
// reject is repaired from what LOOK actually saw, and each repair is listed.
func finalize(d Draft, l Look, now time.Time) (Obituary, []string) {
	var repaired []string
	ids := l.byID()
	o := Obituary{
		Name:            strings.TrimSpace(d.Name),
		Born:            normalizeBorn(d.Born, now),
		Died:            now.Format("January 2, 2006"),
		Dateline:        now.Format("Monday, January 2, 2006"),
		Headline:        clip(clean(d.Headline), 90),
		SurvivedBy:      lowerStart(trimPrefix(prose(d.SurvivedBy), "survived by")),
		InLieuOfFlowers: lowerStart(trimPrefix(prose(d.InLieuOfFlowers), "in lieu of flowers")),
		Epitaph:         strings.Trim(strings.TrimSpace(d.Epitaph), `"“”`),
	}

	if nameProblem(o.Name) != "" {
		if o.Name == "" {
			o.Name = "The Departed"
		} else {
			o.Name = clip(o.Name, maxName)
		}
		repaired = append(repaired, repairedName)
	}

	injuries := l.injuries()
	switch {
	case len(injuries) == 0:
		o.Cause = CauseOut{Text: naturalCause, Natural: true}
	case causeProblem(d.CauseOfDeath, l) != "":
		o.Cause = CauseOut{Text: sentenceCase(injuries[0].Text) + ".", DetailID: injuries[0].ID}
		repaired = append(repaired, repairedCause)
	default:
		o.Cause = CauseOut{Text: prose(d.CauseOfDeath.Text), DetailID: d.CauseOfDeath.DetailID}
	}

	dropped := false
	for _, s := range d.Life {
		if sentenceProblem(s, ids) != "" || len(o.Life) == maxLife {
			dropped = true
			continue
		}
		o.Life = append(o.Life, Sentence{Text: prose(s.Text), DetailIDs: s.DetailIDs})
	}
	if len(o.Life) == 0 {
		pick := l.Details[0]
		for _, det := range l.Details {
			if det.ID != o.Cause.DetailID {
				pick = det
				break
			}
		}
		o.Life = []Sentence{{Text: "Those who knew it will remember it for this: " + pick.Text + ".", DetailIDs: []string{pick.ID}}}
		repaired = append(repaired, repairedNoLife)
	} else if dropped {
		repaired = append(repaired, repairedLife)
	}

	jokeFixed := false
	if jokeProblem(o.SurvivedBy) != "" {
		o.SurvivedBy, jokeFixed = "everything else in the drawer.", true
	}
	if jokeProblem(o.InLieuOfFlowers) != "" {
		o.InLieuOfFlowers, jokeFixed = "please recycle responsibly.", true
	}
	if jokeFixed {
		repaired = append(repaired, repairedJoke)
	}
	o.SurvivedBy, o.InLieuOfFlowers = endSentence(o.SurvivedBy), endSentence(o.InLieuOfFlowers)
	if epitaphProblem(o.Epitaph) != "" {
		o.Epitaph = "Rest in pieces."
		repaired = append(repaired, repairedEpitaph)
	}
	return o, repaired
}

var (
	yearRe  = regexp.MustCompile(`\b(1[89]\d\d|20\d\d)\b`)
	idRefRe = regexp.MustCompile(`\s*[(\[]\s*d\d+(\s*,\s*d\d+)*\s*[)\]]`)
)

// prose removes detail ids the writer left in a sentence, like "(d1)" or
// "[d2, d3]"; they belong in the id fields, not on the card.
func prose(s string) string {
	// Some answers end a sentence with "(detail_ids: [d1])": cut from there.
	if i := strings.Index(strings.ToLower(s), "detail_id"); i >= 0 {
		s = strings.TrimRight(s[:i], " ([:")
	}
	return strings.Join(strings.Fields(idRefRe.ReplaceAllString(s, "")), " ")
}

// normalizeBorn makes the birth line start with "est." or read "Unknown".
func normalizeBorn(b string, now time.Time) string {
	b = strings.TrimSpace(b)
	lower := strings.ToLower(b)
	switch {
	case b == "", strings.HasPrefix(lower, "unknown"):
		return "Unknown"
	case strings.HasPrefix(lower, "est"):
		rest := strings.TrimLeft(b[3:], ". ")
		if rest == "" || utf8.RuneCountInString(rest) > 30 {
			return "Unknown"
		}
		return "est. " + rest
	}
	if y := yearRe.FindString(b); y != "" && y <= now.Format("2006") {
		return "est. " + y
	}
	return "Unknown"
}

// clip cuts s to at most n runes at a word boundary.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndexFunc(string(r), unicode.IsSpace); i > 0 {
		return strings.TrimSpace(string(r)[:i])
	}
	return string(r)
}

func sentenceCase(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// trimPrefix drops a label the writer repeated, since the card prints it.
func trimPrefix(s, label string) string {
	if len(s) >= len(label) && strings.EqualFold(s[:len(label)], label) {
		return strings.TrimLeft(s[len(label):], ",: ")
	}
	return s
}

// lowerStart lowercases a common opening word so the line reads on from
// the printed label: "In lieu of flowers, please ...".
func lowerStart(s string) string {
	first, _, _ := strings.Cut(s, " ")
	switch strings.ToLower(strings.TrimRight(first, ",")) {
	case "please", "kindly", "send", "donate", "consider", "make", "give", "stop", "buy", "hug",
		"remember", "take", "do", "the", "a", "an", "his", "her", "its", "their", "two", "three",
		"four", "five", "several", "many", "every", "all", "one", "some", "countless":
		return strings.ToLower(s[:1]) + s[1:]
	}
	return s
}

// endSentence adds the final period a printed line needs.
func endSentence(s string) string {
	if s == "" || strings.ContainsAny(s[len(s)-1:], ".!?") {
		return s
	}
	return s + "."
}
