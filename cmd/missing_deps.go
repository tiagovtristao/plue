package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/tiagovtristao/plue/repo"
	"github.com/tiagovtristao/plue/targetlookup"
	"github.com/tiagovtristao/plz/src/cli"
	"github.com/tiagovtristao/plz/src/core"
	"github.com/tiagovtristao/plz/src/fs"
	"github.com/tiagovtristao/plz/src/output"
	"github.com/tiagovtristao/plz/src/parse"
	"github.com/tiagovtristao/plz/src/plz"
)

var missingDeps = &cobra.Command{
	Use:   "missing-deps file",
	Short: "TODO",
	Long:  `TODO`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFile := args[0]

		repoFile := cmd.Flag("repo-config").Value.String()
		repo := repo.ParseConfig(repoFile)

		sourceFileFullPath := repo.FullFilePath(sourceFile)

		if !fs.FileExists(sourceFileFullPath) {
			log.Fatalf("File '%s' doesn't exist", sourceFileFullPath)
		} else if !repo.SupportsFileType(sourceFileFullPath) {
			log.Fatalf("Support for '%s' file type hasn't been set up", sourceFileFullPath)
		}

		// Get source file criteria
		sourceFileCriteria, err := repo.NewSourceFileCriteria(sourceFileFullPath)
		if err != nil {
			log.Fatal(err)
		}
		// Get deps' criteria list
		criteriaList, err := repo.ResolveDeps(sourceFileFullPath)
		if err != nil {
			log.Fatal(err)
		}
		// Append source file criteria to criteria list
		criteriaList = append(criteriaList, sourceFileCriteria)

		// Init Please
		plzConfig, plzState := initPlz(repo)

		// 1. It finds out which packages need parsing
		// 2. Links lookup criteria to related packages
		pkgTargets := make([]core.BuildLabel, 0)
		pkgLookups := make(map[string][]*targetlookup.Criteria)
		for _, criteria := range criteriaList {
			for _, pkg := range (*criteria).Packages(plzState) {
				buildFileName := core.FindPackageFileName(plzState, pkg)

				if _, exists := pkgLookups[buildFileName]; !exists {
					pkgTargets = append(pkgTargets, pkg)
					pkgLookups[buildFileName] = make([]*targetlookup.Criteria, 0)
				}

				pkgLookups[buildFileName] = append(pkgLookups[buildFileName], criteria)
			}
		}

		// Keeps track of found criteria
		foundCriteria := make(map[*targetlookup.Criteria]*targetlookup.ResolvedLookup)

		var outerWg sync.WaitGroup
		outerWg.Add(2)
		snapshotsInitialised := make(chan bool)
		go func() {
			// This is necessary as it sets up the snapshots channel
			parse.InitParser(plzState)
			// Initialise snapshots
			snapshots := plzState.Parser.InterpreterSnapshots()
			snapshotsInitialised <- true

			// Listens to snapshot information to match it against the expected criteria
			for snapshot := range snapshots {
				snapshot.BuildFileName = repo.RelativeFilePath(snapshot.BuildFileName)

				if lookups, exists := pkgLookups[snapshot.BuildFileName]; exists {
					for _, criteria := range lookups {
						if _, exists := foundCriteria[criteria]; !exists {
							foundLookup := (*criteria).Find(snapshot)

							if foundLookup != nil {
								foundCriteria[criteria] = foundLookup
							}
						}
					}
				}
			}

			outerWg.Done()
		}()

		go func(){
			<- snapshotsInitialised

			// Run Please
			plzState.Results()
			var innerWg sync.WaitGroup
			innerWg.Add(1)
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				output.MonitorState(ctx, plzState, true, false, false, "")
				innerWg.Done()
			}()
			plz.Run(pkgTargets, nil, plzState, plzConfig, cli.Arch{})
			cancel()
			innerWg.Wait()

			outerWg.Done()
		}()
		outerWg.Wait()

		// Bail out if it couldn't find all of them
		for _, c := range criteriaList {
			if _, found := foundCriteria[c]; !found {
				// TODO: Display better human readable info
				log.Fatalf("Criteria not found: %+v\n", *c)
			}
		}

		// Get source file's listed deps
		sourceFileResolved, _ := foundCriteria[sourceFileCriteria]
		sourceFileLookup := sourceFileResolved.Lookup.(targetlookup.FileCriteriaLookupCall)
		sourceFileListedDeps := sourceFileResolved.GetDeps(sourceFileLookup.Deps)

		fmt.Printf("# Source file's BUILD listed dependency targets:\n")
		for _, dep := range sourceFileListedDeps {
			fmt.Printf("%v\n", dep)
		}

		// Generate mapping between imports/dependencies and associated targets
		importTargets := make(map[string]core.BuildLabel)
		for criteria, resolved := range foundCriteria {
			if criteria != sourceFileCriteria {
				switch lookup := resolved.Lookup.(type) {
				case targetlookup.FileCriteriaLookupCall:
					label := resolved.GetLabel(lookup.Label)
					fileCriteria, _ := (*criteria).(*targetlookup.FileCriteria)

					importTargets[fileCriteria.ImportID] = core.BuildLabel{PackageName: resolved.Label.PackageName, Name: label}

				case targetlookup.PackageCriteriaLookup:
					label := resolved.GetLabel(lookup.Call.Label)
					pkgCriteria, _ := (*criteria).(*targetlookup.PackageCriteria)

					importTargets[pkgCriteria.ImportID] = core.BuildLabel{PackageName: resolved.Label.PackageName, Name: label}

				default:
					log.Fatalf("Unsupported lookup type: %T", lookup)
				}
			}
		}

		missingDepTargets := dedupTargetsMap(importTargets, sourceFileListedDeps)

		fmt.Printf("\n# Source files's missing dependency targets:\n")
		for _, t := range missingDepTargets {
			fmt.Printf("\"%s\",\n", t)
		}
	},
}

func initPlz(r *repo.Repo) (*core.Configuration, *core.BuildState) {
	if err := os.Chdir(r.Repo); err != nil {
		log.Fatal(err)
	}

	core.RepoRoot = r.Repo

	cli.InitLogging(cli.MinVerbosity)

	plzConfig, err := core.ReadDefaultConfigFiles([]string{})
	plzConfig.FeatureFlags.RemovePleasings = true
	if err != nil {
		log.Fatalf("Error reading Please config file: %s", err)
	}
	plzConfig.Display.SystemStats = false

	plzState := core.NewBuildState(plzConfig)
	plzState.NeedBuild = false
	plzState.DownloadOutputs = true
	plzState.CleanWorkdirs = true

	return plzConfig, plzState
}

func dedupTargetsMap(targetsMap map[string]core.BuildLabel, targetList []string) map[string]core.BuildLabel {
	isTargetListed := func(target core.BuildLabel) bool {
		for _, t := range targetList {
			if t == target.String() {
				return true
			}
		}
		return false
	}

	deduped := make(map[string]core.BuildLabel)
	for k, t := range targetsMap {
		if !isTargetListed(t) {
			deduped[k] = t
		}
	}

	return deduped
}
