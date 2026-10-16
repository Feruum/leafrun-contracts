package contracts_test

import (
	"context"
	generation "github.com/leafrun/leafrun-contracts/generation/v1"
	projects "github.com/leafrun/leafrun-contracts/projects/v1"
	storage "github.com/leafrun/leafrun-contracts/storage/v1"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExplicitOwnerBearerClientsRejectAmbientCookieJar(t *testing.T) {
	for _, kind := range []string{"generation", "projects", "storage"} {
		t.Run(kind, func(t *testing.T) {
			cookieSeen := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cookieSeen = r.Header.Get("Cookie") != ""
				if r.Header.Get("Authorization") != "Bearer explicit-token" {
					t.Error("explicit credential missing")
				}
				w.Header().Set("Content-Type", "application/json")
				switch kind {
				case "generation":
					w.Write([]byte(`{"jobId":"job"}`))
				case "projects":
					w.Write([]byte(`{"releaseId":"release"}`))
				case "storage":
					w.Write([]byte(`{"snapshotId":"snapshot"}`))
				}
			}))
			defer server.Close()
			jar, _ := cookiejar.New(nil)
			target, _ := url.Parse(server.URL)
			jar.SetCookies(target, []*http.Cookie{{Name: "ory_kratos_session", Value: "ambient-human-cookie", Path: "/"}})
			transport := &http.Client{Jar: jar}
			ctx := context.Background()
			switch kind {
			case "generation":
				c, e := generation.NewClient(server.URL, "explicit-token", transport)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = c.GetJob(ctx, "job"); e != nil {
					t.Fatal(e)
				}
			case "projects":
				c, e := projects.NewClient(server.URL, "explicit-token", transport)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = c.GetRelease(ctx, "release"); e != nil {
					t.Fatal(e)
				}
			case "storage":
				c, e := storage.NewClient(server.URL, "explicit-token", transport)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = c.GetSnapshot(ctx, "snapshot"); e != nil {
					t.Fatal(e)
				}
			}
			if cookieSeen {
				t.Fatal("ambient cookie crossed explicit owner bearer boundary")
			}
		})
	}
}
