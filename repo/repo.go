package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/tiagovtristao/plue/targetlookup"
)

type ExtensionConfig struct {
	SourceFileCriteriaLookup []targetlookup.FileCriteriaLookupCall `json:"sourceFileCriteriaLookup"`
	DepsResolver             string                                `json:"depsResolver"`
}

type Repo struct {
	Repo             string                     `json:"repo"`
	ExtensionsConfig map[string]ExtensionConfig `json:"extensionsConfig"`
}

func (r *Repo) FullFilePath(filePath string) string {
	relativeFilePath := r.RelativeFilePath(filePath)

	return filepath.Join(r.Repo+"/", relativeFilePath)
}

func (r *Repo) RelativeFilePath(filePath string) string {
	return strings.TrimPrefix(filePath, r.Repo+"/")
}

func (r *Repo) GetFileExtConfig(filePath string) (ExtensionConfig, error) {
	sourceFileExtension := filepath.Ext(filePath)

	config, exists := r.ExtensionsConfig[sourceFileExtension]
	if !exists {
		return ExtensionConfig{}, errors.New("File type not supported")
	}

	return config, nil
}

func (r *Repo) SupportsFileType(filePath string) bool {
	_, err := r.GetFileExtConfig(filePath)

	return err == nil
}

func (r *Repo) NewSourceFileCriteria(filePath string) (*targetlookup.Criteria, error) {
	extConfig, err := r.GetFileExtConfig(filePath)
	if err != nil {
		return nil, err
	}

	fileCriteria := &targetlookup.FileCriteria{
		ImportID: "",
		Lookup: targetlookup.FileCriteriaLookup{
			File:  r.RelativeFilePath(filePath),
			Calls: extConfig.SourceFileCriteriaLookup,
		},
	}
	var c targetlookup.Criteria = fileCriteria
	return &c, nil
}

func (r *Repo) ResolveDeps(filePath string) ([]*targetlookup.Criteria, error) {
	fullFilePath := r.FullFilePath(filePath)

	extConfig, err := r.GetFileExtConfig(fullFilePath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(extConfig.DepsResolver, fullFilePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("REPO=%s", r.Repo))

	data, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	criteriaList := make([]*targetlookup.Criteria, 0)

	// TODO: Handle remaining error paths
	jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		t, err := jsonparser.GetString(value, "type")
		if err != nil {
			log.Fatal(err)
		}

		v := jsonparser.Delete(value, "type")

		switch t {
		case "file":
			var fileCriteria targetlookup.FileCriteria

			err := json.Unmarshal(v, &fileCriteria)
			if err != nil {
				log.Fatal(err)
			}

			var c targetlookup.Criteria = &fileCriteria
			criteriaList = append(criteriaList, &c)

		case "package":
			var pkgCriteria targetlookup.PackageCriteria

			err := json.Unmarshal(v, &pkgCriteria)
			if err != nil {
				log.Fatal(err)
			}

			var c targetlookup.Criteria = &pkgCriteria
			criteriaList = append(criteriaList, &c)

		default:
			log.Fatalf("Invalid type: %s", t)
		}
	})

	return criteriaList, nil
}
