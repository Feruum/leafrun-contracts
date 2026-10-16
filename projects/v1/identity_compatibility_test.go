package projectsv1

import (
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"testing"
)

func TestE05PrincipalAliasPreservesExecutionGolden(t *testing.T) {
	request := selectionFixture(t)
	execution := resolvedFixture(t, request)
	// These hashes were measured at the E05 baseline a7e06b7, independently of E06.
	const selectionGolden = "47440c507366c08b53ce8354ffe6ee35c650e9f0b9c03474449072f04e7dd24a"
	const executionGolden = "003fcc2088e8df1b88c15bf6e2e60013be42147c59936870c382800bbd5f4fe6"
	if request.SelectionFingerprint != selectionGolden {
		t.Fatal("E05 selection changed")
	}
	var principal identityv1.Principal = execution.Actor
	if principal != (identityv1.Principal{Subject: "alice", Domain: "user"}) {
		t.Fatal("stable principal is not comparable")
	}
	data, _ := json.Marshal(execution)
	_, _, hash, err := NormalizeExecution(data)
	if err != nil || hash != executionGolden {
		t.Fatalf("E05 execution changed: %s %v", hash, err)
	}
}
