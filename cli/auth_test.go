package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rommapp/grout/api"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

func pairServer(t *testing.T, replies ...fakeromm.TokenReply) (*fakeromm.Server, string) {
	t.Helper()
	srv := fakeromm.New(t)
	srv.SetDeviceInit(romm.DeviceInit{
		DeviceCode:               "device-code",
		UserCode:                 "WXYZ-2345",
		VerificationPath:         "/pair",
		VerificationPathComplete: "/pair?code=WXYZ-2345",
		ExpiresIn:                900,
		Interval:                 1,
	})
	srv.SetTokenReplies(replies...)
	srv.AddExchangeCode("ABCD2345", romm.ClientToken{
		ID:        4,
		Name:      "living room",
		Scopes:    romm.Scopes,
		ExpiresAt: "2027-02-01T00:00:00Z",
		DeviceID:  "dev-3",
		RawToken:  "raw-1",
	})
	dir := t.TempDir()
	code, e, _ := runCLI(t, dir, "call", "settings.set_host", fmt.Sprintf(`{"root_uri":%q}`, srv.URL()))
	if code != ExitOK {
		t.Fatalf("set_host code=%d env=%+v", code, e)
	}
	return srv, dir
}

func approved() fakeromm.TokenReply {
	return fakeromm.TokenReply{Token: romm.DeviceToken{
		AccessToken: "tok-1",
		DeviceID:    "dev-9",
		Scopes:      romm.Scopes,
		ExpiresAt:   "2027-01-01T00:00:00Z",
	}}
}

func runPair(t *testing.T, dir string, args ...string) (ExitCode, []envelope, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(context.Background(), Env{
		Dir:      dir,
		Stdout:   &out,
		Stderr:   &errb,
		Firmware: "testfw",
		Environ:  map[string]string{"GROUT_ROOT": t.TempDir()},
		Client:   api.ClientSpec{Name: "grout", Version: "9.9.9"},
	}, args)
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	var envs []envelope
	for dec.More() {
		var e envelope
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("stdout not JSON: %q stderr=%q", out.String(), errb.String())
		}
		envs = append(envs, e)
	}
	return code, envs, out.String()
}

func field(t *testing.T, raw json.RawMessage, key string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return string(m[key])
}

func TestPairPrintsStartThenResult(t *testing.T) {
	srv, dir := pairServer(t, approved())
	code, envs, raw := runPair(t, dir, "pair", "--name", "slot")
	if code != ExitOK || len(envs) != 2 {
		t.Fatalf("code=%d out=%s", code, raw)
	}
	if field(t, envs[0].Result, "user_code") != `"WXYZ-2345"` {
		t.Fatalf("start = %s", envs[0].Result)
	}
	if field(t, envs[1].Result, "outcome") != `"approved"` {
		t.Fatalf("wait = %s", envs[1].Result)
	}
	reqs := srv.InitRequests()
	if len(reqs) != 1 || reqs[0].Client != "grout" || reqs[0].ClientVersion != "9.9.9" {
		t.Fatalf("init requests = %+v", reqs)
	}
	t.Log(raw)
}

func TestPairDeniedExitsSix(t *testing.T) {
	_, dir := pairServer(t, fakeromm.TokenReply{State: romm.PairDenied})
	code, envs, raw := runPair(t, dir, "pair")
	if code != ExitUnauthorized || len(envs) != 2 {
		t.Fatalf("code=%d out=%s", code, raw)
	}
	if field(t, envs[1].Result, "outcome") != `"denied"` {
		t.Fatalf("wait = %s", envs[1].Result)
	}
}

func TestPairStartFailureStopsAfterOneEnvelope(t *testing.T) {
	code, envs, raw := runPair(t, t.TempDir(), "pair")
	if code != ExitNotConfigured || len(envs) != 1 || envs[0].OK {
		t.Fatalf("code=%d out=%s", code, raw)
	}
}

func TestLoginWithCode(t *testing.T) {
	_, dir := pairServer(t)
	code, e, raw := runCLI(t, dir, "login", "--code", "abcd-2345")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d out=%s", code, raw)
	}
	if field(t, e.Result, "username") == "" {
		t.Fatalf("result = %s", e.Result)
	}
}

func TestLoginBadCodeExitsSix(t *testing.T) {
	_, dir := pairServer(t)
	code, e, raw := runCLI(t, dir, "login", "--code", "nope-0000")
	if code != ExitUnauthorized || e.Error == nil || e.Error.Code != api.CodeInvalidCode {
		t.Fatalf("code=%d out=%s", code, raw)
	}
}

func TestLoginWithoutCodeIsUsage(t *testing.T) {
	code := Run(context.Background(), Env{Dir: t.TempDir(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}, []string{"login"})
	if code != ExitUsage {
		t.Fatalf("code=%d", code)
	}
}

func TestLogout(t *testing.T) {
	_, dir := pairServer(t)
	if code, _, raw := runCLI(t, dir, "login", "--code", "abcd-2345"); code != ExitOK {
		t.Fatalf("login code=%d out=%s", code, raw)
	}
	code, e, raw := runCLI(t, dir, "logout")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d out=%s", code, raw)
	}
	var res struct {
		Hosts []struct {
			HasToken bool `json:"has_token"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(e.Result, &res); err != nil || len(res.Hosts) != 1 || res.Hosts[0].HasToken {
		t.Fatalf("result = %s", e.Result)
	}
}
