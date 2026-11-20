package main

/*
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>

typedef const char grout_cchar;
typedef void (*grout_event_fn)(void* userdata, const char* event_json);

static inline void grout_invoke(grout_event_fn fn, void* userdata, const char* event_json) {
	fn(userdata, event_json);
}

static inline uint64_t grout_thread_id(void) {
	return (uint64_t)(uintptr_t)pthread_self();
}
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/rommapp/grout/api"
)

var instances = newTable(func() uint64 { return uint64(C.grout_thread_id()) })

func main() {}

func cstring(b []byte) *C.char {
	return C.CString(string(b))
}

func gostring(s *C.grout_cchar) string {
	return C.GoString((*C.char)(unsafe.Pointer(s)))
}

func internal(where string, r any) []byte {
	return failure(api.CodeInternal, "internal error in %s: %v", where, r)
}

//export grout_version
func grout_version() (out *C.char) {
	defer func() {
		if r := recover(); r != nil {
			out = cstring(internal("grout_version", r))
		}
	}()
	raw, err := json.Marshal(api.Version())
	if err != nil {
		return cstring(failure(api.CodeInternal, "encoding version: %v", err))
	}
	return cstring(raw)
}

//export grout_open
func grout_open(options_json *C.grout_cchar, error_out **C.char) (h C.uintptr_t) {
	defer func() {
		if r := recover(); r != nil {
			h = 0
			if error_out != nil {
				*error_out = cstring(internal("grout_open", r))
			}
		}
	}()
	if error_out != nil {
		*error_out = nil
	}
	var raw []byte
	if options_json != nil {
		raw = []byte(gostring(options_json))
	}
	handle, failed := instances.open(raw)
	if handle == 0 && error_out != nil {
		*error_out = cstring(failed)
	}
	return C.uintptr_t(handle)
}

//export grout_call
func grout_call(handle C.uintptr_t, call_id C.int64_t, method *C.grout_cchar, args_json *C.grout_cchar) (out *C.char) {
	defer func() {
		if r := recover(); r != nil {
			out = cstring(internal("grout_call", r))
		}
	}()
	if method == nil {
		return cstring(failure(api.CodeBadRequest, "method is required"))
	}
	var raw []byte
	if args_json != nil {
		raw = []byte(gostring(args_json))
	}
	return cstring(instances.call(uintptr(handle), int64(call_id), gostring(method), raw))
}

//export grout_cancel
func grout_cancel(handle C.uintptr_t, call_id C.int64_t) {
	defer func() { _ = recover() }()
	instances.cancel(uintptr(handle), int64(call_id))
}

//export grout_set_event_callback
func grout_set_event_callback(handle C.uintptr_t, fn C.grout_event_fn, userdata unsafe.Pointer) {
	defer func() { _ = recover() }()
	if fn == nil {
		instances.setSink(uintptr(handle), nil)
		return
	}
	instances.setSink(uintptr(handle), func(_ int64, payload []byte) {
		s := C.CString(string(payload))
		defer C.free(unsafe.Pointer(s))
		C.grout_invoke(fn, userdata, s)
	})
}

//export grout_free
func grout_free(s *C.char) {
	defer func() { _ = recover() }()
	C.free(unsafe.Pointer(s))
}

//export grout_close
func grout_close(handle C.uintptr_t) {
	defer func() { _ = recover() }()
	instances.close(uintptr(handle))
}
