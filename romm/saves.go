package romm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sonh/qs"
)

const (
	maxSaveListBytes     = int64(16 * 1024 * 1024)
	maxSaveListRecords   = 10000
	maxSaveListErrorBody = int64(64 * 1024)
)

type saveListLimits struct {
	bytes   int64
	records int
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

type Save struct {
	ID             int       `json:"id"`
	RomID          int       `json:"rom_id"`
	UserID         int       `json:"user_id"`
	FileName       string    `json:"file_name"`
	FileNameNoTags string    `json:"file_name_no_tags"`
	FileNameNoExt  string    `json:"file_name_no_ext"`
	FileExtension  string    `json:"file_extension"`
	FilePath       string    `json:"file_path"`
	FileSizeBytes  int64     `json:"file_size_bytes"`
	FullPath       string    `json:"full_path"`
	DownloadPath   string    `json:"download_path"`
	MissingFromFs  bool      `json:"missing_from_fs"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Emulator       string    `json:"emulator"`
	Screenshot     struct {
		ID             int       `json:"id"`
		RomID          int       `json:"rom_id"`
		UserID         int       `json:"user_id"`
		FileName       string    `json:"file_name"`
		FileNameNoTags string    `json:"file_name_no_tags"`
		FileNameNoExt  string    `json:"file_name_no_ext"`
		FileExtension  string    `json:"file_extension"`
		FilePath       string    `json:"file_path"`
		FileSizeBytes  int64     `json:"file_size_bytes"`
		FullPath       string    `json:"full_path"`
		DownloadPath   string    `json:"download_path"`
		MissingFromFs  bool      `json:"missing_from_fs"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	} `json:"screenshot"`

	// New fields for device-aware saves
	Slot        *string          `json:"slot,omitempty"`
	ContentHash *string          `json:"content_hash,omitempty"`
	DeviceSyncs []DeviceSaveSync `json:"device_syncs,omitempty"`
}

type DeviceSaveSync struct {
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	IsUntracked  bool      `json:"is_untracked"`
	IsCurrent    bool      `json:"is_current"`
}

type SaveSummary struct {
	TotalCount int            `json:"total_count"`
	Slots      []SaveSlotInfo `json:"slots"`
}

type SaveSlotInfo struct {
	Slot   *string `json:"slot"`
	Count  int     `json:"count"`
	Latest Save    `json:"latest"`
}

type SaveQuery struct {
	RomID      int    `qs:"rom_id,omitempty"`
	Emulator   string `qs:"emulator,omitempty"`
	PlatformID int    `qs:"platform_id,omitempty"`
	DeviceID   string `qs:"device_id,omitempty"`
	Slot       string `qs:"slot,omitempty"`
}

func (sq SaveQuery) Valid() bool {
	return sq.RomID != 0 || sq.PlatformID != 0 || sq.DeviceID != ""
}

type UploadSaveQuery struct {
	RomID            int    `qs:"rom_id,omitempty"`
	DeviceID         string `qs:"device_id,omitempty"`
	Slot             string `qs:"slot,omitempty"`
	Emulator         string `qs:"emulator,omitempty"`
	Overwrite        bool   `qs:"overwrite,omitempty"`
	Autocleanup      bool   `qs:"autocleanup,omitempty"`
	AutocleanupLimit int    `qs:"autocleanup_limit,omitempty"`
}

func (uq UploadSaveQuery) Valid() bool {
	return uq.RomID != 0
}

type SaveContentQuery struct {
	DeviceID string `qs:"device_id,omitempty"`
	// No omitempty: the server defaults optimistic to true, and grout deliberately
	// sends optimistic=false so the device isn't marked synced until the file is
	// written and POST /downloaded confirms it. omitempty would drop the false value.
	Optimistic bool `qs:"optimistic"`
}

func (scq SaveContentQuery) Valid() bool {
	return scq.DeviceID != ""
}

type SaveDeviceBody struct {
	DeviceID string `json:"device_id"`
}

type SaveSummaryQuery struct {
	RomID int `qs:"rom_id"`
}

func (ssq SaveSummaryQuery) Valid() bool {
	return ssq.RomID != 0
}

func (c *Client) GetSaves(query SaveQuery) ([]Save, error) {
	var saves []Save
	err := c.doRequest("GET", endpointSaves, query, nil, &saves)
	return saves, err
}

// GetSavesForROMIDs fetches a bounded save list and returns matching ROMs only
// after the complete response has been validated.
func (c *Client) GetSavesForROMIDs(query SaveQuery, romIDs []int) ([]Save, error) {
	return c.streamSavesForROMIDs(query, romIDs, saveListLimits{
		bytes:   maxSaveListBytes,
		records: maxSaveListRecords,
	})
}

func (c *Client) streamSavesForROMIDs(query SaveQuery, romIDs []int, limits saveListLimits) ([]Save, error) {
	wanted := make(map[int]struct{}, len(romIDs))
	for _, romID := range romIDs {
		if romID > 0 {
			wanted[romID] = struct{}{}
		}
	}
	if !query.Valid() {
		return nil, fmt.Errorf("save list query is invalid")
	}
	if len(wanted) == 0 {
		return []Save{}, nil
	}

	req, err := http.NewRequest(http.MethodGet, c.baseURL+endpointSaves, nil)
	if err != nil {
		return nil, err
	}
	values, err := qs.NewEncoder().Values(query)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = values.Encode()
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return []Save{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxSaveListErrorBody)); err != nil {
			return nil, fmt.Errorf("save list request failed with status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("save list request failed with status %d", resp.StatusCode)
	}

	return decodeSaveList(resp.Body, wanted, limits)
}

func decodeSaveList(body io.Reader, wanted map[int]struct{}, limits saveListLimits) ([]Save, error) {
	reader := &countingReader{r: io.LimitReader(body, limits.bytes+1)}
	decoder := json.NewDecoder(reader)
	fail := func(err error) ([]Save, error) {
		if reader.n > limits.bytes {
			return nil, fmt.Errorf("save list decoded byte limit exceeded")
		}
		return nil, err
	}

	token, err := decoder.Token()
	if err != nil {
		return fail(err)
	}
	if opening, ok := token.(json.Delim); !ok || opening != '[' {
		return fail(fmt.Errorf("save list must be a top-level array"))
	}

	retained := make([]Save, 0)
	records := 0
	for decoder.More() {
		var save Save
		if err := decoder.Decode(&save); err != nil {
			return fail(err)
		}
		records++
		if records > limits.records {
			return fail(fmt.Errorf("save list record limit exceeded"))
		}
		if _, ok := wanted[save.RomID]; ok && save.RomID > 0 {
			retained = append(retained, save)
		}
	}

	closing, err := decoder.Token()
	if err != nil {
		return fail(err)
	}
	if delim, ok := closing.(json.Delim); !ok || delim != ']' {
		return fail(fmt.Errorf("save list closing array token missing"))
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		return fail(err)
	}
	if reader.n > limits.bytes {
		return fail(fmt.Errorf("save list decoded byte limit exceeded"))
	}
	return retained, nil
}

func (c *Client) DownloadSave(downloadPath string) ([]byte, error) {
	return c.doRequestRaw("GET", downloadPath, nil)
}

func (c *Client) DownloadSaveByID(saveID int, deviceID string, optimistic bool) ([]byte, error) {
	path := fmt.Sprintf(endpointSaveContent, saveID)
	query := SaveContentQuery{
		DeviceID:   deviceID,
		Optimistic: optimistic,
	}
	return c.doRequestRawWithQuery("GET", path, query)
}

func (c *Client) ConfirmSaveDownloaded(saveID int, deviceID string) error {
	path := fmt.Sprintf(endpointSaveDownloaded, saveID)
	body := SaveDeviceBody{DeviceID: deviceID}
	return c.doRequest("POST", path, nil, body, nil)
}

func (c *Client) GetSaveSummary(romID int) (SaveSummary, error) {
	var summary SaveSummary
	query := SaveSummaryQuery{RomID: romID}
	err := c.doRequest("GET", endpointSaveSummary, query, nil, &summary)
	return summary, err
}

// UpdateSave re-uploads a file to an existing save by ID (PUT /api/saves/{id}).
// This updates the save's content and updatedAt in place without creating a new record.
func (c *Client) UpdateSave(saveID int, savePath string) (Save, error) {
	file, err := os.Open(savePath)
	if err != nil {
		return Save{}, err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("saveFile", filepath.Base(savePath))
	if err != nil {
		return Save{}, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return Save{}, err
	}

	if err := writer.Close(); err != nil {
		return Save{}, err
	}

	path := fmt.Sprintf(endpointSaveByID, saveID)
	var res Save
	err = c.doMultipartRequest("PUT", path, nil, &buf, writer.FormDataContentType(), &res)
	if err != nil {
		return Save{}, err
	}

	return res, nil
}

func (c *Client) UploadSaveWithQuery(query UploadSaveQuery, savePath string) (Save, error) {
	file, err := os.Open(savePath)
	if err != nil {
		return Save{}, err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("saveFile", filepath.Base(savePath))
	if err != nil {
		return Save{}, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return Save{}, err
	}

	if err := writer.Close(); err != nil {
		return Save{}, err
	}

	var res Save
	err = c.doMultipartRequest("POST", endpointSaves, query, &buf, writer.FormDataContentType(), &res)
	if err != nil {
		return Save{}, err
	}

	return res, nil
}
