package main

import (
	"encoding/json"
	projectsv1 "github.com/leafrun/leafrun-contracts/projects/v1"
	"os"
)

func main() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(projectsv1.OpenAPI()); err != nil {
		panic(err)
	}
}
