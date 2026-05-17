package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

// HandleNotFound handles requests that don't match any route.
// It always logs [UNHANDLED] so unimplemented endpoints are visible in plain output.
// When proxyLogBody is enabled it also logs the request body (truncated to 512 bytes).
func (s *Server) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	if s.proxyLogBody && r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		preview := body
		truncated := ""

		if len(preview) > 512 {
			preview = preview[:512]
			truncated = "…"
		}

		log.Printf("[UNHANDLED] %s %s body(%d bytes): %s%s", r.Method, r.URL.Path, len(body), preview, truncated)
	} else {
		log.Printf("[UNHANDLED] %s %s", r.Method, r.URL.Path)
	}

	http.NotFound(w, r)
}
