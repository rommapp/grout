package leaf

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path"
	"time"

	"github.com/rommapp/grout/core/firmware/host"
)

const (
	rescanTimeout = 2 * time.Second
	maxFrame      = 16 << 20
)

func socketPath(h host.Host) string {
	for _, name := range []string{"JAWAKA_SOCKET_PATH", "JAWAKAD_SOCKET_PATH"} {
		if p := h.Getenv(name); p != "" {
			return p
		}
	}
	for _, name := range []string{"UMRK_RUNTIME_PATH", "JAWAKA_RUNTIME_DIR"} {
		if dir := h.Getenv(name); dir != "" {
			return path.Join(dir, "jawakad.sock")
		}
	}
	tmp := h.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	return path.Join(tmp, "jawaka-runtime", "jawakad.sock")
}

func Rescan(ctx context.Context, h host.Host) error {
	ctx, cancel := context.WithTimeout(ctx, rescanTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socketPath(h))
	if err != nil {
		return fmt.Errorf("connecting to jawakad: %w", err)
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("connecting to jawakad: %w", err)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()

	if err := writeFrame(conn, []byte(`{"type":"scan-library"}`)); err != nil {
		return fmt.Errorf("sending rescan request: %w", err)
	}
	payload, err := readFrame(conn)
	if err != nil {
		return fmt.Errorf("reading rescan reply: %w", err)
	}
	var reply struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &reply); err != nil {
		return fmt.Errorf("parsing rescan reply: %w", err)
	}
	if reply.Type != "ok" {
		return fmt.Errorf("jawakad rejected the rescan: %s", payload)
	}
	return nil
}

func writeFrame(w io.Writer, payload []byte) error {
	frame := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(payload)), uint32(len(payload)))
	_, err := w.Write(append(frame, payload...))
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header)
	if size > maxFrame {
		return nil, fmt.Errorf("frame of %d bytes exceeds the limit", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
