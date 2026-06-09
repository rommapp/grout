package romm

import (
	"fmt"
	"time"
)

// ClientSaveState is one local save reported to the orchestrator during negotiate.
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

// SyncOperationSchema is one operation the orchestrator wants the client to perform.
type SyncOperationSchema struct {
	Action            string     `json:"action"` // upload | download | conflict | no_op
	RomID             int        `json:"rom_id"`
	SaveID            *int       `json:"save_id"`
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
// grout does not track playtime, so play_sessions is omitted.
type SyncCompletePayload struct {
	OperationsCompleted int `json:"operations_completed"`
	OperationsFailed    int `json:"operations_failed"`
}

// SyncSessionSchema describes a sync session (returned by complete).
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

// SyncCompleteResponse is the response from the complete endpoint. grout ignores
// the play-session ingest result; only the session is modeled.
type SyncCompleteResponse struct {
	Session SyncSessionSchema `json:"session"`
}

// Negotiate sends the client's local save states and receives the sync plan.
func (c *Client) Negotiate(payload SyncNegotiatePayload) (SyncNegotiateResponse, error) {
	var resp SyncNegotiateResponse
	err := c.doRequest("POST", endpointSyncNegotiate, nil, payload, &resp)
	return resp, err
}

// CompleteSession marks a sync session complete with the executed counts.
func (c *Client) CompleteSession(sessionID int, payload SyncCompletePayload) error {
	path := fmt.Sprintf(endpointSyncSessionComplete, sessionID)
	return c.doRequest("POST", path, nil, payload, nil)
}
