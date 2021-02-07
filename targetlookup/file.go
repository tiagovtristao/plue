package targetlookup

import (
	"path/filepath"
	"strings"

	"github.com/tiagovtristao/plz/src/core"
	"github.com/tiagovtristao/plz/src/parse/snapshot"
)

type FileCriteriaLookupCall struct {
	Id    string `json:"id"`
	Srcs  string `json:"srcs"`
	Deps  string `json:"deps"`
	Label string `json:"label"`
}

type FileCriteriaLookup struct {
	File  string                   `json:"file"`
	Calls []FileCriteriaLookupCall `json:"calls"`
}

type FileCriteria struct {
	ImportID string             `json:"importId"`
	Lookup   FileCriteriaLookup `json:"lookup"`
}

func (c *FileCriteria) Packages(plzState *core.BuildState) []core.BuildLabel {
	return []core.BuildLabel{core.FindOwningPackage(plzState, c.Lookup.File)}
}

func (c *FileCriteria) Find(s snapshot.Interpreter) *ResolvedLookup {
	if s.InitialisedCall == nil {
		return nil
	}

	pkgName := filepath.Dir(s.BuildFileName)
	relativeLookupFile := strings.TrimPrefix(c.Lookup.File, pkgName+"/")

	containsFile := func(srcsKey string) bool {
		if srcs, exists := s.InitialisedCall.Args[srcsKey]; exists {
			if srcs.Str != nil {
				if srcs.Str.Value == relativeLookupFile {
					return true
				}
			} else if srcs.StrList != nil {
				for _, v := range srcs.StrList.Value {
					if v == relativeLookupFile {
						return true
					}
				}
			}
			// TODO: Dict type missing
		}

		return false
	}

	for _, call := range c.Lookup.Calls {
		if call.Id == s.InitialisedCall.Name {
			if containsFile(call.Srcs) {
				return &ResolvedLookup{
					Label: core.BuildLabel{
						PackageName: pkgName,
						Name:        s.InitialisedCall.Args[call.Label].Str.Value,
					},
					Lookup:   call,
					Snapshot: s,
				}
			}
		}
	}

	return nil
}
