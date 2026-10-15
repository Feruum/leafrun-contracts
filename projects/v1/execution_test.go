package projectsv1

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecutionFingerprintIsSeparateAndFrozen(t *testing.T) {
	request := selectionFixture(t)
	execution := resolvedFixture(t, request)
	data, err := json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	_, canonical, hash, err := NormalizeExecution(data)
	if err != nil {
		t.Fatal(err)
	}
	if hash == request.SelectionFingerprint {
		t.Fatal("selection and execution identities conflated")
	}
	_, again, againHash, err := NormalizeExecution(canonical)
	if err != nil || !bytes.Equal(again, canonical) || againHash != hash {
		t.Fatal("execution canonical form is unstable")
	}
	execution.Descriptor.ExecutionDigest = strings.Repeat("d", 64)
	data, _ = json.Marshal(execution)
	_, _, changed, err := NormalizeExecution(data)
	if err != nil || changed == hash {
		t.Fatal("resolved profile digest not fingerprinted")
	}
}
