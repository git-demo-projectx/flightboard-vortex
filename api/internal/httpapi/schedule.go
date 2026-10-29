package httpapi

import (
	"net/http"

	"github.com/git-demo-projectx/flightboard/api/internal/workorder"
)

func (s *Server) rescheduleWorkOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in workorder.Schedule
	if !readJSON(w, r, &in) {
		return
	}
	wo, err := s.store.Reschedule(id, in)
	s.respond(w, http.StatusOK, wo, err)
}
