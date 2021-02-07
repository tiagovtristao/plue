package targetlookup

import (
	"github.com/tiagovtristao/plz/src/core"
	"github.com/tiagovtristao/plz/src/parse/snapshot"
)

type Criteria interface {
	Packages(*core.BuildState) []core.BuildLabel
	Find(snapshot.Interpreter) *ResolvedLookup
}

type ResolvedLookup struct {
	Label    core.BuildLabel
	Lookup   interface{}
	Snapshot snapshot.Interpreter
}

func (rl *ResolvedLookup) GetLabel(name string) string {
	ic := rl.Snapshot.InitialisedCall

	if ic != nil {
		// Expected to be string
		return ic.Args[name].Str.Value
	}

	return ""
}

func (rl *ResolvedLookup) GetDeps(name string) []string {
	ic := rl.Snapshot.InitialisedCall

	if ic != nil {
		// Expected to be a string list
		return ic.Args[name].StrList.Value
	}

	return nil
}
