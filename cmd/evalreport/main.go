// Command evalreport turns the last live run (eval/run.json), the photo
// review (eval/review.json) and the variability passes
// (eval/variability.json) into eval/results.md and eval/results.csv.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type detail struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Kind string `json:"kind"`
}

type run struct {
	File     string   `json:"file"`
	Expected string   `json:"expected"`
	Subject  string   `json:"subject"`
	Object   string   `json:"object"`
	Details  []detail `json:"details"`
	Obituary *struct {
		Name  string `json:"name"`
		Cause struct {
			Text     string `json:"text"`
			DetailID string `json:"detail_id"`
			Natural  bool   `json:"natural"`
		} `json:"cause"`
	} `json:"obituary"`
	Notice *struct {
		Title string `json:"title"`
	} `json:"notice"`
	Checks *struct {
		FirstTryPass   bool     `json:"first_try_pass"`
		Retried        bool     `json:"retried"`
		FallbackUsed   bool     `json:"fallback_used"`
		SoftMismatches int      `json:"soft_mismatches"`
		Repaired       []string `json:"repaired"`
		FirstProblems  []string `json:"first_problems"`
	} `json:"checks"`
	Timings struct {
		LookMS  int64 `json:"look_ms"`
		WriteMS int64 `json:"write_ms"`
	} `json:"timings"`
	Error string `json:"error"`
}

type runFile struct {
	Date   string `json:"date"`
	Model  string `json:"model"`
	Ollama string `json:"ollama"`
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Photos []run  `json:"photos"`
}

type review map[string]struct {
	Details []struct {
		Text    string `json:"text"`
		Verdict string `json:"verdict"` // true, false or unsure
	} `json:"details"`
	Note string `json:"note"`
}

func main() {
	dir := "eval"
	var rf runFile
	mustRead(filepath.Join(dir, "run.json"), &rf)
	rev := review{}
	readIfExists(filepath.Join(dir, "review.json"), &rev)
	var vari []run
	readIfExists(filepath.Join(dir, "variability.json"), &vari)

	var md strings.Builder
	rows := [][]string{{"photo", "expected", "subject", "object", "details", "first-try pass", "retried", "fallback",
		"soft mismatches", "look s", "write s", "details_true (reviewed)"}}

	var (
		obits, firstPass, retried, fallbacks, gateRight int
		listed, isTrue, isFalse, unsure, unreviewed     int
		falseCauses                                     []string
		looks, writes                                   []float64
	)
	for _, r := range rf.Photos {
		gate := r.Subject
		if gate == r.Expected {
			gateRight++
		}
		looks = append(looks, float64(r.Timings.LookMS)/1000)
		row := []string{r.File, r.Expected, r.Subject, r.Object, "", "", "", "", "", sec(r.Timings.LookMS), "", ""}
		if r.Obituary != nil && r.Checks != nil {
			obits++
			writes = append(writes, float64(r.Timings.WriteMS)/1000)
			row[3] = r.Object
			row[4] = fmt.Sprint(len(r.Details))
			row[5], row[6], row[7] = yes(r.Checks.FirstTryPass), yes(r.Checks.Retried), yes(r.Checks.FallbackUsed)
			row[8] = fmt.Sprint(r.Checks.SoftMismatches)
			row[10] = sec(r.Timings.WriteMS)
			if r.Checks.FirstTryPass {
				firstPass++
			}
			if r.Checks.Retried {
				retried++
			}
			if r.Checks.FallbackUsed {
				fallbacks++
			}
			t, f, u, n := 0, 0, 0, 0
			verdicts := map[string]string{}
			for _, d := range rev[r.File].Details {
				verdicts[d.Text] = d.Verdict
			}
			for _, d := range r.Details {
				switch verdicts[d.Text] {
				case "true":
					t++
				case "false":
					f++
					if d.ID == r.Obituary.Cause.DetailID {
						falseCauses = append(falseCauses, fmt.Sprintf("%s (%q)", r.File, d.Text))
					}
				case "unsure":
					u++
				default:
					n++
				}
			}
			listed += len(r.Details)
			isTrue, isFalse, unsure, unreviewed = isTrue+t, isFalse+f, unsure+u, unreviewed+n
			row[11] = fmt.Sprintf("%d/%d", t, len(r.Details))
			if f+u+n > 0 {
				row[11] += fmt.Sprintf(" (%d false, %d unsure", f, u)
				if n > 0 {
					row[11] += fmt.Sprintf(", %d not reviewed", n)
				}
				row[11] += ")"
			}
		} else {
			row[3] = "–"
			if r.Notice != nil {
				row[4] = "notice: " + r.Notice.Title
			}
			if r.Error != "" {
				row[4] = "error: " + r.Error
			}
		}
		rows = append(rows, row)
	}

	fmt.Fprintf(&md, "# Eval results\n\n")
	fmt.Fprintf(&md, "Real photos from Wikimedia Commons, the real pipeline, the real model. Run on %s.\n\n", day(rf.Date))
	fmt.Fprintf(&md, "- Model: `%s` through Ollama %s\n- Machine: %s, %s memory\n\n", rf.Model, rf.Ollama, rf.CPU, rf.Memory)
	fmt.Fprintf(&md, "## Summary\n\n| | |\n|---|---|\n")
	fmt.Fprintf(&md, "| Photos | %d |\n", len(rf.Photos))
	fmt.Fprintf(&md, "| Gate decision correct (thing / person / animal / living plant / unclear) | %d of %d |\n", gateRight, len(rf.Photos))
	fmt.Fprintf(&md, "| Obituaries printed | %d |\n", obits)
	fmt.Fprintf(&md, "| CHECK passed on the first draft | %d of %d |\n", firstPass, obits)
	fmt.Fprintf(&md, "| Rewritten once after CHECK | %d |\n", retried)
	fmt.Fprintf(&md, "| Repaired by code after the retry (fallback) | %d |\n", fallbacks)
	fmt.Fprintf(&md, "| Details listed on printed obituaries | %d |\n", listed)
	fmt.Fprintf(&md, "| Details really in the photo (reviewed by eye) | %d true, %d false, %d unsure |\n", isTrue, isFalse, unsure)
	if unreviewed > 0 {
		fmt.Fprintf(&md, "| Details not reviewed yet | %d |\n", unreviewed)
	}
	fmt.Fprintf(&md, "| Causes of death resting on a detail marked false | %d |\n", len(falseCauses))
	fmt.Fprintf(&md, "| Median seconds, LOOK (all photos) | %.1f |\n", median(looks))
	fmt.Fprintf(&md, "| Median seconds, WRITE + CHECK (obituaries) | %.1f |\n\n", median(writes))
	if len(falseCauses) > 0 {
		fmt.Fprintf(&md, "Causes of death built on a detail that is not in the photo: %s. CHECK can prove that every sentence points at a listed detail; it cannot prove the detail itself is real. That part is the vision model's job, and this review measures it.\n\n", strings.Join(falseCauses, "; "))
	}

	fmt.Fprintf(&md, "## Per photo\n\n")
	writeTable(&md, rows)

	fmt.Fprintf(&md, "\n## Review notes\n\n")
	for _, r := range rf.Photos {
		if n := rev[r.File].Note; n != "" {
			fmt.Fprintf(&md, "- **%s**: %s\n", r.File, n)
		}
	}
	for _, r := range rf.Photos {
		if r.Checks != nil && len(r.Checks.FirstProblems) > 0 {
			fmt.Fprintf(&md, "- **%s**, first draft rejected by CHECK: %s\n", r.File, strings.Join(r.Checks.FirstProblems, "; "))
		}
	}

	if len(vari) > 0 {
		fmt.Fprintf(&md, "\n## Variability: the same photo, three runs\n\n")
		byFile := map[string][]run{}
		var order []string
		for _, r := range rf.Photos {
			byFile[r.File] = []run{r}
		}
		for _, v := range vari {
			if !slices.Contains(order, v.File) {
				order = append(order, v.File)
			}
			byFile[v.File] = append(byFile[v.File], v)
		}
		for _, f := range order {
			fmt.Fprintf(&md, "**%s**\n\n| run | name | details |\n|---|---|---|\n", f)
			for i, r := range byFile[f] {
				name := "–"
				if r.Obituary != nil {
					name = r.Obituary.Name
				}
				var ds []string
				for _, d := range r.Details {
					ds = append(ds, d.Text)
				}
				fmt.Fprintf(&md, "| %d | %s | %s |\n", i+1, name, strings.Join(ds, "; "))
			}
			md.WriteString("\n")
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "results.md"), []byte(md.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "results.csv"))
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(f)
	w.WriteAll(rows)
	f.Close()
	fmt.Printf("wrote %s and %s: %d photos, %d obituaries, %d/%d details true\n",
		filepath.Join(dir, "results.md"), filepath.Join(dir, "results.csv"), len(rf.Photos), obits, isTrue, listed)
}

func writeTable(b *strings.Builder, rows [][]string) {
	for i, r := range rows {
		cells := make([]string, len(r))
		for j, c := range r {
			cells[j] = strings.ReplaceAll(c, "|", "/")
		}
		fmt.Fprintf(b, "| %s |\n", strings.Join(cells, " | "))
		if i == 0 {
			fmt.Fprintf(b, "|%s\n", strings.Repeat("---|", len(r)))
		}
	}
}

func mustRead(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("%v (run `make live` first)", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("%s: %v", path, err)
	}
}

func readIfExists(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("%s: %v", path, err)
	}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func sec(ms int64) string { return fmt.Sprintf("%.1f", float64(ms)/1000) }

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func day(rfc string) string {
	if len(rfc) >= 10 {
		return rfc[:10]
	}
	return rfc
}
