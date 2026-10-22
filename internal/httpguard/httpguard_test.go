package httpguard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectAllowsLoopbackSameOriginAndNativeRequests(t *testing.T) {
	cases := []struct {
		name   string
		method string
		host   string
		path   string
		origin string
	}{
		{name: "localhost Vite proxy", method: "GET", host: "localhost:5174", path: "/api/health", origin: "http://localhost:5174"},
		{name: "loopback IPv4", method: "GET", host: "127.0.0.1:43821", path: "/api/health", origin: "http://127.0.0.1:43821"},
		{name: "loopback IPv6", method: "GET", host: "[::1]:43821", path: "/api/health", origin: "http://[::1]:43821"},
		{name: "native client without Origin", method: "GET", host: "localhost:43821", path: "/api/health"},
		{name: "native MCP mutation without Origin", method: "POST", host: "localhost:43821", path: "/api/projects"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, called := runGuardRequest(tc.method, tc.host, tc.path, tc.origin, "")
			if !called || w.Code != http.StatusNoContent {
				t.Fatalf("request rejected: called=%v status=%d body=%q", called, w.Code, w.Body.String())
			}
		})
	}
}

func TestProtectRejectsForeignOriginsAndRebindingHosts(t *testing.T) {
	cases := []struct {
		name   string
		method string
		host   string
		path   string
		origin string
		fetch  string
	}{
		{name: "foreign DNS host", method: "GET", host: "attacker.example", path: "/api/health"},
		{name: "localhost suffix rebinding", method: "GET", host: "localhost.attacker.example", path: "/api/health"},
		{name: "loopback suffix rebinding", method: "GET", host: "127.0.0.1.attacker.example", path: "/api/health"},
		{name: "foreign Origin on mutation", method: "PUT", host: "localhost:43821", path: "/api/preferences", origin: "https://attacker.example"},
		{name: "different loopback authority", method: "GET", host: "localhost:43821", path: "/api/health", origin: "http://127.0.0.1:43821"},
		{name: "opaque API Origin", method: "GET", host: "localhost:43821", path: "/api/projects", origin: "null"},
		{name: "cross-site metadata", method: "GET", host: "localhost:43821", path: "/api/health", origin: "http://localhost:43821", fetch: "cross-site"},
		{name: "bad Origin path", method: "GET", host: "localhost:43821", path: "/api/health", origin: "http://localhost:43821/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, called := runGuardRequest(tc.method, tc.host, tc.path, tc.origin, tc.fetch)
			if called || w.Code != http.StatusForbidden {
				t.Fatalf("request was not rejected: called=%v status=%d body=%q", called, w.Code, w.Body.String())
			}
		})
	}
}

func TestProtectAllowsOpaqueOriginOnlyForPreviewAssetReads(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			w, called := runGuardRequest(method, "localhost:43821", "/preview/demo/assets/app.mjs", "null", "cross-site")
			if !called || w.Code != http.StatusNoContent {
				t.Fatalf("opaque preview resource rejected: called=%v status=%d", called, w.Code)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("guard unexpectedly opened CORS with %q", got)
			}
		})
	}

	for _, tc := range []struct {
		name   string
		method string
		path   string
		origin string
	}{
		{name: "opaque API read", method: http.MethodGet, path: "/api/projects/demo/files", origin: "null"},
		{name: "opaque websocket", method: http.MethodGet, path: "/api/projects/demo/watch", origin: "null"},
		{name: "opaque preview document", method: http.MethodGet, path: "/preview/demo/index.html", origin: "null"},
		{name: "opaque preview mutation", method: http.MethodPost, path: "/preview/demo/file.js", origin: "null"},
		{name: "foreign preview Origin", method: http.MethodGet, path: "/preview/demo/file.js", origin: "http://attacker.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, called := runGuardRequest(tc.method, "localhost:43821", tc.path, tc.origin, "cross-site")
			if called || w.Code != http.StatusForbidden {
				t.Fatalf("request was not rejected: called=%v status=%d", called, w.Code)
			}
		})
	}
}

func TestProtectAllowsCrossSiteMetadataOnlyForPreviewSubresources(t *testing.T) {
	w, called := runGuardRequestWithDest(http.MethodGet, "localhost:43821", "/preview/demo/assets/site.css", "", "cross-site", "style")
	if !called || w.Code != http.StatusNoContent {
		t.Fatalf("embedded preview stylesheet rejected: called=%v status=%d", called, w.Code)
	}

	w, called = runGuardRequestWithDest(http.MethodGet, "localhost:43821", "/preview/demo/assets/app.js", "", "cross-site", "script")
	if !called || w.Code != http.StatusNoContent {
		t.Fatalf("embedded preview script rejected: called=%v status=%d", called, w.Code)
	}

	for _, dest := range []string{"", "document"} {
		w, called = runGuardRequestWithDest(http.MethodGet, "localhost:43821", "/preview/demo/index.html", "", "cross-site", dest)
		if called || w.Code != http.StatusForbidden {
			t.Fatalf("cross-site preview document accepted for destination %q: called=%v status=%d", dest, called, w.Code)
		}
	}
}

func runGuardRequest(method, host, path, origin, fetchSite string) (*httptest.ResponseRecorder, bool) {
	return runGuardRequestWithDest(method, host, path, origin, fetchSite, "")
}

func runGuardRequestWithDest(method, host, path, origin, fetchSite, fetchDest string) (*httptest.ResponseRecorder, bool) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(method, "http://"+host+path, nil)
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if fetchSite != "" {
		req.Header.Set("Sec-Fetch-Site", fetchSite)
	}
	if fetchDest != "" {
		req.Header.Set("Sec-Fetch-Dest", fetchDest)
	}
	w := httptest.NewRecorder()
	Protect(next).ServeHTTP(w, req)
	return w, called
}
