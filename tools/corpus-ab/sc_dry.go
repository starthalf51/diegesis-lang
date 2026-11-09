package main

// Dry run: runs the whole pipeline with a fake completer (fixed JSON). 0 real calls.

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

type fakeC struct{ n int }

var idRe = regexp.MustCompile(`(?m)^(E\d+)\. `)
var wRe = regexp.MustCompile(`(?m)^(W\d+)\. `)

func (f *fakeC) Complete(_ context.Context, sys, user string) (render.Result, error) {
	f.n++
	ok := func(s string) (render.Result, error) {
		return render.Result{Text: "```json\n" + s + "\n```", StopReason: "end_turn"}, nil
	}
	if strings.Contains(sys, "reconstructs") || strings.Contains(sys, "convert a piece of narrative prose") {
		return ok(`{"acts":[],"facts_stated":[],"new_events":[]}`)
	}
	if f.n == 2 {
		return render.Result{Text: "```json\n{broken", StopReason: "end_turn"}, nil // broken JSON → retry path
	}
	tier2 := strings.Contains(sys, "re-check")
	var items []string
	sec := user
	if tier2 {
		sec = user[strings.Index(user, "## Recheck these items"):]
	} else {
		sec = user[:strings.Index(user, "## Prose")]
	}
	for _, m := range idRe.FindAllStringSubmatch(sec, -1) {
		a := "yes"
		if m[1] == "E1" && !tier2 {
			a = "no" // no E1 in stage 1 → stage 2 fires
		}
		items = append(items, fmt.Sprintf(`{"id":%q,"fact_present":%q,"as_stated":%q,"quote":"q"}`, m[1], a, a))
	}
	if tier2 {
		return ok(`{"recheck":[` + strings.Join(items, ",") + `],"candidates":[{"id":"C1","verdict":"extra","item":"","quote":"q"}]}`)
	}
	var ws []string
	for _, m := range wRe.FindAllStringSubmatch(sec, -1) {
		ws = append(ws, fmt.Sprintf(`{"id":%q,"revealed":"no","quote":""}`, m[1]))
	}
	other := `[]`
	if f.n == 1 {
		other = `[{"actor":"","verb":"does","targets":[],"action":"","text":"x","quote":"zz"}]`
	}
	return ok(`{"items":[` + strings.Join(items, ",") + `],"withheld":[` + strings.Join(ws, ",") + `],"other":` + other + `}`)
}

func runDry(ctx context.Context) {
	out := "examples/corpus-ab/scorecard-dry"
	_ = os.RemoveAll(out)
	defer os.RemoveAll(out)
	_ = os.MkdirAll(out, 0o755)
	f := &fakeC{}
	scCompleter = func() render.Completer { return f }
	runSC(ctx, "sc-gen", 65, out, "")
	runSC(ctx, "sc-run", 65, out, "orig-chibi,l-haejin")
	runSC(ctx, "sc-old", 65, out, "d1-chibi,l-haejin")
	runSC(ctx, "sc-report", 65, out, "")
	fmt.Println("dry-run ok; fake calls:", f.n)
}
