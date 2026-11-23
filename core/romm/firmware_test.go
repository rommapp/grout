package romm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlatformFirmware(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Write([]byte(`[{"id":4,"platform_id":9,"file_name":"gba_bios.bin","file_path":"bios/gba","file_size_bytes":16384,"is_verified":true,"md5_hash":"abc","missing_from_fs":false,"extra":1}]`))
	}))
	defer srv.Close()
	list, err := New(Options{BaseURL: srv.URL}).PlatformFirmware(context.Background(), 9)
	if err != nil {
		t.Fatal(err)
	}
	want := Firmware{ID: 4, PlatformID: 9, FileName: "gba_bios.bin", FilePath: "bios/gba", SizeBytes: 16384, IsVerified: true, MD5: "abc"}
	if path != "/api/firmware" || query != "platform_id=9" || len(list) != 1 || list[0] != want {
		t.Fatalf("path=%q query=%q list=%+v", path, query, list)
	}
}
