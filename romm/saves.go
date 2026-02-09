package romm

import (
	"bytes"
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
}

type SaveQuery struct {
	RomID      int    `qs:"rom_id"`
	Emulator   string `qs:"emulator"`
	PlatformID int    `qs:"platform_id"`
}

func (sq SaveQuery) Valid() bool {
	return sq.RomID != 0 || sq.PlatformID != 0
}

func (c *Client) GetSaves(query SaveQuery) ([]Save, error) {
	var saves []Save
	err := c.doRequest("GET", endpointSaves, query, nil, &saves)
	return saves, err
}

func (c *Client) DownloadSave(downloadPath string) ([]byte, error) {
	return c.doRequestRaw("GET", downloadPath, nil)
}

func (c *Client) UploadSave(romID int, savePath string, emulator string) (Save, error) {
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
	err = c.doMultipartRequest("POST", endpointSaves, SaveQuery{RomID: romID, Emulator: emulator}, &buf, writer.FormDataContentType(), &res)
	if err != nil {
		return Save{}, err
	}

	return res, nil
}
