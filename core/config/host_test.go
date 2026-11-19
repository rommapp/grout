package config

import "testing"

func TestHostURL(t *testing.T) {
	cases := []struct {
		h    Host
		want string
	}{
		{Host{RootURI: "https://romm.local"}, "https://romm.local"},
		{Host{RootURI: "https://romm.local/", Port: 8443}, "https://romm.local:8443"},
		{Host{RootURI: "http://10.0.0.2", Port: 80}, "http://10.0.0.2:80"},
	}
	for _, c := range cases {
		if got := c.h.URL(); got != c.want {
			t.Errorf("%+v: got %q want %q", c.h, got, c.want)
		}
	}
}

func TestAuthHeader(t *testing.T) {
	if (Host{}).AuthHeader() != "" || (Host{Token: "abc"}).AuthHeader() != "Bearer abc" {
		t.Fatal("auth header")
	}
}
