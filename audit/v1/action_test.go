package auditv1

import (
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"strings"
	"testing"
)

func actionFixture() Action {
	return Action{SchemaVersion: 1, Producer: ProducerStorage, Actor: identityv1.Principal{Subject: "alice", Domain: "user"}, Authority: identityv1.AuthorityRef{Kind: identityv1.KindKratosSession, ID: "session-1", Version: 1}, OperationID: "op-1", Action: ActionDownloadAuthorized, Resource: ResourceRef{Kind: ResourceArtifact, ID: "artifact-1"}, Outcome: OutcomeSucceeded, Revision: "rev-1"}
}
func TestActionRoundTripAndExactSubjects(t *testing.T) {
	for _, tc := range []struct {
		producer Producer
		action   ActionName
		subject  string
	}{{ProducerAccess, ActionIdentityResolve, "leafrun.audit.access.action.v1"}, {ProducerProjects, ActionReleasePublish, "leafrun.audit.projects.action.v1"}, {ProducerGeneration, ActionGenerationAdmission, "leafrun.audit.generation.action.v1"}, {ProducerStorage, ActionDownloadAuthorized, "leafrun.audit.storage.action.v1"}} {
		a := actionFixture()
		a.Producer = tc.producer
		a.Action = tc.action
		subject, err := Subject(a.Producer)
		if err != nil || subject != tc.subject {
			t.Fatalf("subject %s %v", subject, err)
		}
		data, err := EncodeAction(a)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeAction(subject, data)
		if err != nil || got != a {
			t.Fatalf("roundtrip %#v %v", got, err)
		}
	}
}
func TestActionRejectsInvalidEnumsVersionsAndProducerMismatch(t *testing.T) {
	subject, _ := Subject(ProducerStorage)
	for _, change := range []func(*Action){
		func(a *Action) { a.SchemaVersion = 0 }, func(a *Action) { a.SchemaVersion = 2 }, func(a *Action) { a.Producer = "audit" },
		func(a *Action) { a.Action = "download.completed" }, func(a *Action) { a.Producer = ProducerAccess }, func(a *Action) { a.Action = ActionReleaseRetire },
		func(a *Action) { a.Resource.Kind = "arbitrary" }, func(a *Action) { a.Resource.ID = "" }, func(a *Action) { a.Outcome = "arbitrary" }, func(a *Action) { a.OperationID = "\n" }, func(a *Action) { a.Revision = strings.Repeat("x", 257) }, func(a *Action) { a.Authority.Version = 1 << 63 }, func(a *Action) { a.Actor.Subject = "" },
	} {
		a := actionFixture()
		change(&a)
		if a.Validate(subject) == nil {
			t.Fatalf("accepted %#v", a)
		}
	}
	a := actionFixture()
	for _, bad := range []string{"leafrun.audit.storage.action.v2", "leafrun.audit.projects.action.v1", "leafrun.audit.storage.action.v1.extra"} {
		if a.Validate(bad) == nil {
			t.Fatal("bad subject accepted")
		}
	}
	if _, err := Subject("audit"); err == nil {
		t.Fatal("fifth producer accepted")
	}
}
func TestActionStrictlyRejectsSecretFieldsAndMalformedJSON(t *testing.T) {
	a := actionFixture()
	data, _ := json.Marshal(a)
	subject, _ := Subject(a.Producer)
	for _, field := range []string{"data", "parameter", "source", "email", "token", "password", "secret", "message", "error", "secret-as-field-name"} {
		bad := strings.TrimSuffix(string(data), "}") + `,"` + field + `":"sensitive"}`
		if _, err := DecodeAction(subject, []byte(bad)); err == nil || strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "secret-as-field-name") {
			t.Fatalf("accepted or echoed forbidden field: %v", err)
		}
	}
	for _, bad := range []string{string(data) + ` {}`, strings.Replace(string(data), `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1), strings.Replace(string(data), `"subject":"alice"`, `"subject":"alice","email":"sensitive"`, 1), strings.Replace(string(data), `"kind":"artifact"`, `"kind":"artifact","source":"sensitive"`, 1), strings.Replace(string(data), `"id":"session-1"`, `"id":"session-1","token":"sensitive"`, 1), strings.Replace(string(data), `"operationId":"op-1"`, `"operationId":"\ud800"`, 1), "{\"operationId\":\"\xff\"}", string(data) + strings.Repeat(" ", MaxActionBytes)} {
		if _, err := DecodeAction(subject, []byte(bad)); err == nil {
			t.Fatalf("accepted malformed JSON %q", bad)
		}
	}
}
