package fakeromm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rommapp/grout/core/romm"
)

type Call struct {
	Method string
	Path   string
	Query  string
	Auth   string
}

type TokenReply struct {
	State romm.PairState
	Token romm.DeviceToken
}

type DeviceRequest struct {
	Method   string
	DeviceID string
	Body     map[string]any
}

type Collection struct {
	ID        string
	Name      string
	RomIDs    []int
	UpdatedAt time.Time
}

type Server struct {
	srv    *httptest.Server
	Client *romm.Client

	mu          sync.Mutex
	Platforms   []romm.Platform
	Roms        []romm.Rom
	Collections map[romm.CollectionKind][]Collection
	Hidden      map[int]bool
	OmitTotal   bool

	Content         map[int][]byte
	FirmwareList    []romm.Firmware
	FirmwareContent map[int][]byte
	Details         map[int]romm.RomDetail
	Assets          map[string][]byte
	contentDelay    time.Duration
	declaredLength  map[int]int

	calls      []Call
	pageCounts map[int]int

	failAfter    map[int]int
	failPaths    map[string]bool
	failOnce     map[string]int
	pathHooks    map[string]func()
	beforePage   func(platformID, offset int)
	Clock        time.Time
	version      string
	snapshots    bool
	unauthorized bool

	deviceInit     romm.DeviceInit
	initRequests   []romm.DeviceInitRequest
	tokenReplies   []TokenReply
	tokenHook      func()
	exchangeCodes  map[string]romm.ClientToken
	exchangeStatus int
	username       string
	rejected       map[string]bool
	deviceRequests []DeviceRequest
	nextDevice     int
}

func New(t *testing.T) *Server {
	t.Helper()
	f := &Server{
		Collections: map[romm.CollectionKind][]Collection{},
		Hidden:      map[int]bool{},
		pageCounts:  map[int]int{},
		failAfter:   map[int]int{},
		failPaths:   map[string]bool{},
		failOnce:    map[string]int{},
		pathHooks:   map[string]func(){},
		version:     "5.0.0",
		snapshots:   true,
		deviceInit: romm.DeviceInit{
			DeviceCode:               "device-code",
			UserCode:                 "WXYZ-2345",
			VerificationPath:         "/pair",
			VerificationPathComplete: "/pair?code=WXYZ-2345",
			ExpiresIn:                900,
			Interval:                 5,
		},
		exchangeCodes: map[string]romm.ClientToken{},
		rejected:      map[string]bool{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	f.Client = romm.New(romm.Options{BaseURL: f.srv.URL})
	return f
}

func (f *Server) URL() string {
	return f.srv.URL
}

func (f *Server) Close() {
	f.srv.Close()
}

func (f *Server) Mutate(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *Server) SetContentDelay(d time.Duration) {
	f.Mutate(func() { f.contentDelay = d })
}

func (f *Server) SetContentLength(romID, n int) {
	f.Mutate(func() {
		if f.declaredLength == nil {
			f.declaredLength = map[int]int{}
		}
		f.declaredLength[romID] = n
	})
}

func (f *Server) SetFailAfter(platformID, n int) {
	f.Mutate(func() { f.failAfter[platformID] = n })
}

func (f *Server) ClearFailAfter(platformID int) {
	f.Mutate(func() { delete(f.failAfter, platformID) })
}

func (f *Server) FailPath(path string) {
	f.Mutate(func() { f.failPaths[path] = true })
}

func (f *Server) SetPathHook(path string, fn func()) {
	f.Mutate(func() { f.pathHooks[path] = fn })
}

func (f *Server) FailTimes(path string, n int) {
	f.Mutate(func() { f.failOnce[path] = n })
}

func (f *Server) SetBeforePage(fn func(platformID, offset int)) {
	f.Mutate(func() { f.beforePage = fn })
}

func (f *Server) SetClock(t time.Time) {
	f.Mutate(func() { f.Clock = t })
}

func (f *Server) SetHeartbeat(version string, snapshots bool) {
	f.Mutate(func() {
		f.version = version
		f.snapshots = snapshots
	})
}

func (f *Server) SetUnauthorized(v bool) {
	f.Mutate(func() { f.unauthorized = v })
}

func (f *Server) SetDeviceInit(d romm.DeviceInit) {
	f.Mutate(func() { f.deviceInit = d })
}

func (f *Server) InitRequests() []romm.DeviceInitRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]romm.DeviceInitRequest(nil), f.initRequests...)
}

func (f *Server) SetTokenReplies(replies ...TokenReply) {
	f.Mutate(func() { f.tokenReplies = append([]TokenReply(nil), replies...) })
}

func (f *Server) SetTokenHook(fn func()) {
	f.Mutate(func() { f.tokenHook = fn })
}

func (f *Server) AddExchangeCode(code string, tok romm.ClientToken) {
	f.Mutate(func() { f.exchangeCodes[code] = tok })
}

func (f *Server) SetExchangeStatus(code int) {
	f.Mutate(func() { f.exchangeStatus = code })
}

func (f *Server) SetUsername(name string) {
	f.Mutate(func() { f.username = name })
}

func (f *Server) RejectToken(token string) {
	f.Mutate(func() { f.rejected["Bearer "+token] = true })
}

func (f *Server) DeviceRequests() []DeviceRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DeviceRequest(nil), f.deviceRequests...)
}

func (f *Server) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

func (f *Server) CallsTo(path string) []Call {
	var out []Call
	for _, c := range f.Calls() {
		if c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

func (f *Server) RomPages(platformID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pageCounts[platformID]
}

func Roms(platformID, firstID, n int, at time.Time) []romm.Rom {
	out := make([]romm.Rom, 0, n)
	for i := 0; i < n; i++ {
		id := firstID + i
		out = append(out, romm.Rom{ID: id, PlatformID: platformID, Name: fmt.Sprintf("game %d", id), UpdatedAt: at})
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func parseAfter(q string) (time.Time, bool) {
	if q == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, q)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (f *Server) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	auth := r.Header.Get("Authorization")
	f.calls = append(f.calls, Call{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Auth: auth})
	forced := f.failPaths[r.URL.Path]
	if f.failOnce[r.URL.Path] > 0 {
		f.failOnce[r.URL.Path]--
		forced = true
	}
	clock := f.Clock
	unauthorized := f.unauthorized || f.rejected[auth]
	hook := f.pathHooks[r.URL.Path]
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if !clock.IsZero() {
		w.Header().Set("Date", clock.UTC().Format(http.TimeFormat))
	}
	if unauthorized {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if forced {
		http.Error(w, "forced failure", http.StatusInternalServerError)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/devices/") && r.Method == http.MethodPut {
		f.serveDeviceUpdate(w, r, strings.TrimPrefix(r.URL.Path, "/api/devices/"))
		return
	}
	f.mu.Lock()
	asset, isAsset := f.Assets[r.URL.Path]
	f.mu.Unlock()
	if isAsset {
		w.Header().Set("Content-Length", strconv.Itoa(len(asset)))
		_, _ = w.Write(asset)
		return
	}
	q := r.URL.Query()
	if id, rest, ok := splitContentPath(r.URL.Path, "/api/roms/"); ok && rest != "" {
		f.serveContent(w, r, id, rest, false)
		return
	}
	if id, rest, ok := splitContentPath(r.URL.Path, "/api/firmware/"); ok && rest != "" {
		f.serveContent(w, r, id, rest, true)
		return
	}
	if rest, ok := strings.CutPrefix(r.URL.Path, "/api/roms/"); ok {
		if id, err := strconv.Atoi(rest); err == nil {
			f.serveDetail(w, r, id)
			return
		}
	}
	switch r.URL.Path {
	case "/api/auth/device/init":
		f.serveDeviceInit(w, r)
	case "/api/auth/device/token":
		f.serveDeviceToken(w)
	case "/api/client-tokens/exchange":
		f.serveExchange(w, r)
	case "/api/users/me":
		f.mu.Lock()
		name := f.username
		f.mu.Unlock()
		writeJSON(w, map[string]any{"username": name})
	case "/api/devices":
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		f.serveDeviceRegister(w, r)
	case "/api/heartbeat":
		f.mu.Lock()
		body := map[string]any{
			"SYSTEM":    map[string]any{"VERSION": f.version},
			"SAVE_SYNC": map[string]any{"SNAPSHOTS": f.snapshots},
		}
		f.mu.Unlock()
		writeJSON(w, body)
	case "/api/platforms":
		f.mu.Lock()
		out := append([]romm.Platform{}, f.Platforms...)
		f.mu.Unlock()
		writeJSON(w, out)
	case "/api/platforms/identifiers":
		f.mu.Lock()
		out := []int{}
		for _, p := range f.Platforms {
			out = append(out, p.ID)
		}
		f.mu.Unlock()
		writeJSON(w, out)
	case "/api/firmware":
		platformID, _ := strconv.Atoi(q.Get("platform_id"))
		f.mu.Lock()
		out := []romm.Firmware{}
		for _, fw := range f.FirmwareList {
			if fw.PlatformID == platformID {
				out = append(out, fw)
			}
		}
		f.mu.Unlock()
		writeJSON(w, out)
	case "/api/roms":
		f.serveRoms(w, q.Get("platform_ids"), q.Get("updated_after"), q.Get("limit"), q.Get("offset"), q.Get("with_total") == "true")
	case "/api/roms/identifiers":
		f.mu.Lock()
		out := []int{}
		for _, rom := range f.Roms {
			if !f.Hidden[rom.ID] {
				out = append(out, rom.ID)
			}
		}
		f.mu.Unlock()
		sort.Ints(out)
		writeJSON(w, out)
	case "/api/collections":
		f.serveCollections(w, romm.CollectionRegular, q.Get("updated_after"))
	case "/api/collections/smart":
		f.serveCollections(w, romm.CollectionSmart, q.Get("updated_after"))
	case "/api/collections/virtual":
		f.serveCollections(w, romm.CollectionVirtual, "")
	case "/api/collections/identifiers":
		f.serveCollectionIDs(w, romm.CollectionRegular)
	case "/api/collections/smart/identifiers":
		f.serveCollectionIDs(w, romm.CollectionSmart)
	case "/api/collections/virtual/identifiers":
		f.serveCollectionIDs(w, romm.CollectionVirtual)
	default:
		http.NotFound(w, r)
	}
}

func (f *Server) serveRoms(w http.ResponseWriter, platformIDs, updatedAfter, limitRaw, offsetRaw string, withTotal bool) {
	limit, _ := strconv.Atoi(limitRaw)
	offset, _ := strconv.Atoi(offsetRaw)
	want := map[int]bool{}
	first := 0
	for _, part := range strings.Split(platformIDs, ",") {
		if id, err := strconv.Atoi(part); err == nil {
			want[id] = true
			if first == 0 {
				first = id
			}
		}
	}

	f.mu.Lock()
	hook := f.beforePage
	f.mu.Unlock()
	if hook != nil {
		hook(first, offset)
	}

	f.mu.Lock()
	f.pageCounts[first]++
	failAt, failing := f.failAfter[first]
	if failing && f.pageCounts[first] == failAt {
		f.mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	after, hasAfter := parseAfter(updatedAfter)
	var matched []romm.Rom
	for _, rom := range f.Roms {
		if len(want) > 0 && !want[rom.PlatformID] {
			continue
		}
		if hasAfter && !rom.UpdatedAt.After(after) {
			continue
		}
		if f.Hidden[rom.ID] {
			continue
		}
		matched = append(matched, rom)
	}
	omitTotal := f.OmitTotal
	f.mu.Unlock()

	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	items := []romm.Rom{}
	if offset < len(matched) {
		end := offset + limit
		if limit <= 0 || end > len(matched) {
			end = len(matched)
		}
		items = matched[offset:end]
	}
	body := map[string]any{"items": items, "limit": limit, "offset": offset}
	if withTotal && !omitTotal {
		body["total"] = len(matched)
	}
	writeJSON(w, body)
}

func collectionID(kind romm.CollectionKind, id string) any {
	if kind == romm.CollectionVirtual {
		return id
	}
	n, _ := strconv.Atoi(id)
	return n
}

func (f *Server) serveCollections(w http.ResponseWriter, kind romm.CollectionKind, updatedAfter string) {
	after, hasAfter := parseAfter(updatedAfter)
	f.mu.Lock()
	out := []map[string]any{}
	for _, c := range f.Collections[kind] {
		if hasAfter && !c.UpdatedAt.After(after) {
			continue
		}
		ids := c.RomIDs
		if ids == nil {
			ids = []int{}
		}
		out = append(out, map[string]any{
			"id":         collectionID(kind, c.ID),
			"name":       c.Name,
			"rom_ids":    ids,
			"updated_at": c.UpdatedAt,
		})
	}
	f.mu.Unlock()
	writeJSON(w, out)
}

func (f *Server) serveCollectionIDs(w http.ResponseWriter, kind romm.CollectionKind) {
	f.mu.Lock()
	out := []any{}
	for _, c := range f.Collections[kind] {
		out = append(out, collectionID(kind, c.ID))
	}
	f.mu.Unlock()
	writeJSON(w, out)
}

func (f *Server) serveDeviceInit(w http.ResponseWriter, r *http.Request) {
	var req romm.DeviceInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad body", http.StatusUnprocessableEntity)
		return
	}
	f.mu.Lock()
	f.initRequests = append(f.initRequests, req)
	out := f.deviceInit
	f.mu.Unlock()
	writeJSON(w, out)
}

func (f *Server) serveDeviceToken(w http.ResponseWriter) {
	f.mu.Lock()
	hook := f.tokenHook
	reply := TokenReply{State: romm.PairPending}
	if len(f.tokenReplies) > 0 {
		reply = f.tokenReplies[0]
		if len(f.tokenReplies) > 1 {
			f.tokenReplies = f.tokenReplies[1:]
		}
	}
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if reply.State != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": string(reply.State)})
		return
	}
	writeJSON(w, reply.Token)
}

func (f *Server) serveExchange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad body", http.StatusUnprocessableEntity)
		return
	}
	f.mu.Lock()
	status := f.exchangeStatus
	tok, ok := f.exchangeCodes[req.Code]
	f.mu.Unlock()
	if status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, tok)
}

func (f *Server) serveDeviceRegister(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", http.StatusUnprocessableEntity)
		return
	}
	f.mu.Lock()
	f.nextDevice++
	id := fmt.Sprintf("device-%d", f.nextDevice)
	f.deviceRequests = append(f.deviceRequests, DeviceRequest{Method: http.MethodPost, DeviceID: id, Body: body})
	f.mu.Unlock()
	name, _ := body["name"].(string)
	writeJSON(w, romm.DeviceRef{DeviceID: id, Name: name})
}

func (f *Server) serveDeviceUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", http.StatusUnprocessableEntity)
		return
	}
	f.mu.Lock()
	f.deviceRequests = append(f.deviceRequests, DeviceRequest{Method: http.MethodPut, DeviceID: id, Body: body})
	f.mu.Unlock()
	writeJSON(w, map[string]any{"device_id": id})
}

func splitContentPath(path, prefix string) (int, string, bool) {
	rest, ok := strings.CutPrefix(path, prefix)
	if !ok {
		return 0, "", false
	}
	idPart, tail, ok := strings.Cut(rest, "/content/")
	if !ok {
		return 0, "", false
	}
	id, err := strconv.Atoi(idPart)
	if err != nil {
		return 0, "", false
	}
	return id, tail, true
}

func (f *Server) serveContent(w http.ResponseWriter, r *http.Request, id int, name string, firmware bool) {
	f.mu.Lock()
	source := f.Content
	if firmware {
		source = f.FirmwareContent
	}
	data, found := source[id]
	delay := f.contentDelay
	declared, overridden := f.declaredLength[id]
	f.mu.Unlock()
	if firmware || !overridden {
		declared = len(data)
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(declared))
	_, _ = w.Write(data)
}

func (f *Server) serveDetail(w http.ResponseWriter, r *http.Request, id int) {
	f.mu.Lock()
	detail, found := f.Details[id]
	if !found {
		for _, rom := range f.Roms {
			if rom.ID == id {
				detail, found = romm.RomDetail{Rom: rom}, true
				break
			}
		}
	}
	f.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, detail)
}
