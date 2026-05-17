package handlers

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gesellix/bose-soundtouch/pkg/service/datastore"
)

func TestHandleNotFound_UnhandledLogging(t *testing.T) {
	captureLog := func(fn func()) string {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(os.Stderr)
		fn()
		return buf.String()
	}

	t.Run("always logs [UNHANDLED] with method and path", func(t *testing.T) {
		ds := datastore.NewDataStore(t.TempDir())
		server := NewServer(ds, nil, "http://localhost", false, false, false)

		req := httptest.NewRequest("GET", "/some/unknown/path", nil)

		logged := captureLog(func() {
			server.HandleNotFound(httptest.NewRecorder(), req)
		})

		if !strings.Contains(logged, "[UNHANDLED]") {
			t.Errorf("expected [UNHANDLED] in log, got: %s", logged)
		}
		if !strings.Contains(logged, "GET") || !strings.Contains(logged, "/some/unknown/path") {
			t.Errorf("expected method and path in log, got: %s", logged)
		}
	})

	t.Run("includes body in log when proxyLogBody is true", func(t *testing.T) {
		ds := datastore.NewDataStore(t.TempDir())
		server := NewServer(ds, nil, "http://localhost", false, true, false)

		req := httptest.NewRequest("POST", "/marge/unknown", bytes.NewBufferString("<payload/>"))

		logged := captureLog(func() {
			server.HandleNotFound(httptest.NewRecorder(), req)
		})

		if !strings.Contains(logged, "<payload/>") {
			t.Errorf("expected body in log when proxyLogBody=true, got: %s", logged)
		}
	})

	t.Run("omits body from log when proxyLogBody is false", func(t *testing.T) {
		ds := datastore.NewDataStore(t.TempDir())
		server := NewServer(ds, nil, "http://localhost", false, false, false)

		req := httptest.NewRequest("POST", "/marge/unknown", bytes.NewBufferString("<secret/>"))

		logged := captureLog(func() {
			server.HandleNotFound(httptest.NewRecorder(), req)
		})

		if strings.Contains(logged, "<secret/>") {
			t.Errorf("expected body omitted when proxyLogBody=false, got: %s", logged)
		}
		if !strings.Contains(logged, "[UNHANDLED]") {
			t.Errorf("expected [UNHANDLED] even without body, got: %s", logged)
		}
	})

	t.Run("returns 404 for unmatched routes", func(t *testing.T) {
		ds := datastore.NewDataStore(t.TempDir())
		server := NewServer(ds, nil, "http://localhost", false, false, false)

		req := httptest.NewRequest("GET", "/some/unknown/path", nil)
		w := httptest.NewRecorder()

		server.HandleNotFound(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unmatched route, got %d", w.Code)
		}
	})
}
