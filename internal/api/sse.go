package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ssePad is a comment the browser can discard. It fills the first chunk
// so Chrome and similar clients start delivering events immediately.
const ssePad = 2048

type sseWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprintf(w, ":%s\n\n", strings.Repeat(" ", ssePad)); err != nil {
		return nil, err
	}
	if err := rc.Flush(); err != nil {
		return nil, err
	}
	return &sseWriter{w: w, rc: rc}, nil
}

func (s *sseWriter) Send(event string, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return err
	}
	return s.rc.Flush()
}
