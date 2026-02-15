package cache

import (
	"database/sql"
	"encoding/json"
	"errors"
	"grout/romm"
	"strconv"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type Type string

const (
	Platform          Type = "platform"
	Collection        Type = "collection"
	SmartCollection   Type = "smart_collection"
	VirtualCollection Type = "virtual_collection"
)

func GetCacheKey(cacheType Type, id string) string {
	return string(cacheType) + "_" + id
}

func GetPlatformCacheKey(platformID int) string {
	return GetCacheKey(Platform, strconv.Itoa(platformID))
}

func GetCollectionCacheKey(collection romm.Collection) string {
	if collection.IsVirtual {
		return GetCacheKey(VirtualCollection, collection.VirtualID)
	}
	if collection.IsSmart {
		return GetCacheKey(SmartCollection, strconv.Itoa(collection.ID))
	}
	return GetCacheKey(Collection, strconv.Itoa(collection.ID))
}

func (cm *Manager) GetPlatformGames(platformID int) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT data_json FROM games WHERE platform_id = ? ORDER BY name
	`, platformID)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseMaxPlayerCount extracts the maximum player count from a ScreenScraper
// player_count string (e.g. "1", "2", "1-4") and the Rom's game-mode heuristic.
// Returns at least 1.
func parseMaxPlayerCount(game romm.Rom) int {
	pc := strings.TrimSpace(game.ScreenScraperMetadata.PlayerCount)
	if pc != "" {
		// Handle range like "1-4" — take the last number
		if idx := strings.LastIndex(pc, "-"); idx >= 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(pc[idx+1:])); err == nil && n > 0 {
				return n
			}
		}
		// Handle plain number
		if n, err := strconv.Atoi(pc); err == nil && n > 0 {
			return n
		}
	}
	// Fall back to the game-mode heuristic already on Rom
	return game.MaxPlayerCount()
}

// anySliceToStrings converts []any to []string, skipping non-string values.
func anySliceToStrings(items []any) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	return result
}

// resolveLookupID returns the integer ID for a value in a lookup table,
// inserting a new row if the value doesn't already exist.
func resolveLookupID(tx *sql.Tx, lookupTable, value string) (int64, error) {
	// Try INSERT OR IGNORE first, then SELECT — avoids the common SELECT-miss path
	_, err := tx.Exec("INSERT OR IGNORE INTO "+lookupTable+" (name) VALUES (?)", value)
	if err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRow("SELECT id FROM "+lookupTable+" WHERE name = ?", value).Scan(&id)
	return id, err
}

// batchInsertJunction resolves string values to lookup IDs and inserts (game_id, fk_id)
// rows into a junction table in batches within the given transaction.
func batchInsertJunction(tx *sql.Tx, junctionTable, fkCol, lookupTable string, gameID int, values []string) error {
	if len(values) == 0 {
		return nil
	}

	// Resolve all values to lookup IDs first
	ids := make([]int64, 0, len(values))
	for _, val := range values {
		id, err := resolveLookupID(tx, lookupTable, val)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}

	const batchSize = 400 // 2 params per row = 800 variables max
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		query := "INSERT OR IGNORE INTO " + junctionTable + " (game_id, " + fkCol + ") VALUES "
		args := make([]any, 0, len(batch)*2)
		for j, fkID := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?)"
			args = append(args, gameID, fkID)
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	return nil
}

func (cm *Manager) SavePlatformGames(platformID int, games []romm.Rom) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO games (
			id, platform_id, platform_fs_slug, name, fs_name, fs_name_no_ext,
			crc_hash, md5_hash, sha1_hash,
			player_count, first_release_date, average_rating, fs_size_bytes,
			is_identified, is_unidentified, missing_from_fs, has_manual, has_multiple_files,
			data_json, updated_at, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
	}
	defer stmt.Close()

	// Delete existing junction table data for this platform's games
	for _, table := range junctionTables {
		_, err := tx.Exec(
			"DELETE FROM "+table+" WHERE game_id IN (SELECT id FROM games WHERE platform_id = ?)",
			platformID,
		)
		if err != nil {
			return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
		}
	}

	now := nowUTC()
	cacheKey := GetPlatformCacheKey(platformID)

	for _, game := range games {
		dataJSON, err := json.Marshal(game)
		if err != nil {
			return newCacheError("save", "games", cacheKey, err)
		}

		_, err = stmt.Exec(
			game.ID,
			game.PlatformID,
			game.PlatformFSSlug,
			game.Name,
			game.FsName,
			game.FsNameNoExt,
			game.CrcHash,
			game.Md5Hash,
			game.Sha1Hash,
			parseMaxPlayerCount(game),
			game.Metadatum.FirstReleaseDate,
			game.Metadatum.AverageRating,
			game.FsSizeBytes,
			boolToInt(game.IsIdentified),
			boolToInt(game.IsUnidentified),
			boolToInt(game.MissingFromFs),
			boolToInt(game.HasManual),
			boolToInt(game.HasMultipleFiles),
			string(dataJSON),
			game.UpdatedAt,
			now,
		)
		if err != nil {
			return newCacheError("save", "games", cacheKey, err)
		}

		// Populate junction tables (junction, fk_col, lookup, values)
		junctions := []struct {
			junctionTable, fkCol, lookupTable string
			values                            []string
		}{
			{"game_genres", "genre_id", "genres", game.Metadatum.Genres},
			{"game_franchises", "franchise_id", "franchises", anySliceToStrings(game.Metadatum.Franchises)},
			{"game_companies", "company_id", "companies", game.Metadatum.Companies},
			{"game_game_modes", "game_mode_id", "game_modes", game.Metadatum.GameModes},
			{"game_age_ratings", "age_rating_id", "age_ratings", game.Metadatum.AgeRatings},
			{"game_regions", "region_id", "regions", game.Regions},
			{"game_languages", "language_id", "languages", game.Languages},
			{"game_tags", "tag_id", "tags", anySliceToStrings(game.Tags)},
		}
		for _, jt := range junctions {
			if err := batchInsertJunction(tx, jt.junctionTable, jt.fkCol, jt.lookupTable, game.ID, jt.values); err != nil {
				return newCacheError("save", "games", cacheKey, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "games", cacheKey, err)
	}

	return nil
}

func (cm *Manager) GetCollectionGames(collection romm.Collection) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	collectionID, err := cm.getCollectionInternalID(collection)
	if err != nil {
		return nil, err
	}

	rows, err := cm.db.Query(`
		SELECT g.data_json FROM games g
		INNER JOIN game_collections gc ON g.id = gc.game_id
		WHERE gc.collection_id = ?
		ORDER BY g.name
	`, collectionID)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

func (cm *Manager) SaveCollectionGames(collection romm.Collection, games []romm.Rom) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	if len(games) == 0 {
		return nil
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get the collection's internal ID (collection should already be saved)
	collectionID, err := cm.getCollectionInternalIDLocked(collection)
	if err != nil {
		return err
	}

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
	}
	defer tx.Rollback()

	// Delete existing game-collection mappings for this collection
	_, err = tx.Exec(`DELETE FROM game_collections WHERE collection_id = ?`, collectionID)
	if err != nil {
		return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
	}

	// Batch insert mappings for better performance
	// SQLite supports up to 999 variables, so batch in groups
	const batchSize = 400 // 2 params per row = 800 variables max
	for i := 0; i < len(games); i += batchSize {
		end := i + batchSize
		if end > len(games) {
			end = len(games)
		}
		batch := games[i:end]

		// Build batch insert query
		query := "INSERT OR IGNORE INTO game_collections (game_id, collection_id) VALUES "
		args := make([]interface{}, 0, len(batch)*2)
		for j, game := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?)"
			args = append(args, game.ID, collectionID)
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
	}

	logger.Debug("Saved collection game mappings", "collection", collection.Name, "count", len(games))
	return nil
}

func (cm *Manager) getCollectionInternalID(collection romm.Collection) (int64, error) {
	var id int64
	var err error

	if collection.IsVirtual {
		err = cm.db.QueryRow(`SELECT id FROM collections WHERE virtual_id = ?`, collection.VirtualID).Scan(&id)
	} else {
		collType := "regular"
		if collection.IsSmart {
			collType = "smart"
		}
		err = cm.db.QueryRow(`SELECT id FROM collections WHERE romm_id = ? AND type = ?`, collection.ID, collType).Scan(&id)
	}

	if errors.Is(err, sql.ErrNoRows) {
		cm.stats.recordMiss()
		return 0, ErrCacheMiss
	}
	if err != nil {
		cm.stats.recordError()
		return 0, newCacheError("get", "collections", GetCollectionCacheKey(collection), err)
	}

	return id, nil
}

func (cm *Manager) getCollectionInternalIDLocked(collection romm.Collection) (int64, error) {
	return cm.getCollectionInternalID(collection)
}

func (cm *Manager) ResolveCollectionID(collection romm.Collection) (int64, error) {
	if cm == nil || !cm.initialized {
		return 0, ErrNotInitialized
	}
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.getCollectionInternalID(collection)
}

func (cm *Manager) SaveAllCollectionMappings(collections []romm.Collection) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	if len(collections) == 0 {
		return nil
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "collection_mappings", "", err)
	}
	defer tx.Rollback()

	// Clear all existing mappings
	if _, err := tx.Exec(`DELETE FROM game_collections`); err != nil {
		return newCacheError("save", "collection_mappings", "", err)
	}

	// Build a map of collection identifiers to internal IDs
	collectionIDs := make(map[string]int64)
	for _, coll := range collections {
		var id int64
		var err error

		if coll.IsVirtual {
			err = tx.QueryRow(`SELECT id FROM collections WHERE virtual_id = ?`, coll.VirtualID).Scan(&id)
		} else {
			collType := "regular"
			if coll.IsSmart {
				collType = "smart"
			}
			err = tx.QueryRow(`SELECT id FROM collections WHERE romm_id = ? AND type = ?`, coll.ID, collType).Scan(&id)
		}

		if err == nil {
			collectionIDs[GetCollectionCacheKey(coll)] = id
		}
	}

	// Batch insert all mappings
	const batchSize = 400
	var allMappings []struct {
		gameID       int
		collectionID int64
	}

	for _, coll := range collections {
		collID, ok := collectionIDs[GetCollectionCacheKey(coll)]
		if !ok {
			continue
		}
		for _, romID := range coll.ROMIDs {
			allMappings = append(allMappings, struct {
				gameID       int
				collectionID int64
			}{romID, collID})
		}
	}

	for i := 0; i < len(allMappings); i += batchSize {
		end := i + batchSize
		if end > len(allMappings) {
			end = len(allMappings)
		}
		batch := allMappings[i:end]

		query := "INSERT OR IGNORE INTO game_collections (game_id, collection_id) VALUES "
		args := make([]interface{}, 0, len(batch)*2)
		for j, m := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?)"
			args = append(args, m.gameID, m.collectionID)
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return newCacheError("save", "collection_mappings", "", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "collection_mappings", "", err)
	}

	logger.Debug("Saved all collection mappings", "collections", len(collections), "mappings", len(allMappings))
	return nil
}

func (cm *Manager) GetGamesByIDs(gameIDs []int) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	if len(gameIDs) == 0 {
		return nil, nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Build query with placeholders
	placeholders := make([]string, len(gameIDs))
	args := make([]interface{}, len(gameIDs))
	for i, id := range gameIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT data_json FROM games WHERE id IN (" + strings.Join(placeholders, ",") + ") ORDER BY name"

	rows, err := cm.db.Query(query, args...)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", "batch", err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", "batch", err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", "batch", err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", "batch", err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

func (cm *Manager) GetCachedGameIDsForPlatforms(fsSlugs []string) map[int]bool {
	if cm == nil || !cm.initialized || len(fsSlugs) == 0 {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	placeholders := make([]byte, 0, len(fsSlugs)*2-1)
	args := make([]any, len(fsSlugs))
	for i, slug := range fsSlugs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args[i] = slug
	}

	rows, err := cm.db.Query(
		`SELECT id FROM games WHERE platform_fs_slug IN (`+string(placeholders)+`)`,
		args...,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	gameIDs := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			gameIDs[id] = true
		}
	}

	return gameIDs
}

// GameFilter defines criteria for filtering games at the database level.
// All non-zero/non-empty fields are ANDed together.
// Slice fields (e.g., Genres) use OR within the slice (match any of the given values).
type GameFilter struct {
	PlatformID           int
	PlatformSlugs        []string
	CollectionInternalID int64
	Genres               []string
	Franchises           []string
	Companies            []string
	GameModes            []string
	AgeRatings           []string
	Regions              []string
	Languages            []string
	Tags                 []string
	IsIdentified         *bool
	IsUnidentified       *bool
	MissingFromFs        *bool
	HasManual            *bool
	HasMultiple          *bool
	MinRating            float64
	MaxRating            float64
	MinReleaseDate       int64
	MaxReleaseDate       int64
	MinSizeBytes         int64
	MaxSizeBytes         int64
	NameSearch           string
}

// HasActiveFilters returns true if any filter criteria are set.
func (f GameFilter) HasActiveFilters() bool {
	return len(f.PlatformSlugs) > 0 || len(f.Genres) > 0 || len(f.Franchises) > 0 || len(f.Companies) > 0 ||
		len(f.GameModes) > 0 || len(f.AgeRatings) > 0 || len(f.Regions) > 0 ||
		len(f.Languages) > 0 || len(f.Tags) > 0 ||
		f.IsIdentified != nil || f.IsUnidentified != nil || f.MissingFromFs != nil ||
		f.HasManual != nil || f.HasMultiple != nil ||
		f.MinRating > 0 || f.MaxRating > 0 ||
		f.MinReleaseDate > 0 || f.MaxReleaseDate > 0 ||
		f.MinSizeBytes > 0 || f.MaxSizeBytes > 0
}

// GetFilteredGames returns games matching all the given filter criteria.
// Results are always deserialized from data_json for full Rom objects.
func (cm *Manager) GetFilteredGames(filter GameFilter) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	query := "SELECT g.data_json FROM games g WHERE 1=1"
	var args []interface{}

	if filter.CollectionInternalID != 0 {
		query += " AND EXISTS (SELECT 1 FROM game_collections gc WHERE gc.game_id = g.id AND gc.collection_id = ?)"
		args = append(args, filter.CollectionInternalID)
	}

	if filter.PlatformID != 0 {
		query += " AND g.platform_id = ?"
		args = append(args, filter.PlatformID)
	}

	if len(filter.PlatformSlugs) > 0 {
		placeholders := make([]string, len(filter.PlatformSlugs))
		for i, s := range filter.PlatformSlugs {
			placeholders[i] = "?"
			args = append(args, s)
		}
		query += " AND g.platform_fs_slug IN (" + strings.Join(placeholders, ",") + ")"
	}

	if filter.NameSearch != "" {
		query += " AND g.name LIKE ?"
		args = append(args, "%"+filter.NameSearch+"%")
	}

	// Scalar boolean filters
	if filter.IsIdentified != nil {
		query += " AND g.is_identified = ?"
		args = append(args, boolToInt(*filter.IsIdentified))
	}
	if filter.IsUnidentified != nil {
		query += " AND g.is_unidentified = ?"
		args = append(args, boolToInt(*filter.IsUnidentified))
	}
	if filter.MissingFromFs != nil {
		query += " AND g.missing_from_fs = ?"
		args = append(args, boolToInt(*filter.MissingFromFs))
	}
	if filter.HasManual != nil {
		query += " AND g.has_manual = ?"
		args = append(args, boolToInt(*filter.HasManual))
	}
	if filter.HasMultiple != nil {
		query += " AND g.has_multiple_files = ?"
		args = append(args, boolToInt(*filter.HasMultiple))
	}

	// Scalar range filters
	if filter.MinRating > 0 {
		query += " AND g.average_rating >= ?"
		args = append(args, filter.MinRating)
	}
	if filter.MaxRating > 0 {
		query += " AND g.average_rating <= ?"
		args = append(args, filter.MaxRating)
	}
	if filter.MinReleaseDate > 0 {
		query += " AND g.first_release_date >= ?"
		args = append(args, filter.MinReleaseDate)
	}
	if filter.MaxReleaseDate > 0 {
		query += " AND g.first_release_date <= ?"
		args = append(args, filter.MaxReleaseDate)
	}
	if filter.MinSizeBytes > 0 {
		query += " AND g.fs_size_bytes >= ?"
		args = append(args, filter.MinSizeBytes)
	}
	if filter.MaxSizeBytes > 0 {
		query += " AND g.fs_size_bytes <= ?"
		args = append(args, filter.MaxSizeBytes)
	}

	// Junction table filters using EXISTS subqueries through normalized lookup tables
	type junctionFilter struct {
		junctionTable, fkCol, lookupTable string
		values                            []string
	}
	junctions := []junctionFilter{
		{"game_genres", "genre_id", "genres", filter.Genres},
		{"game_franchises", "franchise_id", "franchises", filter.Franchises},
		{"game_companies", "company_id", "companies", filter.Companies},
		{"game_game_modes", "game_mode_id", "game_modes", filter.GameModes},
		{"game_age_ratings", "age_rating_id", "age_ratings", filter.AgeRatings},
		{"game_regions", "region_id", "regions", filter.Regions},
		{"game_languages", "language_id", "languages", filter.Languages},
		{"game_tags", "tag_id", "tags", filter.Tags},
	}

	for _, jf := range junctions {
		if len(jf.values) == 0 {
			continue
		}
		placeholders := make([]string, len(jf.values))
		for i, v := range jf.values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		query += " AND EXISTS (SELECT 1 FROM " + jf.junctionTable + " jt INNER JOIN " + jf.lookupTable + " lt ON lt.id = jt." + jf.fkCol + " WHERE jt.game_id = g.id AND lt.name IN (" + strings.Join(placeholders, ",") + "))"
	}

	query += " ORDER BY g.name"

	rows, err := cm.db.Query(query, args...)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", "filtered", err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", "filtered", err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", "filtered", err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", "filtered", err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

// GetDistinctValues returns all distinct names from a lookup table, optionally
// filtered to only include values that are associated with games on the given platform.
func (cm *Manager) GetDistinctValues(lookupTable, junctionTable, fkCol string, platformID int) ([]string, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var query string
	var args []any

	if platformID != 0 {
		query = "SELECT DISTINCT lt.name FROM " + lookupTable + " lt INNER JOIN " + junctionTable + " jt ON jt." + fkCol + " = lt.id INNER JOIN games g ON g.id = jt.game_id WHERE g.platform_id = ? ORDER BY lt.name"
		args = append(args, platformID)
	} else {
		query = "SELECT lt.name FROM " + lookupTable + " lt WHERE EXISTS (SELECT 1 FROM " + junctionTable + " jt WHERE jt." + fkCol + " = lt.id) ORDER BY lt.name"
	}

	rows, err := cm.db.Query(query, args...)
	if err != nil {
		return nil, newCacheError("get", lookupTable, "distinct", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, newCacheError("get", lookupTable, "distinct", err)
		}
		values = append(values, v)
	}

	return values, rows.Err()
}

func (cm *Manager) GetDistinctValuesWithFilter(lookupTable, junctionTable, fkCol string, platformID int, filter GameFilter) ([]string, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	query := "SELECT DISTINCT lt.name FROM " + lookupTable + " lt INNER JOIN " + junctionTable + " jt ON jt." + fkCol + " = lt.id INNER JOIN games g ON g.id = jt.game_id WHERE 1=1"
	var args []any

	if filter.CollectionInternalID != 0 {
		query += " AND EXISTS (SELECT 1 FROM game_collections gc WHERE gc.game_id = g.id AND gc.collection_id = ?)"
		args = append(args, filter.CollectionInternalID)
	}

	if platformID != 0 {
		query += " AND g.platform_id = ?"
		args = append(args, platformID)
	}

	if len(filter.PlatformSlugs) > 0 {
		placeholders := make([]string, len(filter.PlatformSlugs))
		for i, s := range filter.PlatformSlugs {
			placeholders[i] = "?"
			args = append(args, s)
		}
		query += " AND g.platform_fs_slug IN (" + strings.Join(placeholders, ",") + ")"
	}

	if filter.NameSearch != "" {
		query += " AND g.name LIKE ?"
		args = append(args, "%"+filter.NameSearch+"%")
	}

	type junctionFilter struct {
		junctionTable, fkCol, lookupTable string
		values                            []string
	}
	junctions := []junctionFilter{
		{"game_genres", "genre_id", "genres", filter.Genres},
		{"game_franchises", "franchise_id", "franchises", filter.Franchises},
		{"game_companies", "company_id", "companies", filter.Companies},
		{"game_game_modes", "game_mode_id", "game_modes", filter.GameModes},
		{"game_age_ratings", "age_rating_id", "age_ratings", filter.AgeRatings},
		{"game_regions", "region_id", "regions", filter.Regions},
		{"game_languages", "language_id", "languages", filter.Languages},
		{"game_tags", "tag_id", "tags", filter.Tags},
	}

	for _, jf := range junctions {
		if len(jf.values) == 0 {
			continue
		}
		placeholders := make([]string, len(jf.values))
		for i, v := range jf.values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		query += " AND EXISTS (SELECT 1 FROM " + jf.junctionTable + " jt2 INNER JOIN " + jf.lookupTable + " lt2 ON lt2.id = jt2." + jf.fkCol + " WHERE jt2.game_id = g.id AND lt2.name IN (" + strings.Join(placeholders, ",") + "))"
	}

	query += " ORDER BY lt.name"

	rows, err := cm.db.Query(query, args...)
	if err != nil {
		return nil, newCacheError("get", lookupTable, "distinct-filtered", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, newCacheError("get", lookupTable, "distinct-filtered", err)
		}
		values = append(values, v)
	}

	return values, rows.Err()
}

type PlatformOption struct {
	Slug        string
	DisplayName string
}

func (cm *Manager) GetCollectionPlatforms(collection romm.Collection, filter GameFilter) ([]PlatformOption, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	collectionID, err := cm.getCollectionInternalID(collection)
	if err != nil {
		return nil, err
	}

	query := `SELECT DISTINCT p.fs_slug, p.name, p.custom_name FROM games g
		INNER JOIN game_collections gc ON g.id = gc.game_id
		INNER JOIN platforms p ON p.id = g.platform_id
		WHERE gc.collection_id = ?`
	args := []any{collectionID}

	if filter.NameSearch != "" {
		query += " AND g.name LIKE ?"
		args = append(args, "%"+filter.NameSearch+"%")
	}

	type junctionFilter struct {
		junctionTable, fkCol, lookupTable string
		values                            []string
	}
	junctions := []junctionFilter{
		{"game_genres", "genre_id", "genres", filter.Genres},
		{"game_franchises", "franchise_id", "franchises", filter.Franchises},
		{"game_companies", "company_id", "companies", filter.Companies},
		{"game_game_modes", "game_mode_id", "game_modes", filter.GameModes},
		{"game_age_ratings", "age_rating_id", "age_ratings", filter.AgeRatings},
		{"game_regions", "region_id", "regions", filter.Regions},
		{"game_languages", "language_id", "languages", filter.Languages},
		{"game_tags", "tag_id", "tags", filter.Tags},
	}

	for _, jf := range junctions {
		if len(jf.values) == 0 {
			continue
		}
		placeholders := make([]string, len(jf.values))
		for i, v := range jf.values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		query += " AND EXISTS (SELECT 1 FROM " + jf.junctionTable + " jt INNER JOIN " + jf.lookupTable + " lt ON lt.id = jt." + jf.fkCol + " WHERE jt.game_id = g.id AND lt.name IN (" + strings.Join(placeholders, ",") + "))"
	}

	query += " ORDER BY p.name"

	rows, err := cm.db.Query(query, args...)
	if err != nil {
		return nil, newCacheError("get", "games", "collection-platforms", err)
	}
	defer rows.Close()

	var platforms []PlatformOption
	for rows.Next() {
		var slug, name, customName string
		if err := rows.Scan(&slug, &name, &customName); err != nil {
			return nil, newCacheError("get", "games", "collection-platforms", err)
		}
		displayName := name
		if customName != "" {
			displayName = customName
		}
		platforms = append(platforms, PlatformOption{Slug: slug, DisplayName: displayName})
	}

	return platforms, rows.Err()
}

func (cm *Manager) GetDistinctGenres(platformID int) ([]string, error) {
	return cm.GetDistinctValues("genres", "game_genres", "genre_id", platformID)
}

func (cm *Manager) GetDistinctFranchises(platformID int) ([]string, error) {
	return cm.GetDistinctValues("franchises", "game_franchises", "franchise_id", platformID)
}

func (cm *Manager) GetDistinctCompanies(platformID int) ([]string, error) {
	return cm.GetDistinctValues("companies", "game_companies", "company_id", platformID)
}

func (cm *Manager) GetDistinctGameModes(platformID int) ([]string, error) {
	return cm.GetDistinctValues("game_modes", "game_game_modes", "game_mode_id", platformID)
}

func (cm *Manager) GetDistinctAgeRatings(platformID int) ([]string, error) {
	return cm.GetDistinctValues("age_ratings", "game_age_ratings", "age_rating_id", platformID)
}

func (cm *Manager) GetDistinctRegions(platformID int) ([]string, error) {
	return cm.GetDistinctValues("regions", "game_regions", "region_id", platformID)
}

func (cm *Manager) GetDistinctLanguages(platformID int) ([]string, error) {
	return cm.GetDistinctValues("languages", "game_languages", "language_id", platformID)
}

func (cm *Manager) GetDistinctTags(platformID int) ([]string, error) {
	return cm.GetDistinctValues("tags", "game_tags", "tag_id", platformID)
}

// PurgeDeletedGames removes cached games whose IDs are not in the provided list
// of valid IDs from the server. Also cleans up related junction tables
// and game_collections for the deleted games.
func (cm *Manager) PurgeDeletedGames(validIDs []int) (int64, error) {
	if cm == nil || !cm.initialized {
		return 0, ErrNotInitialized
	}
	if len(validIDs) == 0 {
		return 0, nil
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return 0, newCacheError("purge", "games", "", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("CREATE TEMP TABLE _valid_game_ids (id INTEGER PRIMARY KEY)"); err != nil {
		return 0, newCacheError("purge", "games", "", err)
	}

	const batchSize = 400
	for i := 0; i < len(validIDs); i += batchSize {
		end := i + batchSize
		if end > len(validIDs) {
			end = len(validIDs)
		}
		batch := validIDs[i:end]

		query := "INSERT OR IGNORE INTO _valid_game_ids (id) VALUES "
		args := make([]any, len(batch))
		for j, id := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?)"
			args[j] = id
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return 0, newCacheError("purge", "games", "", err)
		}
	}

	// Clean up junction tables for deleted games
	for _, table := range junctionTables {
		if _, err := tx.Exec("DELETE FROM " + table + " WHERE game_id NOT IN (SELECT id FROM _valid_game_ids)"); err != nil {
			return 0, newCacheError("purge", "games", table, err)
		}
	}

	if _, err := tx.Exec("DELETE FROM game_collections WHERE game_id NOT IN (SELECT id FROM _valid_game_ids)"); err != nil {
		return 0, newCacheError("purge", "games", "game_collections", err)
	}

	result, err := tx.Exec("DELETE FROM games WHERE id NOT IN (SELECT id FROM _valid_game_ids)")
	if err != nil {
		return 0, newCacheError("purge", "games", "", err)
	}

	deleted, _ := result.RowsAffected()

	tx.Exec("DROP TABLE IF EXISTS _valid_game_ids")

	if err := tx.Commit(); err != nil {
		return 0, newCacheError("purge", "games", "", err)
	}

	if deleted > 0 {
		logger.Debug("Purged deleted games from cache", "count", deleted)
	}

	return deleted, nil
}
