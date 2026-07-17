package romm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitDeviceAuth(t *testing.T) {
	var got DeviceAuthInitRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/auth/device/init" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":                "dc-123",
			"user_code":                  "ABCD-1234",
			"verification_path":          "/pair/device",
			"verification_path_complete": "/pair/device?user_code=ABCD-1234",
			"expires_in":                 600,
			"interval":                   5,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	resp, err := client.InitDeviceAuth(DeviceAuthInitRequest{
		ClientDeviceIdentifier: "cid-1",
		Name:                   "My Device",
		Client:                 "grout",
		RequestedScopes:        DeviceAuthScopes,
	})
	if err != nil {
		t.Fatalf("InitDeviceAuth: %v", err)
	}
	if resp.DeviceCode != "dc-123" || resp.UserCode != "ABCD-1234" ||
		resp.VerificationPath != "/pair/device" ||
		resp.VerificationPathComplete != "/pair/device?user_code=ABCD-1234" ||
		resp.ExpiresIn != 600 || resp.Interval != 5 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if got.ClientDeviceIdentifier != "cid-1" || got.Name != "My Device" ||
		got.Client != "grout" || len(got.RequestedScopes) != 9 {
		t.Errorf("unexpected request payload: %+v", got)
	}
}

func TestPollDeviceToken_FlowStates(t *testing.T) {
	tests := []struct {
		detail string
		want   DeviceAuthPollState
	}{
		{"authorization_pending", DeviceAuthPending},
		{"slow_down", DeviceAuthSlowDown},
		{"access_denied", DeviceAuthDenied},
		{"expired_token", DeviceAuthExpired},
	}
	for _, tt := range tests {
		t.Run(tt.detail, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/auth/device/token" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"detail":%q}`, tt.detail)
			}))
			defer srv.Close()

			token, state, err := NewClient(srv.URL).PollDeviceToken("dc-123")
			if err != nil {
				t.Fatalf("PollDeviceToken: %v", err)
			}
			if token != nil {
				t.Errorf("expected nil token, got %+v", token)
			}
			if state != tt.want {
				t.Errorf("state = %v, want %v", state, tt.want)
			}
		})
	}
}

func TestPollDeviceToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeviceCode string `json:"device_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeviceCode != "dc-123" {
			t.Errorf("unexpected poll body: %+v (err %v)", body, err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-abc",
			"device_id":    "dev-1",
			"scopes":       []string{"roms.read"},
			"expires_at":   "2027-07-16T00:00:00Z",
		})
	}))
	defer srv.Close()

	token, state, err := NewClient(srv.URL).PollDeviceToken("dc-123")
	if err != nil {
		t.Fatalf("PollDeviceToken: %v", err)
	}
	if state != DeviceAuthSuccess {
		t.Fatalf("state = %v, want DeviceAuthSuccess", state)
	}
	if token.AccessToken != "tok-abc" || token.DeviceID != "dev-1" ||
		token.ExpiresAt != "2027-07-16T00:00:00Z" || len(token.Scopes) != 1 {
		t.Errorf("unexpected token: %+v", token)
	}
}

func TestPollDeviceToken_NullExpiresAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"tok","device_id":"dev","scopes":[],"expires_at":null}`)
	}))
	defer srv.Close()

	token, state, err := NewClient(srv.URL).PollDeviceToken("dc-123")
	if err != nil || state != DeviceAuthSuccess {
		t.Fatalf("PollDeviceToken: state=%v err=%v", state, err)
	}
	if token.ExpiresAt != "" {
		t.Errorf("ExpiresAt = %q, want empty for JSON null", token.ExpiresAt)
	}
}

func TestPollDeviceToken_UnexpectedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"detail":"boom"}`)
	}))
	defer srv.Close()

	_, _, err := NewClient(srv.URL).PollDeviceToken("dc-123")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
