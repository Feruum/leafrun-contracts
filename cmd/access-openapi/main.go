package main

import (
	"encoding/json"
	accessv1 "github.com/leafrun/leafrun-contracts/access/v1"
	"os"
)

func main() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(accessv1.NewOpenAPI()); err != nil {
		panic(err)
	}
}
