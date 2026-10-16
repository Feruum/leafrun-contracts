package ownerproofv1

// AnnotateSchemas adds the verifier-specific semantic bounds to reflected DTOs.
func AnnotateSchemas(schemas map[string]any) {
	text := func() map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "x-max-utf8-bytes": 256, "description": "Exact valid UTF-8 text, 1–256 bytes; no outer whitespace, NUL, CR or LF. No trimming, normalization or UUID parsing."}
	}
	scope := map[string]any{"type": "object", "required": []string{"workspaceId", "projectId"}, "additionalProperties": false, "properties": map[string]any{"workspaceId": text(), "projectId": text()}}
	for _, name := range []string{"ScopeRequest", "SnapshotBinding", "RevisionBinding", "ImportRequest"} {
		raw, ok := schemas[name]
		if !ok {
			continue
		}
		schema := raw.(map[string]any)
		properties := schema["properties"].(map[string]any)
		properties["scope"] = scope
		for _, field := range []string{"revision", "snapshotId", "idempotencyKey"} {
			if _, ok = properties[field]; ok {
				properties[field] = text()
			}
		}
		if _, ok = properties["contentId"]; ok {
			properties["contentId"] = map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$", "minLength": 64, "maxLength": 64}
		}
	}
	if raw, ok := schemas["ImportRequest"]; ok {
		schema := raw.(map[string]any)
		properties := schema["properties"].(map[string]any)
		properties["kind"] = map[string]any{"type": "string", "enum": []string{"personal", "team"}}
		schema["oneOf"] = []any{map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "personal"}}, "required": []string{"ownerId"}, "not": map[string]any{"required": []string{"teamId"}}}, map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "team"}}, "required": []string{"teamId"}, "not": map[string]any{"required": []string{"ownerId"}}}}
	}
}
