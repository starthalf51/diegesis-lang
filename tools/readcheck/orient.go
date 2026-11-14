package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// orientPrompt is the first-time-reader quiz for an opening episode (what did the reader learn?).
const orientPrompt = `You are a first-time reader of this novel. Below is the whole episode. Answer only from what you read in it. If you are guessing, write "guess"; if you do not know, write "unknown". Answer in the language of the text.
1. What is the protagonist's name?
2. What is the protagonist's situation (family, standing, what is at stake now)?
3. What kind of world is this, and what are its basic rules?
4. At the end of the episode, what does the protagonist want or fear?
5. Do you want to read the next episode? Why? (yes/no + one line)
6. List every piece of information or description that connected to nothing and felt meaningless (quote them).
Reply with JSON only: {"name":"","situation":"","world":"","want":"","next":"","dangling":[""]}
---
`

type orientRead struct {
	Name      *string   `json:"name"`
	Situation *string   `json:"situation"`
	World     *string   `json:"world"`
	Want      *string   `json:"want"`
	Next      *string   `json:"next"`
	Dangling  *[]string `json:"dangling"`
}

// parseOrient takes the first {...} object in s that carries all six keys.
func parseOrient(s string) (orientRead, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		var o orientRead
		if json.NewDecoder(strings.NewReader(s[i:])).Decode(&o) == nil &&
			o.Name != nil && o.Situation != nil && o.World != nil && o.Want != nil && o.Next != nil && o.Dangling != nil {
			return o, true
		}
	}
	return orientRead{}, false
}

func runOrientMode(file, model string, budget int, out io.Writer) int {
	b, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	if budget == 0 {
		budget = 2
	}
	g := &gate{model: model, budget: budget}
	var o orientRead
	if !g.try("orient", orientPrompt+string(b), func(s string) bool { var k bool; o, k = parseOrient(s); return k }) {
		fmt.Fprintf(out, "%s UNDECIDED (unparsable or budget exhausted)\n", file)
		return 2
	}
	*o.Dangling = slices.DeleteFunc(*o.Dangling, func(s string) bool { return strings.TrimSpace(s) == "" }) // the prompt's template echo is not an item
	jb, _ := json.MarshalIndent(o, "", " ")
	dest := file + ".orient.json"
	if err := os.WriteFile(dest, jb, 0o644); err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	fmt.Fprintf(out, "orient %s dangling=%d\nname: %s\nsituation: %s\nworld: %s\nwant: %s\n",
		file, len(*o.Dangling), *o.Name, *o.Situation, *o.World, *o.Want)
	return 0
}
