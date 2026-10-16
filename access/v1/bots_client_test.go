package accessv1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBotClientRejectsMalformedKeyMetadata(t *testing.T) {
	for _, mutate := range []func(*BotKeyResult){func(r *BotKeyResult) { r.Key.Status = "invented" }, func(r *BotKeyResult) { r.Key.Version = 1 << 63 }, func(r *BotKeyResult) { r.Key.Ceiling.Operations = []string{"organization.create"} }, func(r *BotKeyResult) { r.Key.KeyID = "wrong-target" }} {
		out := BotKeyResult{Key: BotKey{BotID: "b", KeyID: "k", Version: 2, Status: "active", Ceiling: CredentialCeiling{Scope: PermissionScope{OrganizationID: "o", WorkspaceID: "w", ProjectID: "p"}, Operations: []string{"generation.create"}}}, Disclosure: "unavailable"}
		mutate(&out)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(out) }))
		client, _ := NewSessionClient(server.URL, "session", nil)
		_, err := client.RotateBotKey(context.Background(), ResolveRequest{Bearer: "human"}, "b", "k", RotateBotKeyRequest{OperationKey: "rotate", ExpectedVersion: 1, Operations: []string{"generation.create"}})
		server.Close()
		if !errors.Is(err, ErrDependency) {
			t.Fatal("malformed or foreign bot key metadata accepted")
		}
	}
}
