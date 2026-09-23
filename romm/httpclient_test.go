package romm

import (
	"crypto/tls"
	"grout/settings"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// selfSignedServer returns a TLS test server using an untrusted certificate,
// standing in for a RomM instance with a self-signed cert.
func selfSignedServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A host that has opted into skipping verification must be able to fetch from a
// self-signed instance. Art, BIOS and cover downloads all go through plain
// http.Clients rather than romm.Client, so this is the seam that keeps them
// consistent with rom downloads.
func TestNewHTTPClient_HonoursInsecureSkipVerify(t *testing.T) {
	srv := selfSignedServer(t)

	host := settings.Host{InsecureSkipVerify: true}
	client := NewHTTPClient(host, DefaultClientTimeout)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("expected the request to succeed with InsecureSkipVerify, got: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// The default must stay strict: a host that has not opted in gets certificate
// verification.
func TestNewHTTPClient_VerifiesByDefault(t *testing.T) {
	srv := selfSignedServer(t)

	client := NewHTTPClient(settings.Host{}, DefaultClientTimeout)

	resp, err := client.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected a certificate error without InsecureSkipVerify")
	}
}

func TestNewHTTPClient_AppliesTimeout(t *testing.T) {
	client := NewHTTPClient(settings.Host{}, 5*time.Second)
	if client.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", client.Timeout, 5*time.Second)
	}

	// A zero timeout means the caller did not ask for one; fall back to the
	// package default rather than blocking forever.
	client = NewHTTPClient(settings.Host{}, 0)
	if client.Timeout != DefaultClientTimeout {
		t.Errorf("Timeout = %v, want the default %v", client.Timeout, DefaultClientTimeout)
	}
}

// Each call must return an independent client so that callers changing a
// timeout or transport cannot affect anyone else.
func TestNewHTTPClient_ReturnsIndependentClients(t *testing.T) {
	a := NewHTTPClient(settings.Host{InsecureSkipVerify: true}, time.Second)
	b := NewHTTPClient(settings.Host{}, time.Minute)

	if a == b {
		t.Fatal("expected distinct client instances")
	}
	if a.Transport == nil {
		t.Fatal("expected a transport when skipping verification")
	}

	at, ok := a.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", a.Transport)
	}
	if at.TLSClientConfig == nil || !at.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be set on the skipping client")
	}

	if b.Transport != nil {
		if bt, ok := b.Transport.(*http.Transport); ok && bt.TLSClientConfig != nil {
			if bt.TLSClientConfig.InsecureSkipVerify {
				t.Error("the verifying client must not skip verification")
			}
		}
	}

	// Guard against a shared tls.Config being mutated through one client.
	if b.Transport != nil {
		bt := b.Transport.(*http.Transport)
		if at.TLSClientConfig == bt.TLSClientConfig && at.TLSClientConfig != (*tls.Config)(nil) {
			t.Error("clients must not share a tls.Config")
		}
	}
}
