package accessv1

import (
	"reflect"
	"sort"
	"testing"
)

func TestOAuthScopeCeilingsAreExactClosedRestrictions(t *testing.T) {
	for scope, operations := range map[string][]string{
		"leafrun.generate": {"generation.create", "generation.execute", "generation.publish", "generation.cancel"},
		"leafrun.status":   {"generation.read"}, "leafrun.source.read": {"source.read"},
		"leafrun.resource.read": {"resource.read"}, "leafrun.download": {"artifact.read", "download"},
		"offline_access": {},
	} {
		t.Run(scope, func(t *testing.T) {
			ceiling, err := OAuthCeiling([]string{scope})
			sort.Strings(operations)
			if err != nil || ceiling == nil || !reflect.DeepEqual(ceiling.Operations, operations) {
				t.Fatal("scope operation set changed")
			}
			for _, op := range []string{"role.manage", "bot.manage", "grant.manage", "audit.read", "organization.create", "project.read", "project.publish"} {
				if ceiling.Allows(PermissionScope{}, op) {
					t.Fatal("scope grants management")
				}
			}
		})
	}
}

func TestOAuthScopesOnlyRestrictOperations(t *testing.T) {
	c, err := OAuthCeiling([]string{"leafrun.generate", "offline_access"})
	if err != nil || c == nil || !c.Allows(PermissionScope{}, "generation.execute") {
		t.Fatal("generate denied")
	}
	for _, op := range []string{"role.manage", "organization.create", "project.read", "source.read", "resource.read", "download", "generation.read"} {
		if c.Allows(PermissionScope{}, op) {
			t.Fatalf("scope escaped: %s", op)
		}
	}
	for _, scopes := range [][]string{{"admin"}, {"leafrun.generate", "leafrun.generate"}, nil} {
		if _, err := OAuthCeiling(scopes); err == nil {
			t.Fatal("invalid scope accepted")
		}
	}
}
