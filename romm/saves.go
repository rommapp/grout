package romm

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

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
	} `json:"Screenshot"`

	// New fields for device-aware saves
	Slot        *string          `json:"slot,omitempty"`
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
	return sq.RomID != 0 || sq.PlatformID != 0
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
	DeviceID   string `qs:"device_id,omitempty"`
	Optimistic bool   `qs:"optimistic,omitempty"`
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

// MarkDeviceSynced confirms this device has the latest save state.
// Used after both uploads and downloads.
func (c *Client) MarkDeviceSynced(saveID int, deviceID string) error {
	return c.ConfirmSaveDownloaded(saveID, deviceID)
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

func (c *Client) UploadSave(romID int, savePath string, emulator string) (Save, error) {
	return c.UploadSaveWithQuery(UploadSaveQuery{
		RomID:    romID,
		Emulator: emulator,
	}, savePath)
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
