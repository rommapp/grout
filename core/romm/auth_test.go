package romm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPostJSONSendsBodyAndDecodes(t *testing.T) {
	var ct, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	var out struct {
		OK bool `json:"ok"`
	}
	err := New(Options{BaseURL: srv.URL}).postJSON(context.Background(), "/x", map[string]string{"a": "b"}, &out)
	if err != nil || ct != "application/json" || body != `{"a":"b"}` || !out.OK {
		t.Fatalf("ct=%q body=%q out=%v err=%v", ct, body, out, err)
	}
}

func TestIsNotFoundAndIsRateLimited(t *testing.T) {
	if !IsNotFound(&StatusError{Code: 404}) || IsNotFound(&StatusError{Code: 500}) || IsNotFound(nil) {
		t.Fatal("IsNotFound")
	}
	if !IsRateLimited(&StatusError{Code: 429}) || IsRateLimited(&StatusError{Code: 404}) || IsRateLimited(nil) {
		t.Fatal("IsRateLimited")
	}
}

func TestExchangeCodeSendsNoAuthHeader(t *testing.T) {
	var auth, body, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	_, err := New(Options{BaseURL: srv.URL, AuthHeader: "Bearer x"}).ExchangeCode(context.Background(), "ABCD2345")
	if err != nil || auth != "" || body != `{"code":"ABCD2345"}` || path != "/api/client-tokens/exchange" {
		t.Fatalf("auth=%q body=%q path=%q err=%v", auth, body, path, err)
	}
}

func TestExchangeCodeDecodesToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":4,"name":"n","scopes":["me.read"],"expires_at":"2027-01-01T00:00:00Z","device_id":"d1","raw_token":"rmm_abc"}`))
	}))
	defer srv.Close()
	tok, err := New(Options{BaseURL: srv.URL}).ExchangeCode(context.Background(), "ABCD2345")
	if err != nil || tok.RawToken != "rmm_abc" || tok.DeviceID != "d1" || tok.ExpiresAt != "2027-01-01T00:00:00Z" || tok.ID != 4 {
		t.Fatalf("tok=%+v err=%v", tok, err)
	}
}

func TestDeviceInitiateSendsScopesInOrder(t *testing.T) {
	var got DeviceInitRequest
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"device_code":"dc","user_code":"UC","verification_path":"/pair","verification_path_complete":"/pair?user_code=UC","expires_in":900,"interval":5}`))
	}))
	defer srv.Close()
	init, err := New(Options{BaseURL: srv.URL, AuthHeader: "Bearer x"}).DeviceInitiate(context.Background(), DeviceInitRequest{Client: "grout", RequestedScopes: Scopes})
	if err != nil || !reflect.DeepEqual(got.RequestedScopes, Scopes) || got.Client != "grout" || auth != "" {
		t.Fatalf("got=%+v auth=%q err=%v", got, auth, err)
	}
	if init.DeviceCode != "dc" || init.UserCode != "UC" || init.ExpiresIn != 900 || init.Interval != 5 || init.VerificationPathComplete != "/pair?user_code=UC" {
		t.Fatalf("init=%+v", init)
	}
}

func TestPollDeviceTokenStates(t *testing.T) {
	for _, st := range []PairState{PairPending, PairSlowDown, PairDenied, PairExpired} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			w.Write([]byte(`{"detail":"` + string(st) + `"}`))
		}))
		_, err := New(Options{BaseURL: srv.URL}).PollDeviceToken(context.Background(), "dc")
		srv.Close()
		var pe *PairStateError
		if !errors.As(err, &pe) || pe.State != st {
			t.Fatalf("state %s err=%v", st, err)
		}
	}
}

func TestPollDeviceTokenApproved(t *testing.T) {
	var auth, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`{"access_token":"rmm_t","device_id":"d9","scopes":["me.read"],"expires_at":"2027-02-02T00:00:00Z"}`))
	}))
	defer srv.Close()
	tok, err := New(Options{BaseURL: srv.URL, AuthHeader: "Bearer x"}).PollDeviceToken(context.Background(), "dc")
	if err != nil || tok.AccessToken != "rmm_t" || tok.DeviceID != "d9" || tok.ExpiresAt != "2027-02-02T00:00:00Z" || auth != "" || body != `{"device_code":"dc"}` {
		t.Fatalf("tok=%+v auth=%q body=%q err=%v", tok, auth, body, err)
	}
}

func TestPollDeviceTokenOtherErrorIsStatusError(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
	}{{400, `{"detail":"bogus"}`}, {500, `boom`}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			w.Write([]byte(tc.body))
		}))
		_, err := New(Options{BaseURL: srv.URL}).PollDeviceToken(context.Background(), "dc")
		srv.Close()
		var se *StatusError
		if !errors.As(err, &se) || se.Code != tc.code {
			t.Fatalf("code %d err=%v", tc.code, err)
		}
	}
}

func TestMe(t *testing.T) {
	var path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"username":"alice","id":1}`))
	}))
	defer srv.Close()
	u, err := New(Options{BaseURL: srv.URL, AuthHeader: "Bearer x"}).Me(context.Background())
	if err != nil || u.Username != "alice" || path != "/api/users/me" || auth != "Bearer x" {
		t.Fatalf("u=%+v path=%q auth=%q err=%v", u, path, auth, err)
	}
}
