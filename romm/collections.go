package romm

import (
	"fmt"
	"time"
)

type Collection struct {
	ID              int       `json:"id"`
	VirtualID       string    `json:"virtual_id,omitempty"`
	IsVirtual       bool      `json:"is_virtual"`
	IsSmart         bool      `json:"is_smart"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	URLCover        string    `json:"url_cover"`
	PathCoverSmall  string    `json:"path_cover_small"`
	PathCoverLarge  string    `json:"path_cover_large"`
	PathCoversSmall []string  `json:"path_covers_small"`
	PathCoversLarge []string  `json:"path_covers_large"`
	IsPublic        bool      `json:"is_public"`
	IsFavorite      bool      `json:"is_favorite"`
	UserID          int       `json:"user_id"`
	ROMIDs          []int     `json:"rom_ids"`
	ROMCount        int       `json:"rom_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type VirtualCollection struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	URLCover        string    `json:"url_cover"`
	PathCoverSmall  string    `json:"path_cover_small"`
	PathCoverLarge  string    `json:"path_cover_large"`
	PathCoversSmall []string  `json:"path_covers_small"`
	PathCoversLarge []string  `json:"path_covers_large"`
	IsPublic        bool      `json:"is_public"`
	IsFavorite      bool      `json:"is_favorite"`
	UserID          int       `json:"user_id"`
	ROMIDs          []int     `json:"rom_ids"`
	ROMCount        int       `json:"rom_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type VirtualCollectionsQuery struct {
	Type string `qs:"type"`
}

func (v VirtualCollectionsQuery) Valid() bool {
	return v.Type != ""
}

func (c *Client) GetCollections() ([]Collection, error) {
	var collections []Collection
	err := c.doRequest("GET", endpointCollections, nil, nil, &collections)
	return collections, err
}

func (c *Client) GetCollection(id int) (Collection, error) {
	var collection Collection
	path := fmt.Sprintf(endpointCollectionByID, id)
	err := c.doRequest("GET", path, nil, nil, &collection)
	return collection, err
}

func (c *Client) GetSmartCollections() ([]Collection, error) {
	var collections []Collection
	err := c.doRequest("GET", endpointSmartCollections, nil, nil, &collections)
	return collections, err
}

func (c *Client) GetVirtualCollections() ([]VirtualCollection, error) {
	var collections []VirtualCollection
	err := c.doRequest("GET", endpointVirtualCollections, VirtualCollectionsQuery{Type: "collection"}, nil, &collections)
	return collections, err
}

// ToCollection converts a VirtualCollection to a Collection for unified handling
func (vc VirtualCollection) ToCollection() Collection {
	return Collection{
		ID:              0, // Virtual collections don't have int IDs
		VirtualID:       vc.ID,
		IsVirtual:       true,
		Name:            vc.Name,
		Description:     vc.Description,
		URLCover:        vc.URLCover,
		PathCoverSmall:  vc.PathCoverSmall,
		PathCoverLarge:  vc.PathCoverLarge,
		PathCoversSmall: vc.PathCoversSmall,
		PathCoversLarge: vc.PathCoversLarge,
		IsPublic:        vc.IsPublic,
		IsFavorite:      vc.IsFavorite,
		UserID:          vc.UserID,
		ROMIDs:          vc.ROMIDs,
		ROMCount:        vc.ROMCount,
		CreatedAt:       vc.CreatedAt,
		UpdatedAt:       vc.UpdatedAt,
	}
}
