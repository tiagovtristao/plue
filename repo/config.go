package repo

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"
)

func ParseConfig(fileName string) *Repo {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	var repo Repo
	if err := json.Unmarshal(content, &repo); err != nil {
		log.Fatal(err)
	}

	// Trim possible trailing '/' character for normalisation
	repo.Repo = strings.TrimSuffix(repo.Repo, "/")

	return &repo
}
