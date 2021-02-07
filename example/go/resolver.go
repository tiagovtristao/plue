package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/tiagovtristao/plue/targetlookup"
)

type FullPackageCriteria struct {
	Type     string                               `json:"type"`
	ImportID string                               `json:"importId"`
	Lookups  []targetlookup.PackageCriteriaLookup `json:"lookups"`
}

func main() {
	repo := os.Getenv("REPO")

	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatalln("A file is required")
	}

	file := flag.Arg(0)

	// Parse file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
	if err != nil {
		log.Fatal(err)
	}

	// Get Go's standard library's packages
	pkgs, err := packages.Load(nil, "std")
	if err != nil {
		log.Fatal(err)
	}

	// Create map of standard libraries' paths
	pkgPaths := make(map[string]bool)
	for _, p := range pkgs {
		pkgPaths[p.PkgPath] = true
	}

	criteriaList := make([]FullPackageCriteria, 0)

	// Iterate through file imports
	for _, t := range f.Imports {
		// Trim double quotes from the import tokens
		pkg := strings.Trim(t.Path.Value, `"`)

		if _, isStdPkg := pkgPaths[pkg]; !isStdPkg {
			p := filepath.Join(repo, pkg)

			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				criteriaList = append(criteriaList, FullPackageCriteria{
					Type:     "package",
					ImportID: pkg,
					Lookups: []targetlookup.PackageCriteriaLookup{{
						Package: pkg,
						Call: targetlookup.PackageCriteriaLookupCall{
							Id: "go_library",
							Args: map[string]string{
								"name": fmt.Sprintf("^%s$", filepath.Base(pkg)),
							},
							Label: "name",
						},
					}},
				})
			} else if strings.HasPrefix(pkg, "github.com") {
				criteriaList = append(criteriaList, FullPackageCriteria{
					Type:     "package",
					ImportID: pkg,
					Lookups: []targetlookup.PackageCriteriaLookup{{
						Package: "third_party/go",
						Call: targetlookup.PackageCriteriaLookupCall{
							Id: "go_get",
							Args: map[string]string{
								"get": fmt.Sprintf("^%s", pkg),
							},
							Label: "name",
						},
					}},
				})
			} else {
				log.Fatalf("Unable to resolve: %s", pkg)
			}
		}
	}

	b, err := json.Marshal(criteriaList)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(b))
}
