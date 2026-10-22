// Package httpguard provides loopback host and browser-origin checks for the
// local application's HTTP handler.
package httpguard

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Protect rejects requests addressed to non-loopback hosts and browser
// requests whose Origin does not match the request's same-origin authority.
// Requests without Origin remain available to native HTTP clients. Opaque
// origins are accepted only for read-only project-preview resource requests;
// API and WebSocket endpoints never receive that exception.
func Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme := requestScheme(r)
		hostAuthority, hostOK := canonicalAuthority(r.Host, scheme)
		if !hostOK {
			reject(w)
			return
		}

		origins := r.Header.Values("Origin")
		if len(origins) > 1 {
			reject(w)
			return
		}
		opaquePreview := false
		if len(origins) == 1 {
			origin := origins[0]
			if origin == "null" {
				opaquePreview = isPreviewAssetRequest(r)
				if !opaquePreview {
					reject(w)
					return
				}
			} else if !sameOrigin(origin, scheme, hostAuthority) {
				reject(w)
				return
			}
		}

		if hasCrossSiteFetchMetadata(r) && !opaquePreview && !(len(origins) == 0 && isPreviewSubresourceRequest(r)) {
			reject(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func reject(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func isPreviewAssetRequest(r *http.Request) bool {
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || !strings.HasPrefix(r.URL.Path, "/preview/") {
		return false
	}
	switch strings.ToLower(path.Ext(r.URL.Path)) {
	case ".css", ".js", ".mjs", ".cjs", ".json", ".svg",
		".png", ".jpg", ".jpeg", ".webp", ".gif", ".avif", ".ico",
		".woff", ".woff2", ".ttf", ".otf", ".eot",
		".mp4", ".webm", ".mov", ".mp3", ".wav", ".ogg", ".wasm":
		return true
	default:
		return false
	}
}

func isPreviewSubresourceRequest(r *http.Request) bool {
	if !isPreviewAssetRequest(r) {
		return false
	}
	for _, value := range r.Header.Values("Sec-Fetch-Dest") {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "script", "style", "font", "image", "audio", "video", "track":
			return true
		}
	}
	return false
}

func hasCrossSiteFetchMetadata(r *http.Request) bool {
	for _, value := range r.Header.Values("Sec-Fetch-Site") {
		for _, site := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(site), "cross-site") {
				return true
			}
		}
	}
	return false
}

func sameOrigin(value, scheme, hostAuthority string) bool {
	if strings.TrimSpace(value) != value {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return false
	}
	if !strings.EqualFold(u.Scheme, scheme) {
		return false
	}
	originAuthority, ok := canonicalAuthority(u.Host, scheme)
	return ok && originAuthority == hostAuthority
}

func canonicalAuthority(authority, scheme string) (string, bool) {
	host, port, ok := splitAuthority(authority)
	if !ok {
		return "", false
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()
		if !addr.IsLoopback() {
			return "", false
		}
		host = addr.String()
	} else {
		host = strings.TrimSuffix(strings.ToLower(host), ".")
		if host != "localhost" {
			return "", false
		}
	}

	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	} else {
		n, err := strconv.Atoi(port)
		if err != nil || n < 0 || n > 65535 {
			return "", false
		}
		port = strconv.Itoa(n)
	}
	return net.JoinHostPort(host, port), true
}

func splitAuthority(authority string) (host, port string, ok bool) {
	if authority == "" || strings.TrimSpace(authority) != authority || strings.ContainsAny(authority, "/\\?#@") {
		return "", "", false
	}
	if h, p, err := net.SplitHostPort(authority); err == nil {
		if h == "" || p == "" {
			return "", "", false
		}
		return h, p, true
	}
	if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
		return authority[1 : len(authority)-1], "", true
	}
	if !strings.Contains(authority, ":") {
		return authority, "", true
	}
	if addr, err := netip.ParseAddr(authority); err == nil {
		return addr.String(), "", true
	}
	return "", "", false
}
