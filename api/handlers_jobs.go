package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agent-matrix/matrix-runtime/internal/jobs"
	"github.com/agent-matrix/matrix-runtime/internal/logs"
)

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req jobs.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	j, err := s.manager.Create(req)
	if err != nil {
		if errors.Is(err, jobs.ErrUnknownType) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":     j.ID,
		"status":     j.Status(),
		"events_url": "/v1/jobs/" + j.ID + "/events",
	})
}

func (s *Server) handleListJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.manager.List()})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.manager.Get(r.PathValue("job_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, j.Snapshot())
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	status, ok := s.manager.Cancel(r.PathValue("job_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	j, ok := s.manager.Get(r.PathValue("job_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	streamEvents(w, r, j.Bus())
}

// streamEvents writes a job's event history and live updates as Server-Sent
// Events until the bus closes or the client disconnects.
func streamEvents(w http.ResponseWriter, r *http.Request, bus *logs.Bus) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	history, ch, cancel := bus.Subscribe()
	defer cancel()

	send := func(e logs.Event) {
		b, _ := json.Marshal(e)
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(b)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	for _, e := range history {
		send(e)
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				return
			}
			send(e)
		}
	}
}
