package accessv1

import (
	"reflect"
	"strings"
)

func botsOpenAPI(b *schemaBuilder, paths map[string]any) {
	for _, r := range []struct {
		method, path, id string
		in, out          reflect.Type
	}{
		{"post", "/v1/workspaces/{workspace}/projects/{project}/bots", "createBot", reflect.TypeFor[CreateBotRequest](), reflect.TypeFor[Bot]()},
		{"post", "/v1/bots/{bot}/keys", "issueBotKey", reflect.TypeFor[BotKeyRequest](), reflect.TypeFor[BotKeyResult]()},
		{"post", "/v1/bots/{bot}/keys/{key}/rotate", "rotateBotKey", reflect.TypeFor[RotateBotKeyRequest](), reflect.TypeFor[BotKeyResult]()},
		{"delete", "/v1/bots/{bot}/keys/{key}", "revokeBotKey", reflect.TypeFor[RevokeBotKeyRequest](), reflect.TypeFor[BotKeyResult]()},
	} {
		params := []any{}
		for _, part := range strings.Split(r.path, "/") {
			if strings.HasPrefix(part, "{") {
				params = append(params, map[string]any{"name": strings.Trim(part, "{}"), "in": "path", "required": true, "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "x-max-utf8-bytes": 256}})
			}
		}
		responses := map[string]any{"200": map[string]any{"description": "Committed public result; secret only on first successful issuance/rotation disclosure. Replay returns original metadata with disclosure=unavailable.", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(r.out)}}}}
		for _, code := range []string{"400", "401", "403", "409", "503"} {
			responses[code] = map[string]any{"description": "Invalid input, unauthenticated, denied, changed-input/stale-version conflict, or dependency uncertainty; no secret-bearing errors."}
		}
		op := map[string]any{"operationId": r.id, "description": "Current owner bot.manage and credential ceiling, rechecked after exact-scope locks, including receipt replay. Actor is server-derived. Bot has one immutable exact workspace/project home and no implicit role or key. 32 random secret bytes, SHA-256 only at rest. Lost disclosure requires a new rotation operationKey. expectedVersion fences distinct concurrent changes.", "parameters": params, "security": []any{map[string]any{"userBearer": []string{}}, map[string]any{"sessionCookie": []string{}}}, "requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(r.in)}}}, "responses": responses}
		paths[r.path] = map[string]any{r.method: op}
	}
	for _, name := range []string{"BotKeyRequest", "RotateBotKeyRequest"} {
		props := b.schemas[name].(map[string]any)["properties"].(map[string]any)
		props["operations"].(map[string]any)["minItems"] = 1
		props["operations"].(map[string]any)["maxItems"] = 17
		props["operations"].(map[string]any)["uniqueItems"] = true
		props["operations"].(map[string]any)["items"].(map[string]any)["enum"] = []string{"project.read", "comment.create", "project.write", "generation.cancel", "project.publish", "role.manage", "bot.manage", "grant.manage", "audit.read", "generation.create", "generation.execute", "generation.publish", "generation.read", "source.read", "resource.read", "artifact.read", "download"}
	}
	for _, name := range []string{"Bot", "CreateBotRequest"} {
		b.schemas[name].(map[string]any)["properties"].(map[string]any)["name"].(map[string]any)["maxLength"] = 128
	}
	props := b.schemas["BotKeyResult"].(map[string]any)["properties"].(map[string]any)
	props["disclosure"].(map[string]any)["enum"] = []string{"first", "unavailable"}
	props["secret"].(map[string]any)["pattern"] = "^lr_bot_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$"
	props["secret"].(map[string]any)["minLength"] = 50
	props["secret"].(map[string]any)["maxLength"] = 50
	b.schemas["BotKeyResult"].(map[string]any)["oneOf"] = []any{
		map[string]any{"properties": map[string]any{"disclosure": map[string]any{"const": "first"}}, "required": []string{"secret"}},
		map[string]any{"properties": map[string]any{"disclosure": map[string]any{"const": "unavailable"}}, "not": map[string]any{"required": []string{"secret"}}},
	}
	b.schemas["BotKey"].(map[string]any)["properties"].(map[string]any)["status"].(map[string]any)["enum"] = []string{"active", "revoked"}
}
