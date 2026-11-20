package grout

import (
	"context"
	"testing"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/test/fakeromm"
)

func TestStatusCodes(t *testing.T) {
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		srv := fakeromm.New(t)
		srv.SetHeartbeat("5.1.0", true)
		g := connected(t, srv)
		st, err := g.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if st.ServerVersion != "5.1.0" || !st.Snapshots || st.Host != srv.URL() || st.ConfigSource != "settings" {
			t.Fatalf("status = %+v", st)
		}
	})

	t.Run("too old", func(t *testing.T) {
		srv := fakeromm.New(t)
		srv.SetHeartbeat("4.9.2", false)
		g := connected(t, srv)
		st, err := g.Status(ctx)
		wantCode(t, err, CodeServerTooOld)
		if st.ServerVersion != "4.9.2" || st.Snapshots {
			t.Fatalf("status = %+v", st)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		srv := fakeromm.New(t)
		srv.SetUnauthorized(true)
		g := connected(t, srv)
		_, err := g.Status(ctx)
		wantCode(t, err, CodeUnauthorized)
	})

	t.Run("unreachable", func(t *testing.T) {
		srv := fakeromm.New(t)
		g := openTest(t)
		if err := g.SetHost(config.Host{RootURI: srv.URL()}); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		_, err := g.Status(ctx)
		wantCode(t, err, CodeServerUnreachable)
	})
}

func signedInHost(url string) config.Host {
	return config.Host{
		DisplayName:         "home",
		RootURI:             url,
		Username:            "alice",
		Token:               "tok-a",
		TokenName:           "handheld",
		TokenExpiresAt:      "2027-01-01T00:00:00Z",
		ClientDeviceID:      "0123456789abcdef0123456789abcdef",
		DeviceID:            "dev-1",
		DeviceName:          "handheld",
		DeviceClientVersion: "1.0.0",
	}
}

func TestSetHostSameURLKeepsSignIn(t *testing.T) {
	g := openTest(t)
	want := signedInHost("http://romm.local")
	if err := g.SetHost(want); err != nil {
		t.Fatal(err)
	}
	if err := g.SetHost(config.Host{DisplayName: "den", RootURI: "http://romm.local"}); err != nil {
		t.Fatal(err)
	}
	want.DisplayName = "den"
	if got := g.Settings().Hosts[0]; got != want {
		t.Fatalf("host = %+v, want %+v", got, want)
	}
	if err := g.SetHost(config.Host{DisplayName: "den", RootURI: "http://romm.local/", InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	want.RootURI = "http://romm.local/"
	want.InsecureSkipVerify = true
	if got := g.Settings().Hosts[0]; got != want {
		t.Fatalf("host = %+v, want %+v", got, want)
	}
}

func TestSetHostKeepsClientDeviceID(t *testing.T) {
	g := openTest(t)
	first := signedInHost("http://romm.local")
	if err := g.SetHost(first); err != nil {
		t.Fatal(err)
	}
	for _, h := range []config.Host{
		{RootURI: "http://romm.local"},
		{RootURI: "http://romm.local", Token: "tok-b", ClientDeviceID: "ffffffffffffffffffffffffffffffff"},
		{RootURI: "http://other.local", ClientDeviceID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
	} {
		if err := g.SetHost(h); err != nil {
			t.Fatal(err)
		}
		if got := g.Settings().Hosts[0].ClientDeviceID; got != first.ClientDeviceID {
			t.Fatalf("after %+v client device id = %q", h, got)
		}
	}
}

func TestSetHostNewURLDropsCarriedCredentials(t *testing.T) {
	ctx := context.Background()
	srvA := fakeromm.New(t)
	srvB := fakeromm.New(t)
	g := openTest(t)
	if err := g.SetHost(signedInHost(srvA.URL())); err != nil {
		t.Fatal(err)
	}
	h := g.Settings().Hosts[0]
	h.RootURI = srvB.URL()
	if err := g.SetHost(h); err != nil {
		t.Fatal(err)
	}
	want := config.Host{DisplayName: "home", RootURI: srvB.URL(), ClientDeviceID: h.ClientDeviceID}
	if got := g.Settings().Hosts[0]; got != want {
		t.Fatalf("host = %+v, want %+v", got, want)
	}
	_, _ = g.Status(ctx)
	_, _ = g.RegisterDevice(ctx, "handheld")
	if len(srvB.Calls()) == 0 {
		t.Fatal("no request reached the new server")
	}
	for _, c := range srvB.Calls() {
		if c.Auth == "Bearer tok-a" {
			t.Fatalf("old token sent to the new server: %+v", c)
		}
	}
}

func TestSetHostNewURLWithNewTokenKeepsIt(t *testing.T) {
	g := openTest(t)
	if err := g.SetHost(signedInHost("http://romm.local")); err != nil {
		t.Fatal(err)
	}
	next := config.Host{RootURI: "http://other.local", Username: "bob", Token: "tok-b", TokenName: "den", TokenExpiresAt: "2027-03-01T00:00:00.25+00:00", DeviceID: "dev-b"}
	if err := g.SetHost(next); err != nil {
		t.Fatal(err)
	}
	next.TokenExpiresAt = "2027-03-01T00:00:00Z"
	next.ClientDeviceID = "0123456789abcdef0123456789abcdef"
	if got := g.Settings().Hosts[0]; got != next {
		t.Fatalf("host = %+v, want %+v", got, next)
	}
}
