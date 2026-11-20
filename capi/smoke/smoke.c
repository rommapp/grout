#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "libgrout.h"

static int events;

static void on_event(void* userdata, const char* event_json) {
	int* count = userdata;
	if (strstr(event_json, "\"call_id\":") == NULL) {
		fprintf(stderr, "event without call_id: %s\n", event_json);
		exit(1);
	}
	(*count)++;
}

static void expect(int step, char* s, const char* want) {
	if (s == NULL || strstr(s, want) == NULL) {
		fprintf(stderr, "step %d: want %s, got %s\n", step, want, s == NULL ? "(null)" : s);
		grout_free(s);
		exit(1);
	}
	grout_free(s);
}

int main(int argc, char** argv) {
	if (argc != 2) {
		fprintf(stderr, "usage: %s scratch-dir\n", argv[0]);
		return 2;
	}
	const char* dir = argv[1];

	expect(1, grout_version(), "\"protocol\":1");

	size_t size = strlen(dir) * 2 + 128;
	char* options = malloc(size);
	if (options == NULL) {
		return 1;
	}
	snprintf(options, size, "{\"data_dir\": \"%s\", \"firmware\": {\"id\": \"testfw\"}, \"env\": {\"GROUT_ROOT\": \"%s\"}}", dir, dir);

	char* err = NULL;
	uintptr_t h = grout_open(options, &err);
	if (h == 0) {
		fprintf(stderr, "step 2: open failed: %s\n", err == NULL ? "(null)" : err);
		grout_free(err);
		return 1;
	}

	err = NULL;
	uintptr_t again = grout_open(options, &err);
	if (again != 0) {
		fprintf(stderr, "step 3: second open returned %lu\n", (unsigned long)again);
		return 1;
	}
	expect(3, err, "data_dir_locked");
	free(options);

	expect(4, grout_call(h, 1, "status", NULL), "not_configured");
	expect(5, grout_call(h, 2, "library.platforms", NULL), "\"ok\":true");

	grout_set_event_callback(h, on_event, &events);
	expect(6, grout_call(h, 4, "library.scan", NULL), "\"ok\":true");

	grout_close(h);
	expect(7, grout_call(h, 3, "status", NULL), "bad_request");

	printf("smoke ok\n");
	return 0;
}
