package main

import (
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
	"reflect"
	"testing"
)

func TestStorageAccessAuthoritySchema(t *testing.T) {
	b := &schemaBuilder{schemas: map[string]any{}}
	b.schema(reflect.TypeFor[storagev1.AccessCheck]())
	properties := b.schemas["AccessCheck"].(map[string]any)["properties"].(map[string]any)
	if properties["authority"] == nil {
		t.Fatal("authority missing")
	}
	authority := b.schemas["AuthorityRef"].(map[string]any)["properties"].(map[string]any)
	if authority["version"].(map[string]any)["maximum"] != identityv1.MaxAuthorityVersion {
		t.Fatal("version is not bounded PostgreSQL bigint")
	}
	if kinds, ok := authority["kind"].(map[string]any)["enum"].([]string); !ok || len(kinds) != 6 {
		t.Fatal("authority kind enum missing")
	}
	if authority["id"].(map[string]any)["x-max-utf8-bytes"] != identityv1.MaxIdentifierBytes {
		t.Fatal("authority ID byte limit missing")
	}
	if b.schemas["AuthorityRef"].(map[string]any)["additionalProperties"] != false {
		t.Fatal("authority reference schema permits secrets")
	}
}
