package leaf

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rommapp/grout/core/firmware/host"
)

func TestLeafRescanSendsFrame(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "jawakad.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- nil
			return
		}
		defer conn.Close()
		header := make([]byte, 4)
		if _, err := io.ReadFull(conn, header); err != nil {
			got <- nil
			return
		}
		body := make([]byte, binary.BigEndian.Uint32(header))
		if _, err := io.ReadFull(conn, body); err != nil {
			got <- nil
			return
		}
		got <- append(header, body...)
		reply := []byte(`{"type":"ok","action":"scan-library started"}`)
		out := binary.BigEndian.AppendUint32(nil, uint32(len(reply)))
		conn.Write(append(out, reply...))
	}()

	h := &host.Fake{Files: fstest.MapFS{}, Env: map[string]string{
		"JAWAKA_SOCKET_PATH": sock,
		"UMRK_RUNTIME_PATH":  "/nowhere",
	}}
	if err := Rescan(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"type":"scan-library"}`)
	want := append(binary.BigEndian.AppendUint32(nil, uint32(len(payload))), payload...)
	if frame := <-got; !bytes.Equal(frame, want) {
		t.Fatalf("frame = %q, want %q", frame, want)
	}
}

func TestLeafRescanNoSocket(t *testing.T) {
	dir := t.TempDir()
	h := &host.Fake{Files: fstest.MapFS{}, Env: map[string]string{"UMRK_RUNTIME_PATH": dir}}
	done := make(chan error, 1)
	go func() { done <- Rescan(context.Background(), h) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Rescan without a socket returned nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Rescan without a socket hung")
	}
}

func TestLeafSocketPathOrder(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"JAWAKA_SOCKET_PATH": "/a", "JAWAKAD_SOCKET_PATH": "/b", "UMRK_RUNTIME_PATH": "/c"}, "/a"},
		{map[string]string{"JAWAKAD_SOCKET_PATH": "/b", "UMRK_RUNTIME_PATH": "/c"}, "/b"},
		{map[string]string{"UMRK_RUNTIME_PATH": "/c", "JAWAKA_RUNTIME_DIR": "/d"}, "/c/jawakad.sock"},
		{map[string]string{"JAWAKA_RUNTIME_DIR": "/d", "TMPDIR": "/t"}, "/d/jawakad.sock"},
		{map[string]string{"TMPDIR": "/t"}, "/t/jawaka-runtime/jawakad.sock"},
		{nil, "/tmp/jawaka-runtime/jawakad.sock"},
	} {
		if got := socketPath(&host.Fake{Env: tc.env}); got != tc.want {
			t.Errorf("socketPath(%v) = %q, want %q", tc.env, got, tc.want)
		}
	}
}
