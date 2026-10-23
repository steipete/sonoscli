package sonos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMAPI_BeginAuthentication_APIClientIdentityError(t *testing.T) {
	for _, tt := range []struct {
		name     string
		status   int
		body     string
		wantHint bool
	}{
		{"unregistered caller", http.StatusForbidden, `{"error":{"code":403,"status":"PERMISSION_DENIED","message":"Method doesn't allow unregistered callers (callers without established identity). Please use API Key or other form of API consumer identity to call this API. private account detail"}}`, true},
		{"other permission error", http.StatusForbidden, `{"error":{"status":"PERMISSION_DENIED","message":"private account detail"}}`, false},
		{"non JSON response", http.StatusForbidden, `<html>private account detail</html>`, false},
		{"other HTTP status", http.StatusUnauthorized, `{"error":{"status":"PERMISSION_DENIED","message":"Method doesn't allow unregistered callers. private account detail"}}`, false},
		{"other API status", http.StatusForbidden, `{"error":{"status":"UNAUTHENTICATED","message":"Method doesn't allow unregistered callers. private account detail"}}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := &SMAPIClient{
				httpClient:  srv.Client(),
				Service:     MusicServiceDescriptor{ID: "284", Auth: MusicServiceAuthAppLink, SecureURI: srv.URL},
				HouseholdID: "test-household", DeviceID: "test-device", TokenStore: newMemTokenStore(),
			}
			_, err := c.BeginAuthentication(context.Background())
			if err == nil || !strings.Contains(err.Error(), http.StatusText(tt.status)) {
				t.Fatalf("expected HTTP status in error, got %v", err)
			}
			if got := strings.Contains(err.Error(), "API client identity"); got != tt.wantHint {
				t.Fatalf("API identity hint = %v, want %v: %v", got, tt.wantHint, err)
			}
			if tt.wantHint && (!strings.Contains(err.Error(), "Sonos app") || !strings.Contains(err.Error(), "sonos play-url") || !strings.Contains(err.Error(), "not account authentication")) {
				t.Fatalf("missing supported playback alternative and authentication distinction: %v", err)
			}
			if strings.Contains(err.Error(), "private account detail") {
				t.Fatalf("error exposed response body: %v", err)
			}
		})
	}
}
