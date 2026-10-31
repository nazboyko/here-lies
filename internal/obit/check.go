package obit

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits CHECK enforces.
const (
	maxName      = 40
	maxSentence  = 220
	maxEpitaph   = 12 // words
	maxLife      = 3
	naturalCause = "Natural causes. It was simply no longer needed."
)

// Report is what CHECK found in a draft. Problems block the draft and are
// written as instructions the writer can follow on a retry. Soft mismatches
// are only counted.
type Report struct {
	Problems       []string
	SoftMismatches int
}

// OK reports whether the draft can be printed as it is.
func (r Report) OK() bool { return len(r.Problems) == 0 }

// Check validates a draft against what LOOK saw. No model is involved.
func Check(d Draft, l Look) Report {
	var r Report
	ids := l.byID()
	natural := len(l.injuries()) == 0

	if p := nameProblem(d.Name); p != "" {
		r.Problems = append(r.Problems, p)
	}
	if !natural {
		if p := causeProblem(d.CauseOfDeath, l); p != "" {
			r.Problems = append(r.Problems, p)
		} else if !sharesWord(d.CauseOfDeath.Text, []string{d.CauseOfDeath.DetailID}, ids) {
			r.SoftMismatches++
		}
	}
	switch n := len(d.Life); {
	case n == 0:
		r.Problems = append(r.Problems, "life is empty: write 2 or 3 sentences, each citing detail ids")
	case n > maxLife:
		r.Problems = append(r.Problems, fmt.Sprintf("life has %d sentences: write at most %d", n, maxLife))
	}
	for i, s := range d.Life {
		if p := sentenceProblem(s, ids); p != "" {
			r.Problems = append(r.Problems, fmt.Sprintf("life sentence %d %s", i+1, p))
		} else if !sharesWord(s.Text, s.DetailIDs, ids) {
			r.SoftMismatches++
		}
	}
	for _, f := range []struct{ name, text string }{
		{"survived_by", d.SurvivedBy},
		{"in_lieu_of_flowers", d.InLieuOfFlowers},
	} {
		if p := jokeProblem(f.text); p != "" {
			r.Problems = append(r.Problems, f.name+" "+p)
		}
	}
	if p := epitaphProblem(d.Epitaph); p != "" {
		r.Problems = append(r.Problems, p)
	}
	return r
}

func nameProblem(name string) string {
	switch n := utf8.RuneCountInString(strings.TrimSpace(name)); {
	case n == 0:
		return "name is empty"
	case n > maxName:
		return fmt.Sprintf("name is %d characters: keep it at %d or fewer", n, maxName)
	}
	return ""
}

func causeProblem(c Cause, l Look) string {
	if strings.TrimSpace(c.Text) == "" {
		return "cause_of_death.text is empty"
	}
	if n := utf8.RuneCountInString(c.Text); n > maxSentence {
		return fmt.Sprintf("cause_of_death.text is %d characters: keep it under %d", n, maxSentence)
	}
	allowed := idList(l.injuries())
	d, ok := l.byID()[c.DetailID]
	switch {
	case !ok:
		return fmt.Sprintf("cause_of_death cites %q, which is not one of the details: use one of %s", c.DetailID, allowed)
	case d.Kind == Feature:
		return fmt.Sprintf("cause_of_death cites %s, which is a feature, not damage or wear: use one of %s", d.ID, allowed)
	}
	return ""
}

func sentenceProblem(s Sentence, ids map[string]Detail) string {
	if strings.TrimSpace(s.Text) == "" {
		return "is empty"
	}
	if n := utf8.RuneCountInString(s.Text); n > maxSentence {
		return fmt.Sprintf("is %d characters: keep it under %d", n, maxSentence)
	}
	if len(s.DetailIDs) == 0 {
		return "cites no detail: every sentence must mention a detail and list its id"
	}
	for _, id := range s.DetailIDs {
		if _, ok := ids[id]; !ok {
			return fmt.Sprintf("cites %q, which is not one of the details", id)
		}
	}
	return ""
}

func jokeProblem(text string) string {
	switch n := utf8.RuneCountInString(strings.TrimSpace(text)); {
	case n == 0:
		return "is empty"
	case n > maxSentence:
		return fmt.Sprintf("is %d characters: keep it under %d", n, maxSentence)
	}
	return ""
}

func epitaphProblem(e string) string {
	switch n := len(strings.Fields(e)); {
	case n == 0:
		return "epitaph is empty"
	case n > maxEpitaph:
		return fmt.Sprintf("epitaph is %d words: keep it at %d or fewer", n, maxEpitaph)
	}
	return ""
}

func idList(ds []Detail) string {
	var ids []string
	for _, d := range ds {
		ids = append(ids, d.ID)
	}
	return strings.Join(ids, ", ")
}

// sharesWord reports whether the sentence and at least one cited detail
// have a content word in common. It is a soft signal: a sentence can talk
// about a detail in other words.
func sharesWord(sentence string, cited []string, ids map[string]Detail) bool {
	words := contentWords(sentence)
	for _, id := range cited {
		for _, w := range contentWords(ids[id].Text) {
			if slices.ContainsFunc(words, func(v string) bool { return sameWord(v, w) }) {
				return true
			}
		}
	}
	return false
}

// sameWord matches equal stems, or a stem of four or more letters that
// starts the other word ("shoe" and "shoes", "glass" and "glasses").
func sameWord(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	return a == b || (len(a) >= 4 && strings.HasPrefix(b, a))
}

var stopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an and are as at be been but by for from had has have he her hers him his
		i in into is it its itself me my near no nor not of off on once one only or our out over own she so some
		than that the their them then there these they this those through to too under up very was we were what
		when where which while who whom why will with would you your all any both each few more most other such
		can did do does doing just now about above after again against before below between during further here
		how same should until upon yet ever every still also lot`) {
		stopwords[w] = true
	}
}

// contentWords lowercases, strips punctuation and stopwords, and trims
// common suffixes so "yellowed" meets "yellowing".
func contentWords(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) < 3 || stopwords[w] {
			continue
		}
		out = append(out, stem(w))
	}
	return out
}

func stem(w string) string {
	for _, suf := range []string{"ing", "ed", "s", "ly"} {
		if strings.HasSuffix(w, suf) && len(w)-len(suf) >= 3 {
			return strings.TrimSuffix(w, suf)
		}
	}
	return w
}
