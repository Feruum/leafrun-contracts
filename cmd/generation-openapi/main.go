// Command generation-openapi emits the Generation v1 HTTP specification.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
)

func main() {
	data, err := json.MarshalIndent(generationv1.OpenAPI(), "", "  ")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if len(os.Args) == 2 {
		if err = os.WriteFile(os.Args[1], data, 0644); err != nil {
			panic(err)
		}
		return
	}
	fmt.Print(string(data))
}
