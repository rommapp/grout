package romm

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

type CollectionKind string

const (
	CollectionRegular CollectionKind = "regular"
	CollectionSmart   CollectionKind = "smart"
	CollectionVirtual CollectionKind = "virtual"
)

type Collection struct {
	Kind      CollectionKind `json:"-"`
	RemoteID  string         `json:"-"`
	Name      string         `json:"name"`
	RomIDs    []int          `json:"rom_ids"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type collectionWire struct {
	Collection
	ID json.RawMessage `json:"id"`
}

func collectionBase(kind CollectionKind) string {
	switch kind {
	case CollectionSmart:
		return "/api/collections/smart"
	case CollectionVirtual:
		return "/api/collections/virtual"
	}
	return "/api/collections"
}

func remoteID(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return "", err
	}
	return strconv.FormatInt(n, 10), nil
}

func (c *Client) Collections(ctx context.Context, kind CollectionKind, updatedAfter string) ([]Collection, error) {
	var query url.Values
	switch {
	case kind == CollectionVirtual:
		query = url.Values{"type": {"collection"}}
	case updatedAfter != "":
		query = url.Values{"updated_after": {updatedAfter}}
	}
	var wire []collectionWire
	if err := c.getJSON(ctx, collectionBase(kind), query, &wire); err != nil {
		return nil, err
	}
	out := make([]Collection, 0, len(wire))
	for _, w := range wire {
		id, err := remoteID(w.ID)
		if err != nil {
			return nil, err
		}
		col := w.Collection
		col.Kind = kind
		col.RemoteID = id
		out = append(out, col)
	}
	return out, nil
}

func (c *Client) CollectionIDs(ctx context.Context, kind CollectionKind) ([]string, error) {
	var raw []json.RawMessage
	if err := c.getJSON(ctx, collectionBase(kind)+"/identifiers", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		id, err := remoteID(r)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
