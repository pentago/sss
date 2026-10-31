package metric

import (
	"net/http/httptest"
	"testing"

	"sss/internal/config"
)

// The endpoint exposes instance-wide counts, so it must stay hidden from
// clients outside METRICS_ALLOWED_NETWORKS or without the configured credentials.
func TestHandlerAccessControl(t *testing.T) {
	prev := config.Opts
	t.Cleanup(func() { config.Opts = prev })

	cases := []struct {
		name       string
		env        []string
		remoteAddr string
		user, pass string
		want       int
	}{
		{"default network allows loopback", nil, "127.0.0.1:1234", "", "", 200},
		{"default network rejects others", nil, "192.0.2.1:1234", "", "", 404},
		{"configured network", []string{"METRICS_ALLOWED_NETWORKS=192.0.2.0/24"}, "192.0.2.1:1234", "", "", 200},
		{"auth required", []string{"METRICS_USERNAME=u", "METRICS_PASSWORD=p"}, "127.0.0.1:1234", "", "", 404},
		{"wrong password", []string{"METRICS_USERNAME=u", "METRICS_PASSWORD=p"}, "127.0.0.1:1234", "u", "x", 404},
		{"valid auth", []string{"METRICS_USERNAME=u", "METRICS_PASSWORD=p"}, "127.0.0.1:1234", "u", "p", 200},
		{"valid auth from untrusted network", []string{"METRICS_USERNAME=u", "METRICS_PASSWORD=p"}, "192.0.2.1:1234", "u", "p", 404},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := config.Parse(tc.env)
			if err != nil {
				t.Fatal(err)
			}
			config.Opts = opts
			r := httptest.NewRequest("GET", "/metrics", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.user != "" {
				r.SetBasicAuth(tc.user, tc.pass)
			}
			w := httptest.NewRecorder()
			Handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Errorf("got %d, want %d", w.Code, tc.want)
			}
		})
	}
}
