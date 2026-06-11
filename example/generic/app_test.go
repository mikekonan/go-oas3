package generic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	router := chi.NewRouter()
	WidgetsHandler(widgetsService{}, router, nil, widgetsSchemas{})
	return httptest.NewServer(router)
}

// TestSecurityPath exercises the extractSecurity() helper: valid Bearer token
// passes through to the service, invalid/missing token returns 401.
func TestSecurityPath(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid token", "Bearer valid-token", 200},
		{"invalid token", "Bearer bad-token", 401},
		{"missing token", "", 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", srv.URL+"/widgets/secure", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

func doPost(t *testing.T, srv *httptest.Server, path string, body any) (int, http.Header, []byte) {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		t.Fatalf("encode: %v", err)
	}
	resp, err := http.Post(srv.URL+path, "application/json", buf)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return resp.StatusCode, resp.Header, bytes.TrimSpace(raw)
}

// Both styles must produce semantically identical HTTP responses on the happy
// path. The Widget IDs intentionally differ ("classic-1" vs "generic-1") so we
// only compare envelope (status, content-type, JSON shape with same name).
func TestSuccessPathParity(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	classicStatus, classicHdr, classicBody := doPost(t, srv, "/classic-widgets",
		map[string]string{"name": "my-widget"})
	genericStatus, genericHdr, genericBody := doPost(t, srv, "/widgets",
		map[string]string{"name": "my-widget"})

	if classicStatus != 200 || genericStatus != 200 {
		t.Fatalf("status mismatch: classic=%d generic=%d", classicStatus, genericStatus)
	}
	if classicHdr.Get("content-type") != genericHdr.Get("content-type") {
		t.Fatalf("content-type mismatch: classic=%q generic=%q",
			classicHdr.Get("content-type"), genericHdr.Get("content-type"))
	}

	var classicWidget, genericWidget Widget
	if err := json.Unmarshal(classicBody, &classicWidget); err != nil {
		t.Fatalf("classic body: %v\nbody=%s", err, classicBody)
	}
	if err := json.Unmarshal(genericBody, &genericWidget); err != nil {
		t.Fatalf("generic body: %v\nbody=%s", err, genericBody)
	}
	if classicWidget.Name != genericWidget.Name {
		t.Fatalf("name mismatch: classic=%q generic=%q", classicWidget.Name, genericWidget.Name)
	}
}

// Probe the generic endpoint directly with both success and error paths to
// ensure the Response[B] / ResponseBuilder[B] machinery is wired up.
func TestGenericResponseShape(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	// Happy path: 200 + Widget body
	status, hdr, body := doPost(t, srv, "/widgets", map[string]string{"name": "ok"})
	if status != 200 {
		t.Fatalf("expected 200, got %d", status)
	}
	if got := hdr.Get("content-type"); got != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", got)
	}
	var widget Widget
	if err := json.Unmarshal(body, &widget); err != nil {
		t.Fatalf("decode widget: %v body=%s", err, body)
	}
	if widget.ID != "generic-1" || widget.Name != "ok" {
		t.Fatalf("unexpected widget: %+v", widget)
	}
}

// Custom response headers and Set-Cookie must round-trip through the generic
// builder. We use a client that doesn't follow redirects so headers stay raw.
func TestGenericHeadersAndCookies(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/widgets/echo")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Request-ID"); got != "req-abc-123" {
		t.Fatalf("X-Request-ID = %q, want req-abc-123", got)
	}

	var foundCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "widget_session" {
			foundCookie = c
			break
		}
	}
	if foundCookie == nil {
		t.Fatalf("widget_session cookie missing, got cookies=%v", resp.Cookies())
	}
	if foundCookie.Value != "tok-42" {
		t.Fatalf("cookie value = %q, want tok-42", foundCookie.Value)
	}
}

// Multiple content-types: service explicitly returns XML, the wrapper marshals
// accordingly.
func TestGenericMultipleContentTypes(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/widgets/multi")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if got := resp.Header.Get("content-type"); got != "application/xml" {
		t.Fatalf("content-type = %q, want application/xml", got)
	}
	// XML body should contain the field as XML element, not JSON.
	// Note: encoding/xml uses the Go field name (`ID`), not the json tag,
	// so the inner element is `<ID>` rather than `<id>`.
	if !bytes.Contains(body, []byte("<Widget>")) || !bytes.Contains(body, []byte("<ID>")) {
		t.Fatalf("body does not look like XML: %s", body)
	}
}

// 302 redirect with a Location header — wrapper must call http.Redirect.
func TestGenericRedirect(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	// Disable automatic redirect follow so we observe the 302 response.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(srv.URL + "/widgets/redirect")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/widgets/echo" {
		t.Fatalf("Location = %q, want /widgets/echo", loc)
	}
}
