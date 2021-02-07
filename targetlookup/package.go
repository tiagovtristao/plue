package targetlookup

import (
	"path/filepath"
	"regexp"

	"github.com/tiagovtristao/plz/src/core"
	"github.com/tiagovtristao/plz/src/parse/snapshot"
)

type PackageCriteriaLookupCall struct {
	Id    string            `json:"id"`
	Args  map[string]string `json:"args"`
	Label string            `json:"label"`
}

type PackageCriteriaLookup struct {
	Package string                    `json:"package"`
	Call    PackageCriteriaLookupCall `json:"call"`
}

type PackageCriteria struct {
	ImportID string                  `json:"importId"`
	Lookups  []PackageCriteriaLookup `json:"lookups"`
}

func (c *PackageCriteria) Packages(plzState *core.BuildState) []core.BuildLabel {
	pkgs := make([]core.BuildLabel, 0, len(c.Lookups))

	for _, l := range c.Lookups {
		pkgs = append(pkgs, core.BuildLabel{PackageName: l.Package, Name: "all"})
	}

	return pkgs
}

func (c *PackageCriteria) Find(s snapshot.Interpreter) *ResolvedLookup {
	if s.InitialisedCall == nil {
		return nil
	}

	pkgName := filepath.Dir(s.BuildFileName)

	matchingArgs := func(args map[string]string) bool {
		for key, value := range args {
			snapValue, exists := s.InitialisedCall.Args[key]
			if !exists {
				break
			}

			if snapValue.Str != nil {
				if matchesRegex(value, snapValue.Str.Value) {
					return true
				}
			} else if snapValue.StrList != nil {
				for _, v := range snapValue.StrList.Value {
					if matchesRegex(value, v) {
						return true
					}
				}
			}
			// TODO: Dict type missing

			break
		}

		return false
	}

	for _, lookup := range c.Lookups {
		if lookup.Package == pkgName {
			if lookup.Call.Id == s.InitialisedCall.Name {
				if matchingArgs(lookup.Call.Args) {
					return &ResolvedLookup{
						Label: core.BuildLabel{
							PackageName: pkgName,
							Name:        s.InitialisedCall.Args[lookup.Call.Label].Str.Value,
						},
						Lookup:   lookup,
						Snapshot: s,
					}
				}
			}
		}
	}

	return nil
}

func matchesRegex(input string, value string) bool {
	if matches, _ := regexp.MatchString(input, value); matches {
		return true
	}
	return false
}
