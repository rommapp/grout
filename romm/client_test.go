package romm

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSaveListProductionDefaults(t *testing.T) {
	if defaultSaveListLimits.maxDecodedBytes != 16777216 || defaultSaveListLimits.maxRecords != 10000 || defaultSaveListLimits.maxErrorBodyBytes != 65536 {
		t.Fatalf("save-list defaults=%+v", defaultSaveListLimits)
	}
}

func streamTestClient(t *testing.T, status int, body string) (*Client, *int) {
	t.Helper()
	requests := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++
		if r.Method != http.MethodGet || r.URL.Path != endpointSaves {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.RawQuery; got != "device_id=device-1" {
			t.Errorf("query = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test" {
			t.Errorf("authorization = %q", got)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL, WithAuthHeader("Bearer test")), requests
}

func TestStreamSavesExactByteAndRecordLimitsSucceed(t *testing.T) {
	body := `[{"id":1,"rom_id":7},{"id":2,"rom_id":8}]`
	client, requests := streamTestClient(t, http.StatusOK, body)
	limits := saveListLimits{maxDecodedBytes: int64(len(body)), maxRecords: 2, maxErrorBodyBytes: 64}
	saves, stats, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7, 8}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if *requests != 1 || len(saves) != 2 || stats.RecordsSeen != 2 || stats.FilteredRecords != 2 || stats.BytesRead != int64(len(body)) {
		t.Fatalf("requests=%d saves=%+v stats=%+v", *requests, saves, stats)
	}
}

func TestStreamSavesOneByteAndOneRecordOverLimitLeakNothing(t *testing.T) {
	body := `[{"id":1,"rom_id":7},{"id":2,"rom_id":8}]`
	for name, limits := range map[string]saveListLimits{
		"byte":   {maxDecodedBytes: int64(len(body) - 1), maxRecords: 2, maxErrorBodyBytes: 64},
		"record": {maxDecodedBytes: int64(len(body)), maxRecords: 1, maxErrorBodyBytes: 64},
	} {
		t.Run(name, func(t *testing.T) {
			client, _ := streamTestClient(t, http.StatusOK, body)
			saves, _, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7, 8}, limits)
			if err == nil || len(saves) != 0 {
				t.Fatalf("err=%v saves=%+v", err, saves)
			}
			want := saveListFailureByteLimit
			if name == "record" {
				want = saveListFailureRecordLimit
			}
			if got := SaveListFallbackReason(err); got != want {
				t.Fatalf("reason=%q want=%q err=%v", got, want, err)
			}
		})
	}
}

func TestStreamSavesByteLimitCountsDecodedCompressedBody(t *testing.T) {
	decoded := `[{"id":1,"rom_id":7}]` + strings.Repeat(" ", 2048)
	var compressed bytes.Buffer
	zipper := gzip.NewWriter(&compressed)
	_, _ = io.WriteString(zipper, decoded)
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed.Len() >= len(decoded) {
		t.Fatal("test body did not compress")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed.Bytes())
	}))
	defer server.Close()
	saves, stats, err := NewClient(server.URL).streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{int64(len(decoded) - 1), 10, 64})
	if err == nil || len(saves) != 0 || SaveListFallbackReason(err) != saveListFailureByteLimit || stats.BytesRead != int64(len(decoded)) {
		t.Fatalf("reason=%q bytes=%d saves=%+v err=%v", SaveListFallbackReason(err), stats.BytesRead, saves, err)
	}
}

func TestStreamSavesRejectsInvalidOrIncompleteJSONWithoutPartialResults(t *testing.T) {
	tests := map[string]struct {
		body   string
		reason string
	}{
		"opening":           {`{"id":1}`, saveListFailureDecode},
		"truncated":         {`[{"id":1,"rom_id":7}`, saveListFailureDecode},
		"malformed_record":  {`[{"id":}]`, saveListFailureDecode},
		"trailing_value":    {`[] {}`, saveListFailureTrailingData},
		"trailing_nonwhite": {`[]x`, saveListFailureTrailingData},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client, _ := streamTestClient(t, http.StatusOK, test.body)
			saves, _, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{1, 7}, saveListLimits{1024, 10, 64})
			if err == nil || len(saves) != 0 || SaveListFallbackReason(err) != test.reason {
				t.Fatalf("reason=%q err=%v saves=%+v", SaveListFallbackReason(err), err, saves)
			}
		})
	}
}

func TestGetSavesForROMIDsReturnsNoRecordBeforeTrailingValidation(t *testing.T) {
	client, _ := streamTestClient(t, http.StatusOK, `[{"id":1,"rom_id":7}] trailing`)
	saves, _, err := client.GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7})
	if err == nil || len(saves) != 0 {
		t.Fatalf("err=%v saves=%+v", err, saves)
	}
}

func TestGetSavesForROMIDsRejectsInvalidQueryBeforeHTTP(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	saves, stats, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{}, []int{7})
	if err == nil || len(saves) != 0 || SaveListFallbackReason(err) != "invalid_query" {
		t.Fatalf("reason=%q saves=%+v stats=%+v err=%v", SaveListFallbackReason(err), saves, stats, err)
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid query issued %d requests", requests.Load())
	}
}

func TestGetSavesForROMIDsEmptyPositiveSetReturnsNonNilEmptyWithoutHTTP(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	saves, stats, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{0, -1, 0})
	if err != nil || saves == nil || len(saves) != 0 || stats != (SaveListStreamStats{}) {
		t.Fatalf("saves=%+v stats=%+v err=%v", saves, stats, err)
	}
	if requests.Load() != 0 {
		t.Fatalf("empty positive ID set issued %d requests", requests.Load())
	}
}

func TestGetSavesForROMIDsCopiesPositiveIDsBeforeIO(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseResponse
		_, _ = io.WriteString(w, `[{"id":1,"rom_id":7},{"id":2,"rom_id":999},{"id":3,"rom_id":7}]`)
	}))
	defer server.Close()

	type result struct {
		saves []Save
		stats SaveListStreamStats
		err   error
	}
	romIDs := []int{7, 7, 0, -1}
	resultCh := make(chan result, 1)
	go func() {
		saves, stats, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, romIDs)
		resultCh <- result{saves: saves, stats: stats, err: err}
	}()
	<-requestStarted
	for i := range romIDs {
		romIDs[i] = 999
	}
	close(releaseResponse)
	got := <-resultCh
	if got.err != nil || got.stats.RecordsSeen != 3 || got.stats.FilteredRecords != 2 || len(got.saves) != 2 {
		t.Fatalf("saves=%+v stats=%+v err=%v", got.saves, got.stats, got.err)
	}
	if got.saves[0].ID != 1 || got.saves[1].ID != 3 {
		t.Fatalf("copied-ID order/duplicates=%+v", got.saves)
	}
}

func TestStreamSavesNetworkInterruptionAndTimeoutLeakNothing(t *testing.T) {
	t.Run("interruption", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.(http.Hijacker)
			conn, buf, err := h.Hijack()
			if err != nil {
				t.Fatal(err)
			}
			_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n[{\"id\":1,\"rom_id\":7}")
			_ = buf.Flush()
			_ = conn.Close()
		}))
		defer server.Close()
		saves, _, err := NewClient(server.URL).streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{1024, 10, 64})
		if err == nil || len(saves) != 0 {
			t.Fatalf("err=%v saves=%+v", err, saves)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, `[]`)
		}))
		defer server.Close()
		saves, _, err := NewClient(server.URL, WithTimeout(time.Millisecond)).streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{1024, 10, 64})
		if err == nil || len(saves) != 0 || SaveListFallbackReason(err) != saveListFailureTransport {
			t.Fatalf("reason=%q err=%v saves=%+v", SaveListFallbackReason(err), err, saves)
		}
	})
}

func TestStreamSavesAllNon2xxClassesAndErrorBodyAreBounded(t *testing.T) {
	for _, status := range []int{199, 302, 404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client := NewClient("http://unused")
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 4096))), Header: make(http.Header)}, nil
			})
			saves, _, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{1}, saveListLimits{1024, 10, 64})
			if err == nil || len(saves) != 0 || SaveListFallbackReason(err) != saveListFailureHTTPStatus {
				t.Fatalf("status=%d reason=%q err=%v", status, SaveListFallbackReason(err), err)
			}
			requestErr, ok := err.(*saveListRequestError)
			if !ok || len(requestErr.retainedBody) != 64 {
				t.Fatalf("retained error body = %d, want 64", len(requestErr.retainedBody))
			}
		})
	}
}

func TestStreamSavesDefaultErrorBodyRetentionIs65536Bytes(t *testing.T) {
	client := NewClient("http://unused")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 100000))), Header: make(http.Header)}, nil
	})
	saves, _, err := client.GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{1})
	requestErr, ok := err.(*saveListRequestError)
	if !ok || len(saves) != 0 || len(requestErr.retainedBody) != 65536 {
		t.Fatalf("retained=%d saves=%d err=%v", len(requestErr.retainedBody), len(saves), err)
	}
}

func TestStreamSavesSuccessfulEmptyAndIgnoredDeviceFiltering(t *testing.T) {
	client, _ := streamTestClient(t, http.StatusOK, `[]`)
	saves, stats, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{1024, 10, 64})
	if err != nil || saves == nil || len(saves) != 0 || stats.RecordsSeen != 0 {
		t.Fatalf("empty saves=%+v stats=%+v err=%v", saves, stats, err)
	}

	body := `[{"id":1,"rom_id":7},{"id":2,"rom_id":999},{"id":3,"rom_id":7},{"id":1,"rom_id":7}]`
	client, _ = streamTestClient(t, http.StatusOK, body)
	saves, stats, err = client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{1024, 10, 64})
	if err != nil || stats.RecordsSeen != 4 || stats.FilteredRecords != 3 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	for i, want := range []int{1, 3, 1} {
		if saves[i].ID != want {
			t.Fatalf("order[%d]=%d want=%d", i, saves[i].ID, want)
		}
	}
}

func TestStreamSavesNoContentIsAuthoritativeEmpty(t *testing.T) {
	client, _ := streamTestClient(t, http.StatusNoContent, "")
	saves, stats, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{1024, 10, 64})
	if err != nil || saves == nil || len(saves) != 0 || stats != (SaveListStreamStats{}) {
		t.Fatalf("saves=%+v stats=%+v err=%v", saves, stats, err)
	}
}

func TestStreamSavesPartialContentIsRejected(t *testing.T) {
	client, _ := streamTestClient(t, http.StatusPartialContent, `[{"id":1,"rom_id":7}]`)
	saves, _, err := client.streamSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7}, saveListLimits{1024, 10, 64})
	if err == nil || len(saves) != 0 || SaveListFallbackReason(err) != "http_status" {
		t.Fatalf("reason=%q saves=%+v err=%v", SaveListFallbackReason(err), saves, err)
	}
}

func BenchmarkDecodeSaveList100000_NonProductionRecordLimit100000(b *testing.B) {
	var body bytes.Buffer
	body.WriteByte('[')
	for i := 0; i < 100000; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(&body, `{"id":%d,"rom_id":%d}`, i+1, i+1)
	}
	body.WriteByte(']')
	payload := body.Bytes()
	for _, matches := range []int{0, 10, 1000} {
		b.Run(fmt.Sprintf("M=%d", matches), func(b *testing.B) {
			b.ReportAllocs()
			wanted := make(map[int]struct{}, matches)
			for romID := 1; romID <= matches; romID++ {
				wanted[romID] = struct{}{}
			}
			retainedBytes := 0
			for i := 0; i < b.N; i++ {
				saves, stats, err := decodeSaveList(bytes.NewReader(payload), wanted, saveListLimits{int64(len(payload)), 100000, 64})
				if err != nil || len(saves) != matches || stats.RecordsSeen != 100000 {
					b.Fatalf("saves=%d stats=%+v err=%v", len(saves), stats, err)
				}
				retainedBytes = cap(saves) * int(reflect.TypeOf(Save{}).Size())
			}
			b.ReportMetric(float64(retainedBytes), "retained-B/op")
		})
	}
}
