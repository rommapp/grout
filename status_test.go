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
