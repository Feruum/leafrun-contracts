package accessv1

import "reflect"

func oauthOpenAPI(b *schemaBuilder, paths map[string]any) {
	stringSchema := func() map[string]any { return map[string]any{"type": "string"} }
	scopes := func() map[string]any {
		return map[string]any{"type": "array", "maxItems": OAuthScopeLimit, "uniqueItems": true, "items": map[string]any{"type": "string", "enum": append([]string(nil), OAuthScopes...)}}
	}
	b.schema(reflect.TypeFor[OAuthConsent]())
	metadata := b.schemas["OAuthConsent"].(map[string]any)
	metadata["description"] = "Safe consent metadata only: no credential, provider challenge, code, verifier, actor selector or authority selector. This schema does not define a JSON consent endpoint."
	metadata["properties"].(map[string]any)["scopes"] = scopes()
	for _, purpose := range []string{"login", "consent"} {
		state := stringSchema()
		state["format"], state["pattern"] = "uuid", oauthStatePattern.String()
		state["minLength"], state["maxLength"] = 36, 36
		csrf := map[string]any{"type": "string", "minLength": 43, "maxLength": 43, "pattern": "^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$", "writeOnly": true}
		actions := []string{"accept"}
		if purpose == "consent" {
			actions = append(actions, "deny")
		}
		properties := map[string]any{"state": state, "csrf": csrf, "action": map[string]any{"type": "string", "enum": actions}}
		required := []string{"state", "csrf", "action"}
		name := "OAuthLoginForm"
		if purpose == "consent" {
			name = "OAuthConsentForm"
			properties["scope"] = scopes()
		}
		form := map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
		if purpose == "consent" {
			form["allOf"] = []any{map[string]any{"if": map[string]any{"properties": map[string]any{"action": map[string]any{"const": "accept"}}}, "then": map[string]any{"required": []string{"scope"}, "properties": map[string]any{"scope": map[string]any{"minItems": 1}}}}}
		}
		b.schemas[name] = form
		get := oauthHTMLOperation("oauth"+purpose+"Page", false)
		get["description"] = "Exactly one purpose-specific provider challenge query parameter; unknown or repeated query keys deny. Live Kratos session and owned browser binding are validated; login may redirect to the fixed Kratos login/resume flow. Escaped HTML contains safe client/scopes and owned one-use form state only."
		get["parameters"] = []any{map[string]any{"name": purpose + "_challenge", "in": "query", "required": true, "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": OAuthChallengeLimit, "pattern": `^[\x21-\x2b\x2d-\x3a\x3c-\x7e]+$`, "not": map[string]any{"pattern": `[^\x21-\x2b\x2d-\x3a\x3c-\x7e]`}, "writeOnly": true}}}
		get["x-reject-unknown-query"] = true
		post := oauthHTMLOperation("oauth"+purpose+"Submit", true)
		post["description"] = "Empty query; form only with exactly one state, csrf and action. Cookie-only live Kratos human/session plus owned browser binding and mandatory same-origin POST. State and CSRF are finite one-use bindings, never authority. No caller actor/client/authority/redirect selection. Consent selected scope must be unique, known and a subset of the exact trusted request; accept requires a nonempty selection, deny validates any selection then discards it. Final live checks precede provider acceptance; continuation is constrained to configured Hydra origin and authorize path."
		post["x-reject-unknown-query"] = true
		post["x-empty-query"] = true
		post["x-max-request-bytes"] = 8192
		encoding := map[string]any{}
		if purpose == "consent" {
			encoding["scope"] = map[string]any{"style": "form", "explode": true}
		}
		post["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/x-www-form-urlencoded": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + name}, "encoding": encoding}}}
		paths["/oauth/"+purpose] = map[string]any{"get": get, "post": post}
	}
	resume := oauthHTMLOperation("oauthResume", false)
	delete(resume["responses"].(map[string]any), "200")
	resume["description"] = "Empty query. Resume only through the HttpOnly owned resume UUID cookie and its browser-binding cookie, then revalidate the live Kratos session. No return or state URL override."
	resume["x-empty-query"], resume["x-reject-unknown-query"] = true, true
	paths["/oauth/resume"] = map[string]any{"get": resume}
	paths["/oauth/error"] = map[string]any{"get": map[string]any{"operationId": "oauthSafeError", "security": []any{}, "description": "Fixed safe error HTML; provider query values are never reflected, followed, or used as authority.", "responses": map[string]any{"400": oauthHTMLResponse("Fixed safe OAuth error")}}}
}

func oauthHTMLResponse(description string) map[string]any {
	return map[string]any{"description": description, "content": map[string]any{"text/html": map[string]any{"schema": map[string]any{"type": "string"}}}}
}
func oauthHTMLOperation(id string, post bool) map[string]any {
	responses := map[string]any{"303": map[string]any{"description": "Browser navigation to the fixed login/resume route or validated configured Hydra authorize continuation. No raw continuation is returned in an error body."}}
	if !post {
		responses["200"] = oauthHTMLResponse("Escaped browser page")
	}
	for code, description := range map[string]string{"400": "Malformed bounded browser request", "401": "Missing or invalid browser binding, CSRF or live Kratos session", "403": "Browser origin denied", "503": "Provider or owned state dependency unavailable"} {
		responses[code] = oauthHTMLResponse(description)
	}
	return map[string]any{"operationId": id, "security": []any{map[string]any{"sessionCookie": []string{}}}, "responses": responses}
}
