package accessv1

import "testing"

func TestOAuthOpenAPIBrowserFormsAndCookieOnlySecurity(t *testing.T) {
	spec := NewOpenAPI()
	paths := spec["paths"].(map[string]any)
	for _, path := range []string{"/oauth/login", "/oauth/consent", "/oauth/resume", "/oauth/error"} {
		route, ok := paths[path].(map[string]any)
		if !ok {
			t.Fatalf("browser OAuth route missing: %s", path)
		}
		for method, raw := range route {
			op := raw.(map[string]any)
			security := op["security"].([]any)
			if path != "/oauth/error" {
				if len(security) != 1 {
					t.Fatalf("cookie-only security absent: %s", path)
				}
				if _, ok := security[0].(map[string]any)["sessionCookie"]; !ok {
					t.Fatalf("cookie security missing: %s", path)
				}
			}
			responses := op["responses"].(map[string]any)
			code := "200"
			if method == "post" {
				code = "303"
			} else if path == "/oauth/resume" {
				code = "303"
			} else if path == "/oauth/error" {
				code = "400"
			}
			if _, ok := responses[code]; !ok {
				t.Fatalf("response %s absent: %s %s", code, method, path)
			}
			if method == "post" {
				content := op["requestBody"].(map[string]any)["content"].(map[string]any)
				if len(content) != 1 || content["application/x-www-form-urlencoded"] == nil {
					t.Fatal("OAuth POST must be form-only")
				}
			}
		}
	}
	for path, key := range map[string]string{"/oauth/login": "login_challenge", "/oauth/consent": "consent_challenge"} {
		params := paths[path].(map[string]any)["get"].(map[string]any)["parameters"].([]any)
		if len(params) != 1 || params[0].(map[string]any)["name"] != key {
			t.Fatal("purpose-specific exact query missing")
		}
	}
	if _, ok := paths["/oauth/resume"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"]; ok {
		t.Fatal("resume must only navigate")
	}
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	if len(schemas["OAuthConsent"].(map[string]any)["properties"].(map[string]any)) != 4 {
		t.Fatal("unsafe consent metadata")
	}
	if len(schemas["OAuthLoginForm"].(map[string]any)["properties"].(map[string]any)) != 3 {
		t.Fatal("login scope or override exposed")
	}
}
