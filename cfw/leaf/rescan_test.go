package leaf

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// fakeJawakad answers one request with reply and reports what it was sent.
func fakeJawakad(t *testing.T, reply string) (string, <-chan string) {
	t.Helper()

	// t.TempDir can exceed the unix socket path limit on macOS.
	dir, err := os.MkdirTemp("", "jawaka")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	path := filepath.Join(dir, "jawakad.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	received := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		request, err := readFrame(conn)
		if err != nil {
			return
		}
		received <- string(request)
		_ = writeFrame(conn, []byte(reply))
	}()

	return path, received
}

func TestRequestLibraryRescan_SendsScanLibrary(t *testing.T) {
	path, received := fakeJawakad(t, `{"type":"ok","game_count":3,"app_count":1}`)

	if err := requestLibraryRescan(path); err != nil {
		t.Fatalf("requestLibraryRescan() error = %v", err)
	}
	if got := <-received; got != `{"type":"scan-library"}` {
		t.Errorf("sent %s, want a scan-library request", got)
	}
}

func TestRequestLibraryRescan_ReportsRejection(t *testing.T) {
	path, _ := fakeJawakad(t, `{"type":"error","message":"busy"}`)

	if err := requestLibraryRescan(path); err == nil {
		t.Error("requestLibraryRescan() should fail when jawakad does not answer ok")
	}
}

func TestRequestLibraryRescan_NoDaemon(t *testing.T) {
	if err := requestLibraryRescan(filepath.Join(t.TempDir(), "missing.sock")); err == nil {
		t.Error("requestLibraryRescan() should fail without a daemon")
	}
}

func TestSocketPath(t *testing.T) {
	t.Setenv("JAWAKA_SOCKET_PATH", "")
	t.Setenv("JAWAKAD_SOCKET_PATH", "")
	t.Setenv("UMRK_RUNTIME_PATH", "/run/umrk")
	if got := socketPath(); got != "/run/umrk/jawakad.sock" {
		t.Errorf("socketPath() = %q, want the runtime directory's socket", got)
	}

	t.Setenv("JAWAKA_SOCKET_PATH", "/custom.sock")
	if got := socketPath(); got != "/custom.sock" {
		t.Errorf("socketPath() = %q, want the override", got)
	}
}
