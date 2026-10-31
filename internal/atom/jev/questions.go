// Package jev is the selector that asks Jev narrow judgments about a task and combines them into an offset from base.
package jev

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
)

// Question is one entry of a /v1/systemone request's question set.
type Question struct {
	Type, Instructions string
	Criteria           any
}

// Answers holds Jev's decoded judgments: Scores normalised to 0..1, Nouls as probabilities.
type Answers struct {
	Kind                                      string
	KindConfidence                            float64
	Depth, Ambiguity, Blast, Novelty, Breadth float64
	Check, Mechanical, Design, Judge, Chain   float64
	// Fallback is set when the verdict is the fallback's; Kind is "" when Jev gave no answers.
	Fallback bool
}

// Other is the kind option for work no kind fits; Explain falls back on it.
const Other = "other"

// Option is one choice option: a name and its description (nil is JSON null).
type Option struct {
	Name string
	Desc any
}

// Options is a choice's criteria sent as a JSON object in slice order (Jev favors the first
// option, so the order is the kinds' order, never alphabetical).
type Options []Option

// MarshalJSON writes the options as one object, keys in slice order.
func (o Options) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, opt := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(opt.Name)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(opt.Desc)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// has reports an option named name.
func (o Options) has(name string) bool {
	return slices.ContainsFunc(o, func(opt Option) bool { return opt.Name == name })
}

// builtinKinds is the configured order of the built-in kinds (R-MOD-008), cheapest first.
var builtinKinds = []string{"search", "run", "implement", "debug", "research", "review", "verify", "design"}

// KindOrder is kinds in their configured order: the built-in kinds first, then project kinds by name.
func KindOrder(kinds map[string]int) []string {
	var out []string
	for _, k := range builtinKinds {
		if _, ok := kinds[k]; ok {
			out = append(out, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(kinds)) {
		if !slices.Contains(builtinKinds, k) {
			out = append(out, k)
		}
	}
	return out
}

// Questions returns the request's question set, the kind choice offering kinds in order then Other.
func Questions(kinds []string) map[string]Question {
	opts := make(Options, 0, len(kinds)+1)
	for _, k := range kinds {
		opts = append(opts, Option{k, kindCriteria[k]}) // nil (JSON null) for a project kind with no description
	}
	opts = append(opts, Option{Other, "none of the above kinds fits"})
	qs := map[string]Question{
		"kind": {Type: "choice", Criteria: opts, Instructions: "Which kind of work does `task` ask for? " +
			"Judge the work itself, not any label in `declared_kind`."},
		"depth": {Type: "score", Instructions: "How much reasoning does `task` need?", Criteria: []string{
			"None: the steps are fully given; follow them.",
			"Light: a few straightforward decisions.",
			"Substantial: several interacting decisions or a non-obvious approach.",
			"Deep: hard reasoning about subtle interactions, concurrency, recovery or correctness.",
		}},
		"ambiguity": {Type: "score", Instructions: "How fully specified is `task`?", Criteria: []string{
			"`task` states the goal, the steps and the done condition; nothing is left to choose.",
			"`task` states the goal and the done condition but leaves some details for the worker to choose.",
			"`task` leaves the goal itself unclear: the worker must decide what is wanted.",
		}},
		"blast": {Type: "score", Instructions: "What could a mistake in `task` affect?", Criteria: []string{
			"Local and reversible.",
			"Several modules or shared state, reversible.",
			"Irreversible or external effects (data loss, publishing, money, migrations).",
		}},
		"novelty": {
			Type: "score", Instructions: "How new is the work `task` asks for, as described in `task`?",
			Criteria: []string{
				"`task` names an existing example to follow.",
				"`task` names a related pattern to adapt.",
				"`task` asks for something with no example or pattern named.",
			},
		},
		"breadth": {
			Type: "score", Instructions: "How much does `task` touch, as described in `task`?",
			Criteria: []string{
				"`task` names one file.",
				"`task` names one package or directory.",
				"`task` names or implies several packages or documents.",
			},
		},
		"check": noul("Does `task` name or clearly imply an automated check (tests, build, a " +
			"script's exit status) that decides whether the work succeeded?"),
		"mechanical": noul("Is `task` mechanical: reading, searching, running given commands or " +
			"transcribing, with no design decisions?"),
		"design": noul("Does `task` require choosing between several viable designs or approaches?"),
		"judge":  noul("Is the output of `task` a judgment (a verdict, a review, a conclusion)?"),
		"chain":  noul("Does `task` require a long chain of steps that each depend on the one before?"),
	}
	return qs
}

// kindCriteria describes each built-in kind for the kind choice: what it covers and excludes.
var kindCriteria = map[string]map[string]string{
	"search": {
		"covers":   "find, map, list or read code/docs",
		"excludes": "any edits",
	},
	"run": {
		"covers":   "execute a specified procedure (tests, experiments, data collection) and report",
		"excludes": "deciding what to run or changing code",
	},
	"implement": {
		"covers":   "edit code or docs to a written spec with a check",
		"excludes": "choosing the design",
	},
	"debug": {
		"covers":   "find the cause of a failure",
		"excludes": "work with no failure to explain",
	},
	"research": {
		"covers":   "gather and synthesize external or cross-cutting evidence",
		"excludes": "editing code",
	},
	"review": {
		"covers":   "judge a change against a plan or rules",
		"excludes": "making the change",
	},
	"verify": {
		"covers":   "try to prove one claim wrong",
		"excludes": "judging a whole change",
	},
	"design": {
		"covers":   "choose between alternative designs or create a new mechanism",
		"excludes": "implementing a settled design",
	},
}

// noul is a yes/no question asking instructions.
func noul(instructions string) Question { return Question{Type: "noul", Instructions: instructions} }
