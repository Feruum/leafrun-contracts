package main

import (
	"encoding/json"
	auditv1 "github.com/leafrun/leafrun-contracts/audit/v1"
	"os"
)

func main() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if encoder.Encode(auditv1.NewOpenAPI()) != nil {
		os.Exit(1)
	}
}
