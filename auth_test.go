package grout

import (
	"context"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/rommapp/grout/core/auth"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
	"github.com/rommapp/grout/version"
)

func instantTiming() auth.Timing {
	return auth.Timing{Sleep: func(ctx context.Context, d time.Duration) error { return ctx.Err() }}
}

func approved(token string) fakeromm.TokenReply {
	return fakeromm.TokenReply{Token: romm.DeviceToken{
		AccessToken: token,
		DeviceID:    "dev-9",
		Scopes:      romm.Scopes,
		ExpiresAt:   "2027-01-01T00:00:00Z",
	}}
}

func pairHandle(t *testing.T, srv *fakeromm.Server, edit func(*Options)) *Grout {
	t.Helper()
	o := testOptions(t, t.TempDir())
	o.Timing = instantTiming()
	if edit != nil {
		edit(&o)
	}
	g := open(t, o)
	if err := g.SetHost(config.Host{RootURI: srv.URL()}); err != nil {
		t.Fatal(err)
	}
	return g
}

func pairServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := fakeromm.New(t)
	srv.SetUsername("alice")
	return srv
}

func TestPairStartNeedsHostAndName(t *testing.T) {
	ctx := context.Background()
	g := openTest(t)
	_, err := g.PairStart(ctx, "handheld")
	wantCode(t, err, CodeNotConfigured)

	srv := pairServer(t)
	g = pairHandle(t, srv, nil)
	_, err = g.PairStart(ctx, "")
	wantCode(t, err, CodeBadRequest)
	if len(srv.InitRequests()) != 0 {
		t.Fatalf("init requests = %d", len(srv.InitRequests()))
	}
}

func TestPairStartSendsClientIdentityAndFirmware(t *testing.T) {
	ctx := context.Background()
	hexID := regexp.MustCompile(`^[0-9a-f]{32}$`)

	srv := pairServer(t)
	g := pairHandle(t, srv, func(o *Options) { o.Client = Client{Name: "cannoli", Version: "2.3.0"} })
	p, err := g.PairStart(ctx, "handheld")
	if err != nil {
		t.Fatal(err)
	}
	if p.UserCode != "WXYZ-2345" || p.VerificationURL != srv.URL()+"/pair?code=WXYZ-2345" || p.ExpiresAt.IsZero() {
		t.Fatalf("pending = %+v", p)
	}
	reqs := srv.InitRequests()
	if len(reqs) != 1 {
		t.Fatalf("init requests = %d", len(reqs))
	}
	r := reqs[0]
	if r.Client != "cannoli" || r.ClientVersion != "2.3.0" || r.Platform != "testfw" || r.Name != "handheld" {
		t.Fatalf("init = %+v", r)
	}
	if !slices.Equal(r.RequestedScopes, []string{"me.read", "platforms.read", "roms.read", "collections.read", "firmware.read", "assets.read", "assets.write", "devices.read", "devices.write"}) {
		t.Fatalf("scopes = %v", r.RequestedScopes)
	}
	if !hexID.MatchString(r.ClientDeviceIdentifier) {
		t.Fatalf("client device identifier = %q", r.ClientDeviceIdentifier)
	}
	if got := g.Settings().Hosts[0].ClientDeviceID; got != r.ClientDeviceIdentifier {
		t.Fatalf("stored client device id = %q, sent %q", got, r.ClientDeviceIdentifier)
	}

	srv2 := pairServer(t)
	g2 := pairHandle(t, srv2, nil)
	if _, err := g2.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	r2 := srv2.InitRequests()[0]
	if r2.Client != "grout" || r2.ClientVersion != version.Version {
		t.Fatalf("default identity = %q %q", r2.Client, r2.ClientVersion)
	}
	if !hexID.MatchString(r2.ClientDeviceIdentifier) || r2.ClientDeviceIdentifier == r.ClientDeviceIdentifier {
		t.Fatalf("client device identifier = %q", r2.ClientDeviceIdentifier)
	}
}

func TestPairStartKeepsClientDeviceIDAcrossLogout(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.SetTokenReplies(approved("tok-1"))
	g := pairHandle(t, srv, nil)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.PairWait(ctx); err != nil {
		t.Fatal(err)
	}
	if err := g.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	reqs := srv.InitRequests()
	if len(reqs) != 2 || reqs[0].ClientDeviceIdentifier == "" || reqs[0].ClientDeviceIdentifier != reqs[1].ClientDeviceIdentifier {
		t.Fatalf("init requests = %+v", reqs)
	}
}

func TestPairWaitApprovedSavesHost(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.SetTokenReplies(fakeromm.TokenReply{State: romm.PairPending}, fakeromm.TokenReply{State: romm.PairSlowDown}, approved("tok-1"))
	g := pairHandle(t, srv, func(o *Options) { o.Client = Client{Name: "cannoli", Version: "2.3.0"} })
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	res, err := g.PairWait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != auth.OutcomeApproved || res.Username != "alice" || res.DeviceID != "dev-9" || res.ExpiresAt != "2027-01-01T00:00:00Z" {
		t.Fatalf("result = %+v", res)
	}
	h := g.Settings().Hosts[0]
	if h.Token != "tok-1" || h.TokenName != "handheld" || h.TokenExpiresAt != "2027-01-01T00:00:00Z" ||
		h.DeviceID != "dev-9" || h.DeviceName != "handheld" || h.DeviceClientVersion != "2.3.0" || h.Username != "alice" {
		t.Fatalf("host = %+v", h)
	}
	settled := false
	for _, c := range srv.CallsTo("/api/platforms") {
		if c.Auth == "Bearer tok-1" {
			settled = true
		}
	}
	if !settled {
		t.Fatal("token was not settled against /api/platforms")
	}
	_, err = g.PairWait(ctx)
	wantCode(t, err, CodeBadRequest)
}

func TestPairWaitApprovalSurvivesCancel(t *testing.T) {
	srv := pairServer(t)
	srv.SetTokenReplies(approved("tok-1"))
	g := pairHandle(t, srv, nil)
	if _, err := g.PairStart(context.Background(), "handheld"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.SetTokenHook(cancel)
	res, err := g.PairWait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != auth.OutcomeApproved {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	h := g.Settings().Hosts[0]
	if h.Token != "tok-1" || h.DeviceID != "dev-9" || h.Username != "alice" {
		t.Fatalf("host = %+v", h)
	}
}

func TestPairWaitWithoutPendingIsBadRequest(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.SetTokenReplies(fakeromm.TokenReply{State: romm.PairDenied})
	g := pairHandle(t, srv, nil)
	_, err := g.PairWait(ctx)
	wantCode(t, err, CodeBadRequest)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	if res, err := g.PairWait(ctx); err != nil || res.Outcome != auth.OutcomeDenied {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	_, err = g.PairWait(ctx)
	wantCode(t, err, CodeBadRequest)
}

func TestPairWaitConcurrentIsBusy(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.SetTokenReplies(approved("tok-1"))
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	g := pairHandle(t, srv, func(o *Options) {
		o.Timing.Sleep = func(ctx context.Context, d time.Duration) error {
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	type result struct {
		res PairResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := g.PairWait(ctx)
		done <- result{res, err}
	}()
	<-entered
	_, err := g.PairWait(ctx)
	wantCode(t, err, CodeBusy)
	close(release)
	r := <-done
	if r.err != nil || r.res.Outcome != auth.OutcomeApproved {
		t.Fatalf("first wait = %+v, %v", r.res, r.err)
	}
}

func TestPairWaitDeniedAndExpiredClearPending(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		state romm.PairState
		want  auth.Outcome
	}{
		{romm.PairDenied, auth.OutcomeDenied},
		{romm.PairExpired, auth.OutcomeExpired},
	} {
		t.Run(string(tc.want), func(t *testing.T) {
			srv := pairServer(t)
			srv.SetTokenReplies(fakeromm.TokenReply{State: tc.state})
			g := pairHandle(t, srv, nil)
			if _, err := g.PairStart(ctx, "handheld"); err != nil {
				t.Fatal(err)
			}
			res, err := g.PairWait(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != tc.want {
				t.Fatalf("outcome = %q, want %q", res.Outcome, tc.want)
			}
			if h := g.Settings().Hosts[0]; h.Token != "" {
				t.Fatalf("host = %+v", h)
			}
			_, err = g.PairWait(ctx)
			wantCode(t, err, CodeBadRequest)
		})
	}
}

func TestPairWaitCancelledKeepsPending(t *testing.T) {
	srv := pairServer(t)
	srv.SetTokenReplies(approved("tok-1"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelNext := true
	g := pairHandle(t, srv, func(o *Options) {
		o.Timing.Sleep = func(sctx context.Context, d time.Duration) error {
			if cancelNext {
				cancelNext = false
				cancel()
			}
			return sctx.Err()
		}
	})
	if _, err := g.PairStart(context.Background(), "handheld"); err != nil {
		t.Fatal(err)
	}
	_, err := g.PairWait(ctx)
	wantCode(t, err, CodeCancelled)
	if n := len(srv.CallsTo("/api/auth/device/token")); n != 0 {
		t.Fatalf("token polls = %d", n)
	}
	res, err := g.PairWait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != auth.OutcomeApproved || g.Settings().Hosts[0].Token != "tok-1" {
		t.Fatalf("res = %+v, host = %+v", res, g.Settings().Hosts[0])
	}
}

func exchangeToken() romm.ClientToken {
	return romm.ClientToken{
		ID:        4,
		Name:      "living room",
		Scopes:    romm.Scopes,
		ExpiresAt: "2027-02-01T00:00:00Z",
		DeviceID:  "dev-3",
		RawToken:  "raw-1",
	}
}

func TestExchangeCodeSavesTokenAndDevice(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.AddExchangeCode("ABCD2345", exchangeToken())
	g := pairHandle(t, srv, nil)
	res, err := g.ExchangeCode(ctx, "abcd-2345")
	if err != nil {
		t.Fatal(err)
	}
	if res.Username != "alice" || res.DeviceID != "dev-3" || res.ExpiresAt != "2027-02-01T00:00:00Z" || !slices.Equal(res.Scopes, romm.Scopes) {
		t.Fatalf("result = %+v", res)
	}
	h := g.Settings().Hosts[0]
	if h.Token != "raw-1" || h.TokenName != "living room" || h.TokenExpiresAt != "2027-02-01T00:00:00Z" || h.DeviceID != "dev-3" || h.Username != "alice" {
		t.Fatalf("host = %+v", h)
	}
}

func TestExchangeCodeErrorCodes(t *testing.T) {
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		g := pairHandle(t, pairServer(t), nil)
		_, err := g.ExchangeCode(ctx, " - ")
		wantCode(t, err, CodeBadRequest)
		_, err = g.ExchangeCode(ctx, "")
		wantCode(t, err, CodeBadRequest)
	})

	t.Run("not found", func(t *testing.T) {
		g := pairHandle(t, pairServer(t), nil)
		_, err := g.ExchangeCode(ctx, "ZZZZ9999")
		wantCode(t, err, CodeInvalidCode)
	})

	t.Run("forced 404", func(t *testing.T) {
		srv := pairServer(t)
		srv.AddExchangeCode("ABCD2345", exchangeToken())
		srv.SetExchangeStatus(404)
		g := pairHandle(t, srv, nil)
		_, err := g.ExchangeCode(ctx, "ABCD2345")
		wantCode(t, err, CodeInvalidCode)
	})

	t.Run("rate limited", func(t *testing.T) {
		srv := pairServer(t)
		srv.SetExchangeStatus(429)
		g := pairHandle(t, srv, nil)
		_, err := g.ExchangeCode(ctx, "ABCD2345")
		wantCode(t, err, CodeRateLimited)
	})

	t.Run("no host", func(t *testing.T) {
		g := openTest(t)
		_, err := g.ExchangeCode(ctx, "ABCD2345")
		wantCode(t, err, CodeNotConfigured)
	})
}

func TestExchangeCodeRejectedTokenSavesNothing(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.AddExchangeCode("ABCD2345", exchangeToken())
	srv.RejectToken("raw-1")
	g := pairHandle(t, srv, nil)
	before := g.Settings().Hosts[0]
	_, err := g.ExchangeCode(ctx, "ABCD2345")
	wantCode(t, err, CodeUnauthorized)
	if after := g.Settings().Hosts[0]; after != before {
		t.Fatalf("host changed: %+v -> %+v", before, after)
	}
}

func TestLogoutKeepsServerAndClientDeviceID(t *testing.T) {
	g := openTest(t)
	err := g.SetHost(config.Host{
		DisplayName:         "home",
		RootURI:             "http://romm.local",
		Port:                8080,
		Username:            "alice",
		Token:               "tok",
		TokenName:           "handheld",
		TokenExpiresAt:      "2027-01-01T00:00:00Z",
		InsecureSkipVerify:  true,
		ClientDeviceID:      "0123456789abcdef0123456789abcdef",
		DeviceID:            "dev-1",
		DeviceName:          "handheld",
		DeviceClientVersion: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Logout(); err != nil {
		t.Fatal(err)
	}
	want := config.Host{
		DisplayName:        "home",
		RootURI:            "http://romm.local",
		Port:               8080,
		InsecureSkipVerify: true,
		ClientDeviceID:     "0123456789abcdef0123456789abcdef",
	}
	if got := g.Settings().Hosts[0]; got != want {
		t.Fatalf("host = %+v, want %+v", got, want)
	}

	wantCode(t, openTest(t).Logout(), CodeNotConfigured)
}

func TestRegisterDeviceRegistersOnceThenUpdatesVersion(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	dataDir := t.TempDir()
	o := testOptions(t, dataDir)
	o.Client = Client{Name: "cannoli", Version: "1.0.0"}
	g := open(t, o)

	_, err := g.RegisterDevice(ctx, "handheld")
	wantCode(t, err, CodeNotConfigured)
	if err := g.SetHost(config.Host{RootURI: srv.URL()}); err != nil {
		t.Fatal(err)
	}
	_, err = g.RegisterDevice(ctx, "handheld")
	wantCode(t, err, CodeNotConfigured)
	if err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	_, err = g.RegisterDevice(ctx, "")
	wantCode(t, err, CodeBadRequest)

	info, err := g.RegisterDevice(ctx, "handheld")
	if err != nil {
		t.Fatal(err)
	}
	if info.DeviceID != "device-1" || info.Name != "handheld" {
		t.Fatalf("info = %+v", info)
	}
	reqs := srv.DeviceRequests()
	if len(reqs) != 2 {
		t.Fatalf("device requests = %+v", reqs)
	}
	post, put := reqs[0], reqs[1]
	if post.Method != "POST" || post.Body["name"] != "handheld" || post.Body["platform"] != "testfw" ||
		post.Body["client"] != "cannoli" || post.Body["client_version"] != "1.0.0" || post.Body["sync_mode"] != "api" {
		t.Fatalf("register = %+v", post)
	}
	if put.Method != "PUT" || put.DeviceID != "device-1" || put.Body["client_version"] != "1.0.0" {
		t.Fatalf("update = %+v", put)
	}
	h := g.Settings().Hosts[0]
	if h.DeviceID != "device-1" || h.DeviceName != "handheld" || h.DeviceClientVersion != "1.0.0" {
		t.Fatalf("host = %+v", h)
	}

	info, err = g.RegisterDevice(ctx, "handheld")
	if err != nil {
		t.Fatal(err)
	}
	if info.DeviceID != "device-1" || len(srv.DeviceRequests()) != 2 {
		t.Fatalf("info = %+v, requests = %+v", info, srv.DeviceRequests())
	}

	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	o.Client.Version = "1.1.0"
	g = open(t, o)
	info, err = g.RegisterDevice(ctx, "handheld")
	if err != nil {
		t.Fatal(err)
	}
	reqs = srv.DeviceRequests()
	if info.DeviceID != "device-1" || len(reqs) != 3 {
		t.Fatalf("info = %+v, requests = %+v", info, reqs)
	}
	if last := reqs[2]; last.Method != "PUT" || last.DeviceID != "device-1" || last.Body["client_version"] != "1.1.0" {
		t.Fatalf("update = %+v", last)
	}
	if h := g.Settings().Hosts[0]; h.DeviceClientVersion != "1.1.0" {
		t.Fatalf("host = %+v", h)
	}
}

func TestPairWaitAfterSetHostToOtherServerIsBadRequest(t *testing.T) {
	ctx := context.Background()
	srvA := pairServer(t)
	srvA.SetTokenReplies(approved("tok-a"))
	srvB := pairServer(t)
	g := pairHandle(t, srvA, nil)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	if err := g.SetHost(config.Host{RootURI: srvB.URL()}); err != nil {
		t.Fatal(err)
	}
	_, err := g.PairWait(ctx)
	wantCode(t, err, CodeBadRequest)
	if n := len(srvA.CallsTo("/api/auth/device/token")) + len(srvB.CallsTo("/api/auth/device/token")); n != 0 {
		t.Fatalf("token polls = %d", n)
	}
	if h := g.Settings().Hosts[0]; h.Token != "" {
		t.Fatalf("host = %+v", h)
	}
}

func TestApprovalNotSavedWhenHostChangedDuringWait(t *testing.T) {
	ctx := context.Background()
	srvA := pairServer(t)
	srvA.SetTokenReplies(approved("tok-a"))
	srvB := pairServer(t)
	g := pairHandle(t, srvA, nil)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	srvA.SetTokenHook(func() {
		if err := g.SetHost(config.Host{RootURI: srvB.URL()}); err != nil {
			t.Error(err)
		}
	})
	_, err := g.PairWait(ctx)
	wantCode(t, err, CodeBadRequest)
	if h := g.Settings().Hosts[0]; h.RootURI != srvB.URL() || h.Token != "" || h.DeviceID != "" {
		t.Fatalf("host = %+v", h)
	}
	for _, srv := range []*fakeromm.Server{srvA, srvB} {
		for _, c := range srv.Calls() {
			if c.Auth == "Bearer tok-a" {
				t.Fatalf("token sent: %+v", c)
			}
		}
	}
	_, err = g.PairWait(ctx)
	wantCode(t, err, CodeBadRequest)
}

func TestRegisterDeviceFailedPutDoesNotDuplicate(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.FailTimes("/api/devices/device-1", 1)
	o := testOptions(t, t.TempDir())
	o.Client = Client{Name: "cannoli", Version: "1.0.0"}
	g := open(t, o)
	if err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.RegisterDevice(ctx, "handheld"); err == nil {
		t.Fatal("expected the failed update to fail the call")
	}
	if h := g.Settings().Hosts[0]; h.DeviceID != "device-1" || h.DeviceName != "handheld" || h.DeviceClientVersion != "" {
		t.Fatalf("host after failed put = %+v", h)
	}
	info, err := g.RegisterDevice(ctx, "handheld")
	if err != nil {
		t.Fatal(err)
	}
	if info.DeviceID != "device-1" {
		t.Fatalf("info = %+v", info)
	}
	reqs := srv.DeviceRequests()
	if len(reqs) != 2 || reqs[0].Method != "POST" || reqs[1].Method != "PUT" || reqs[1].Body["client_version"] != "1.0.0" {
		t.Fatalf("device requests = %+v", reqs)
	}
	if n := len(srv.CallsTo("/api/devices/device-1")); n != 2 {
		t.Fatalf("put attempts = %d", n)
	}
	if h := g.Settings().Hosts[0]; h.DeviceClientVersion != "1.0.0" {
		t.Fatalf("host = %+v", h)
	}
}

func noRequestCarries(t *testing.T, srv *fakeromm.Server, token string) {
	t.Helper()
	for _, c := range srv.Calls() {
		if c.Auth == "Bearer "+token {
			t.Fatalf("token sent to the other server: %+v", c)
		}
	}
}

func TestExchangeCodeNotSavedWhenHostChanged(t *testing.T) {
	ctx := context.Background()
	srvA := pairServer(t)
	srvA.AddExchangeCode("ABCD2345", exchangeToken())
	srvB := pairServer(t)
	g := pairHandle(t, srvA, nil)
	srvA.SetPathHook("/api/client-tokens/exchange", func() {
		if err := g.SetHost(config.Host{RootURI: srvB.URL()}); err != nil {
			t.Error(err)
		}
	})
	_, err := g.ExchangeCode(ctx, "ABCD2345")
	wantCode(t, err, CodeBadRequest)
	if h := g.Settings().Hosts[0]; h.RootURI != srvB.URL() || h.Token != "" || h.DeviceID != "" || h.Username != "" {
		t.Fatalf("host = %+v", h)
	}
	noRequestCarries(t, srvB, "raw-1")
}

func TestRegisterDeviceNotSavedWhenHostChanged(t *testing.T) {
	ctx := context.Background()
	for _, path := range []string{"/api/devices", "/api/devices/device-1"} {
		t.Run(path, func(t *testing.T) {
			srvA := pairServer(t)
			srvB := pairServer(t)
			g := pairHandle(t, srvA, nil)
			if err := g.SetHost(config.Host{RootURI: srvA.URL(), Token: "tok-a"}); err != nil {
				t.Fatal(err)
			}
			srvA.SetPathHook(path, func() {
				if err := g.SetHost(config.Host{RootURI: srvB.URL(), Token: "tok-b"}); err != nil {
					t.Error(err)
				}
			})
			_, err := g.RegisterDevice(ctx, "handheld")
			wantCode(t, err, CodeBadRequest)
			if h := g.Settings().Hosts[0]; h.RootURI != srvB.URL() || h.Token != "tok-b" || h.DeviceID != "" || h.DeviceName != "" || h.DeviceClientVersion != "" {
				t.Fatalf("host = %+v", h)
			}
			noRequestCarries(t, srvB, "tok-a")
			if n := len(srvB.DeviceRequests()); n != 0 {
				t.Fatalf("device requests to B = %d", n)
			}
		})
	}
}

func TestExchangeCodeSurvivesCancelDuringExchange(t *testing.T) {
	srv := pairServer(t)
	srv.AddExchangeCode("ABCD2345", exchangeToken())
	g := pairHandle(t, srv, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.SetPathHook("/api/client-tokens/exchange", cancel)
	_, err := g.ExchangeCode(ctx, "ABCD2345")
	wantCode(t, err, CodeCancelled)
	h := g.Settings().Hosts[0]
	if h.Token != "raw-1" || h.TokenName != "living room" || h.DeviceID != "dev-3" {
		t.Fatalf("host = %+v", h)
	}
}

func TestExchangeCodeKeepsTokenWhenValidationFailsTransiently(t *testing.T) {
	srv := pairServer(t)
	srv.AddExchangeCode("ABCD2345", exchangeToken())
	srv.FailPath("/api/platforms")
	g := pairHandle(t, srv, nil)
	_, err := g.ExchangeCode(context.Background(), "ABCD2345")
	wantCode(t, err, CodeServerUnreachable)
	h := g.Settings().Hosts[0]
	if h.Token != "raw-1" || h.TokenName != "living room" || h.DeviceID != "dev-3" {
		t.Fatalf("host = %+v", h)
	}
}

func TestExchangeCodeRejectedTokenIsCleared(t *testing.T) {
	srv := pairServer(t)
	srv.AddExchangeCode("ABCD2345", exchangeToken())
	srv.RejectToken("raw-1")
	g := pairHandle(t, srv, nil)
	_, err := g.ExchangeCode(context.Background(), "ABCD2345")
	wantCode(t, err, CodeUnauthorized)
	h := g.Settings().Hosts[0]
	if h.Token != "" || h.TokenName != "" || h.TokenExpiresAt != "" || h.Username != "" ||
		h.DeviceID != "" || h.DeviceName != "" || h.DeviceClientVersion != "" {
		t.Fatalf("host = %+v", h)
	}
}

func TestExchangeCodeClearsStaleDevice(t *testing.T) {
	srv := pairServer(t)
	tok := exchangeToken()
	tok.DeviceID = ""
	srv.AddExchangeCode("ABCD2345", tok)
	g := pairHandle(t, srv, nil)
	err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "old", DeviceID: "dev-old", DeviceName: "handheld", DeviceClientVersion: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := g.ExchangeCode(context.Background(), "ABCD2345")
	if err != nil {
		t.Fatal(err)
	}
	h := g.Settings().Hosts[0]
	if res.DeviceID != "" || h.Token != "raw-1" || h.DeviceID != "" || h.DeviceName != "" || h.DeviceClientVersion != "" {
		t.Fatalf("result = %+v, host = %+v", res, h)
	}
}

func TestPairStartHostChangedDuringConnect(t *testing.T) {
	srvA := pairServer(t)
	srvB := pairServer(t)
	g := pairHandle(t, srvA, nil)
	srvA.SetPathHook("/api/heartbeat", func() {
		if err := g.SetHost(config.Host{RootURI: srvB.URL()}); err != nil {
			t.Error(err)
		}
	})
	_, err := g.PairStart(context.Background(), "handheld")
	wantCode(t, err, CodeBadRequest)
	if n := len(srvA.InitRequests()) + len(srvB.InitRequests()); n != 0 {
		t.Fatalf("init requests = %d", n)
	}
	g.pairMu.Lock()
	pending := g.pending
	g.pairMu.Unlock()
	if pending != nil {
		t.Fatalf("pending = %+v", pending)
	}
}

func TestPairWaitFailedIsServerUnreachable(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.FailTimes("/api/auth/device/token", 3)
	g := pairHandle(t, srv, nil)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	res, err := g.PairWait(ctx)
	wantCode(t, err, CodeServerUnreachable)
	if res.Outcome != auth.OutcomeFailed {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if n := len(srv.CallsTo("/api/auth/device/token")); n != 3 {
		t.Fatalf("token polls = %d", n)
	}
}

func TestPairWaitApprovedWithoutTokenFails(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	srv.SetTokenReplies(approved(""))
	g := pairHandle(t, srv, nil)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	res, err := g.PairWait(ctx)
	wantCode(t, err, CodeServerUnreachable)
	if res.Outcome != auth.OutcomeFailed {
		t.Fatalf("outcome = %q", res.Outcome)
	}
	if h := g.Settings().Hosts[0]; h.Token != "" || h.DeviceID != "" || h.TokenName != "" || h.DeviceName != "" {
		t.Fatalf("host = %+v", h)
	}
}

func TestPairWaitNormalizesExpiry(t *testing.T) {
	ctx := context.Background()
	srv := pairServer(t)
	reply := approved("tok-1")
	reply.Token.ExpiresAt = "2027-01-01T02:00:00.123456+02:00"
	srv.SetTokenReplies(reply)
	g := pairHandle(t, srv, nil)
	if _, err := g.PairStart(ctx, "handheld"); err != nil {
		t.Fatal(err)
	}
	res, err := g.PairWait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpiresAt != "2027-01-01T00:00:00Z" || g.Settings().Hosts[0].TokenExpiresAt != "2027-01-01T00:00:00Z" {
		t.Fatalf("result = %+v, host = %+v", res, g.Settings().Hosts[0])
	}
}

func TestExchangeCodeNormalizesExpiry(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"2027-02-01T00:00:00.654321+00:00", "2027-02-01T00:00:00Z"},
		{"not a time", ""},
	} {
		srv := pairServer(t)
		tok := exchangeToken()
		tok.ExpiresAt = tc.in
		srv.AddExchangeCode("ABCD2345", tok)
		g := pairHandle(t, srv, nil)
		res, err := g.ExchangeCode(context.Background(), "ABCD2345")
		if err != nil {
			t.Fatal(err)
		}
		if res.ExpiresAt != tc.want || g.Settings().Hosts[0].TokenExpiresAt != tc.want {
			t.Fatalf("%q: result = %+v, host = %+v", tc.in, res, g.Settings().Hosts[0])
		}
	}
}
