package leaf

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// rescanTimeout bounds the whole exchange with the launcher daemon. The scan
// itself runs after jawakad answers, so this only covers queueing it.
const rescanTimeout = 2 * time.Second

// maxFrame mirrors the daemon's own limit on a message.
const maxFrame = 16 << 20

// socketPath finds jawakad's control socket the way Leaf's own apps do: an
// explicit override, then the runtime directory jawakad exports to everything
// it launches, then its fallback under the temp directory.
func socketPath() string {
	for _, name := range []string{"JAWAKA_SOCKET_PATH", "JAWAKAD_SOCKET_PATH"} {
		if path := os.Getenv(name); path != "" {
			return path
		}
	}
	for _, name := range []string{"UMRK_RUNTIME_PATH", "JAWAKA_RUNTIME_DIR"} {
		if dir := os.Getenv(name); dir != "" {
			return filepath.Join(dir, "jawakad.sock")
		}
	}
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	return filepath.Join(tmp, "jawaka-runtime", "jawakad.sock")
}

// RequestLibraryRescan asks the Leaf launcher to rescan the library, so games
// just downloaded appear without the user running Rescan Library from the menu.
func RequestLibraryRescan() error {
	return requestLibraryRescan(socketPath())
}

func requestLibraryRescan(path string) error {
	conn, err := net.DialTimeout("unix", path, rescanTimeout)
	if err != nil {
		return fmt.Errorf("connecting to jawakad: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(rescanTimeout))

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

// jawakad frames every message as a big-endian uint32 length and a JSON body.
func writeFrame(w io.Writer, payload []byte) error {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
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
