package romm

import (
	"fmt"
	"time"
)

// ClientSaveState represents the state of a single save file on the client device.
type ClientSaveState struct {
	RomID         int       `json:"rom_id"`
	FileName      string    `json:"file_name"`
	Slot          string    `json:"slot,omitempty"`
	Emulator      string    `json:"emulator,omitempty"`
	ContentHash   string    `json:"content_hash,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	FileSizeBytes int64     `json:"file_size_bytes"`
}

// SyncNegotiatePayload is the request body for POST /api/sync/negotiate.
type SyncNegotiatePayload struct {
	DeviceID string            `json:"device_id"`
	Saves    []ClientSaveState `json:"saves"`
}

// SyncOperationSchema describes a single sync operation returned by the server.
type SyncOperationSchema struct {
	Action            string     `json:"action"` // "upload", "download", "conflict", "no_op"
	RomID             int        `json:"rom_id"`
	SaveID            *int       `json:"save_id"` // nil for new uploads
	FileName          string     `json:"file_name"`
	Slot              *string    `json:"slot,omitempty"`
	Emulator          string     `json:"emulator,omitempty"`
	Reason            string     `json:"reason"`
	ServerUpdatedAt   *time.Time `json:"server_updated_at,omitempty"`
	ServerContentHash *string    `json:"server_content_hash,omitempty"`
}

// SyncNegotiateResponse is the response from POST /api/sync/negotiate.
type SyncNegotiateResponse struct {
	SessionID     int                   `json:"session_id"`
	Operations    []SyncOperationSchema `json:"operations"`
	TotalUpload   int                   `json:"total_upload"`
	TotalDownload int                   `json:"total_download"`
	TotalConflict int                   `json:"total_conflict"`
	TotalNoOp     int                   `json:"total_no_op"`
}

// SyncCompletePayload is the request body for POST /api/sync/sessions/{id}/complete.
type SyncCompletePayload struct {
	OperationsCompleted int `json:"operations_completed"`
	OperationsFailed    int `json:"operations_failed"`
}

// SyncSessionSchema is the response from sync session endpoints.
type SyncSessionSchema struct {
	ID                  int        `json:"id"`
	DeviceID            string     `json:"device_id"`
	UserID              int        `json:"user_id"`
	Status              string     `json:"status"`
	InitiatedAt         time.Time  `json:"initiated_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	OperationsPlanned   int        `json:"operations_planned"`
	OperationsCompleted int        `json:"operations_completed"`
	OperationsFailed    int        `json:"operations_failed"`
	ErrorMessage        *string    `json:"error_message,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Negotiate sends the client's save state to the server and receives sync operations.
func (c *Client) Negotiate(payload SyncNegotiatePayload) (SyncNegotiateResponse, error) {
	var resp SyncNegotiateResponse
	err := c.doRequest("POST", endpointSyncNegotiate, nil, payload, &resp)
	return resp, err
}

// CompleteSession marks a sync session as completed.
func (c *Client) CompleteSession(sessionID int, payload SyncCompletePayload) error {
	path := fmt.Sprintf(endpointSyncSessionComplete, sessionID)
	return c.doRequest("POST", path, nil, payload, nil)
}
