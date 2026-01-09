package romm

import (
	"fmt"
	"time"
)

type Firmware struct {
	ID             int       `json:"id"`
	FileName       string    `json:"file_name"`
	FileNameNoTags string    `json:"file_name_no_tags"`
	FileNameNoExt  string    `json:"file_name_no_ext"`
	FileExtension  string    `json:"file_extension"`
	FilePath       string    `json:"file_path"`
	FileSizeBytes  int64     `json:"file_size_bytes"`
	FullPath       string    `json:"full_path"`
	IsVerified     bool      `json:"is_verified"`
	CRCHash        string    `json:"crc_hash"`
	MD5Hash        string    `json:"md5_hash"`
	SHA1Hash       string    `json:"sha1_hash"`
	MissingFromFS  bool      `json:"missing_from_fs"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	DownloadURL string `json:"-"`
}

type FirmwareOptions struct {
	PlatformID int `qs:"platform_id"`
}

func (fo FirmwareOptions) Valid() bool {
	return fo.PlatformID != 0
}

func (c *Client) GetFirmware(platformID int) ([]Firmware, error) {
	var firmware []Firmware
	err := c.doRequest("GET", endpointFirmware, FirmwareOptions{PlatformID: platformID}, nil, &firmware)
	if err != nil {
		return nil, err
	}

	// Construct download URLs since the API doesn't provide them
	// Format: /api/firmware/{id}/content/{filename}
	for i := range firmware {
		firmware[i].DownloadURL = fmt.Sprintf("/api/firmware/%d/content/%s", firmware[i].ID, firmware[i].FileName)
	}

	return firmware, nil
}
