package api

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rommapp/grout/version"
)

const missingCardDefinition = `
id = "hostfw"
name = "Host firmware"
maintained = "community"

[paths]
roms = { value = "/roms" }
bios = { value = "/bios" }
saves = { value = "/saves" }
`

func testOpenOptions(t *testing.T, dataDir string) OpenOptions {
	t.Helper()
	return OpenOptions{
		DataDir:  dataDir,
		Firmware: FirmwareSpec{ID: "testfw"},
		Env:      map[string]string{"GROUT_ROOT": t.TempDir()},
	}
}

func openServer(t *testing.T, dataDir string) *Server {
	t.Helper()
	s, err := Open(context.Background(), testOpenOptions(t, dataDir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

type reply struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *ErrorBody      `json:"error"`
}

func callRaw(t *testing.T, s *Server, method string, args []byte, emit func(Event)) []byte {
	t.Helper()
	return s.Call(context.Background(), method, args, emit)
}

func decode(t *testing.T, raw []byte) reply {
	t.Helper()
	var r reply
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return r
}

func call(t *testing.T, s *Server, method, args string) reply {
	t.Helper()
	var b []byte
	if args != "" {
		b = []byte(args)
	}
	return decode(t, callRaw(t, s, method, b, nil))
}

func mustOK(t *testing.T, s *Server, method, args string) json.RawMessage {
	t.Helper()
	r := call(t, s, method, args)
	if !r.OK {
		t.Fatalf("%s %s: %+v", method, args, r.Error)
	}
	return r.Result
}

func wantError(t *testing.T, r reply, code Code) {
	t.Helper()
	if r.OK || r.Error == nil || r.Error.Code != code {
		t.Fatalf("reply = ok %v error %+v, want %q", r.OK, r.Error, code)
	}
}

func TestVersion(t *testing.T) {
	v := Version()
	if v.Protocol != 1 {
		t.Fatalf("protocol = %d", v.Protocol)
	}
	if v.Release != version.Version {
		t.Fatalf("release = %q, want %q", v.Release, version.Version)
	}
}

func TestCodesIncludesNewCodesLast(t *testing.T) {
	got := Codes()
	if len(got) != 15 || got[11] != "invalid_code" || got[12] != "rate_limited" || got[13] != "bad_archive" || got[14] != "missing_on_server" {
		t.Fatalf("codes = %v", got)
	}
}

func TestPairOutcomesListed(t *testing.T) {
	want := []PairOutcome{"approved", "denied", "expired", "failed"}
	if got := PairOutcomes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes = %v", got)
	}
}

func TestCodesListsEveryCode(t *testing.T) {
	want := []Code{
		"not_configured", "bad_definition", "server_unreachable", "unauthorized", "server_too_old",
		"busy", "data_dir_locked", "bad_request", "cancelled", "storage", "internal",
	}
	want = append(want, "invalid_code", "rate_limited", "bad_archive", "missing_on_server")
	if got := Codes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v", got)
	}
}

func TestEnvelopeShapes(t *testing.T) {
	s := openServer(t, t.TempDir())

	ok := callRaw(t, s, "library.dims", nil, nil)
	if !strings.HasPrefix(string(ok), `{"ok":true,"result":`) {
		t.Fatalf("success = %s", ok)
	}
	var okKeys map[string]json.RawMessage
	if err := json.Unmarshal(ok, &okKeys); err != nil {
		t.Fatal(err)
	}
	if _, has := okKeys["error"]; has {
		t.Fatalf("success has error: %s", ok)
	}

	bad := callRaw(t, s, "status", nil, nil)
	if !strings.HasPrefix(string(bad), `{"ok":false,"error":{"code":"not_configured","message":"`) {
		t.Fatalf("failure = %s", bad)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(bad, &keys); err != nil {
		t.Fatal(err)
	}
	if _, has := keys["result"]; has {
		t.Fatalf("failure has result: %s", bad)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(keys["error"], &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 2 {
		t.Fatalf("error body = %s", keys["error"])
	}
}

func TestUnknownMethodIsBadRequest(t *testing.T) {
	s := openServer(t, t.TempDir())
	wantError(t, call(t, s, "library.nope", ""), CodeBadRequest)
}

func TestUnknownArgumentFieldIsBadRequest(t *testing.T) {
	s := openServer(t, t.TempDir())
	wantError(t, call(t, s, "library.platforms", `{"bogus":1}`), CodeBadRequest)
	wantError(t, call(t, s, "library.dims", `{"bogus":1}`), CodeBadRequest)
	wantError(t, call(t, s, "library.platforms", `{"all":true} {}`), CodeBadRequest)
	wantError(t, call(t, s, "library.platforms", `[1]`), CodeBadRequest)
}

func TestNullAndEmptyArgsMeanDefaults(t *testing.T) {
	s := openServer(t, t.TempDir())
	for _, args := range [][]byte{nil, []byte(""), []byte("null")} {
		r := decode(t, callRaw(t, s, "library.platforms", args, nil))
		if !r.OK {
			t.Fatalf("args %q: %+v", args, r.Error)
		}
		if string(r.Result) != "[]" {
			t.Fatalf("args %q: result = %s", args, r.Result)
		}
	}
}

func TestPanicIsInternalAndHandleSurvives(t *testing.T) {
	s := openServer(t, t.TempDir())
	s.register("test.panic", func(context.Context, *Server, []byte, func(Event)) (any, error) {
		panic("boom")
	})
	wantError(t, call(t, s, "test.panic", ""), CodeInternal)
	wantError(t, call(t, s, "status", ""), CodeNotConfigured)
}

func TestValidateDefinitionReturnsMessages(t *testing.T) {
	msgs := ValidateDefinition([]byte(missingCardDefinition))
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "paths.card: required") {
			found = true
		}
	}
	if !found {
		t.Fatalf("messages = %v", msgs)
	}
	if msgs := ValidateDefinition([]byte("id = ")); len(msgs) == 0 {
		t.Fatal("parse error gave no messages")
	}
}

func TestOpenErrorEnvelope(t *testing.T) {
	dir := t.TempDir()
	openServer(t, dir)
	_, err := Open(context.Background(), testOpenOptions(t, dir))
	if err == nil {
		t.Fatal("second open succeeded")
	}
	r := decode(t, ErrorEnvelope(err))
	wantError(t, r, CodeDataDirLocked)
	if r.Error.Message == "" {
		t.Fatal("empty message")
	}

	r = decode(t, ErrorEnvelope(errors.New("plain")))
	wantError(t, r, CodeInternal)
}

func TestOpenArtMaxSize(t *testing.T) {
	o, err := DecodeOpenOptions([]byte(`{"data_dir":"/x","art_max_size":{"width":640,"height":480}}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.ArtMaxSize.Width != 640 || o.ArtMaxSize.Height != 480 {
		t.Fatalf("art max size = %+v", o.ArtMaxSize)
	}
	if _, err := DecodeOpenOptions([]byte(`{"art_max_size":{"width":1,"bogus":2}}`)); err == nil {
		t.Fatal("unknown art_max_size field accepted")
	}
	for _, size := range []ArtMaxSize{{Width: -1}, {Height: -5}} {
		o := testOpenOptions(t, t.TempDir())
		o.ArtMaxSize = size
		_, err := Open(context.Background(), o)
		if err == nil {
			t.Fatalf("open with %+v succeeded", size)
		}
		wantError(t, decode(t, ErrorEnvelope(err)), CodeBadRequest)
	}
	o = testOpenOptions(t, t.TempDir())
	o.ArtMaxSize = ArtMaxSize{Width: 320, Height: 0}
	s, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}
