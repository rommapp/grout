package sync

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const sfoMagic = "\x00PSF"

type sfoHeader struct {
	Magic           [4]byte
	Version         uint32
	KeyTableOffset  uint32
	DataTableOffset uint32
	NumEntries      uint32
}

type sfoIndexEntry struct {
	KeyOffset  uint16
	DataFormat uint16
	DataLen    uint32
	DataMaxLen uint32
	DataOffset uint32
}

const (
	sfoFormatUTF8  = 0x0204
	sfoFormatInt32 = 0x0404
)

// ParseSFO parses a PARAM.SFO file and returns the key-value pairs as a map.
func ParseSFO(data []byte) (map[string]string, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("SFO data too short")
	}

	var header sfoHeader
	if err := binary.Read(bytes.NewReader(data[:20]), binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read SFO header: %w", err)
	}

	if string(header.Magic[:]) != sfoMagic {
		return nil, fmt.Errorf("invalid SFO magic: %x", header.Magic)
	}

	result := make(map[string]string)

	for i := uint32(0); i < header.NumEntries; i++ {
		entryOffset := 20 + i*16
		if int(entryOffset+16) > len(data) {
			break
		}

		var entry sfoIndexEntry
		if err := binary.Read(bytes.NewReader(data[entryOffset:entryOffset+16]), binary.LittleEndian, &entry); err != nil {
			continue
		}

		keyStart := header.KeyTableOffset + uint32(entry.KeyOffset)
		if int(keyStart) >= len(data) {
			continue
		}
		keyEnd := bytes.IndexByte(data[keyStart:], 0)
		if keyEnd < 0 {
			continue
		}
		key := string(data[keyStart : keyStart+uint32(keyEnd)])

		dataStart := header.DataTableOffset + entry.DataOffset
		if int(dataStart) >= len(data) {
			continue
		}

		switch entry.DataFormat {
		case sfoFormatUTF8:
			end := dataStart + entry.DataLen
			if int(end) > len(data) {
				end = uint32(len(data))
			}
			val := data[dataStart:end]
			if idx := bytes.IndexByte(val, 0); idx >= 0 {
				val = val[:idx]
			}
			result[key] = string(val)
		case sfoFormatInt32:
			if int(dataStart+4) <= len(data) {
				v := binary.LittleEndian.Uint32(data[dataStart : dataStart+4])
				result[key] = fmt.Sprintf("%d", v)
			}
		}
	}

	return result, nil
}

// ReadPSPSaveTitle reads the PARAM.SFO inside a PPSSPP save directory
// and returns the game title with special characters stripped.
func ReadPSPSaveTitle(saveDirPath string) (string, bool) {
	sfoPath := filepath.Join(saveDirPath, "PARAM.SFO")
	data, err := os.ReadFile(sfoPath)
	if err != nil {
		return "", false
	}

	params, err := ParseSFO(data)
	if err != nil {
		return "", false
	}

	title, ok := params["TITLE"]
	if !ok || title == "" {
		title, ok = params["SAVEDATA_TITLE"]
	}

	if ok && title != "" {
		title = cleanTitle(title)
	}

	return title, ok && title != ""
}

// cleanTitle strips trademark symbols, copyright signs, and other non-standard
// characters that publishers embed in SFO titles but aren't in ROM filenames.
func cleanTitle(title string) string {
	var b strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_' || r == '\'' || r == '.' || r == '!' || r == '?' || r == ',' || r == ':' || r == '&' || r == '(' || r == ')' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
