package httpapi

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/git-demo-projectx/flightboard/api/internal/workorder"
)

func scheduleWeek(t *testing.T, s *Server, start string) weekView {
	t.Helper()
	rec := do(t, s, "GET", "/api/work-orders/week?start="+start, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("week code = %d body %s", rec.Code, rec.Body)
	}
	return decode[weekView](t, rec)
}

func assertRescheduledWeek(t *testing.T, week weekView, start, end time.Time) {
	t.Helper()
	if week.Total != 1 {
		t.Fatalf("new week total = %d, want 1", week.Total)
	}
	for _, day := range week.Days {
		for _, item := range day.WorkOrders {
			if item.ID != 9631804 {
				continue
			}
			if day.Date != "2026-11-03" || !item.Start.Equal(start) || !item.End.Equal(end) {
				t.Errorf("rescheduled week item on %s = %+v", day.Date, item)
			}
			return
		}
	}
	t.Fatal("rescheduled work order missing from new week")
}

func assertUnchangedWorkOrderResponse(t *testing.T, s *Server, path string, beforeCode int, before workorder.WorkOrder) {
	t.Helper()
	rec := do(t, s, "GET", path, "")
	if rec.Code != beforeCode {
		t.Fatalf("GET code = %d, want %d", rec.Code, beforeCode)
	}
	if after := decode[workorder.WorkOrder](t, rec); !reflect.DeepEqual(after, before) {
		t.Errorf("failed reschedule changed work order: before %+v, after %+v", before, after)
	}
}

func TestRescheduleWorkOrder(t *testing.T) {
	s := newTestServer(t)
	body := `{"start":"2026-11-03T09:00:00-06:00","end":"2026-11-03T13:00:00-06:00"}`
	rec := do(t, s, "POST", "/api/work-orders/9631804/schedule", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	got := decode[workorder.WorkOrder](t, rec)
	start := time.Date(2026, 11, 3, 15, 0, 0, 0, time.UTC)
	end := start.Add(4 * time.Hour)
	if got.ID != 9631804 || got.Status != workorder.StatusPublished || got.Schedule == nil || !got.Schedule.Start.Equal(start) || !got.Schedule.End.Equal(end) {
		t.Fatalf("rescheduled work order = %+v, want Published with sent window", got)
	}

	assertRescheduledWeek(t, scheduleWeek(t, s, "2026-11-02"), start, end)
	for _, day := range scheduleWeek(t, s, "2026-10-19").Days {
		for _, item := range day.WorkOrders {
			if item.ID == 9631804 {
				t.Errorf("rescheduled work order still in old week on %s", day.Date)
			}
		}
	}
}

func TestRescheduleWorkOrderErrors(t *testing.T) {
	valid := `{"start":"2026-11-03T09:00:00Z","end":"2026-11-03T13:00:00Z"}`
	backwards := `{"start":"2026-11-03T13:00:00Z","end":"2026-11-03T09:00:00Z"}`
	const missing = "invalid work order: schedule start and end are required"
	const conflict = "conflict: only Draft, Published or Routed work orders can be rescheduled"
	for _, tt := range []struct {
		name string
		id   string
		body string
		code int
		err  string
	}{
		{"id not a number", "abc", valid, http.StatusBadRequest, "id must be a number"},
		{"unknown id", "1", valid, http.StatusNotFound, "not found"},
		{"not JSON", "9631804", "not json", http.StatusBadRequest, "invalid JSON body"},
		{"time not RFC 3339", "9631804", `{"start":"tomorrow","end":"2026-11-03T13:00:00Z"}`, http.StatusBadRequest, "invalid JSON body"},
		{"empty body object", "9631804", `{}`, http.StatusBadRequest, missing},
		{"end only", "9631804", `{"end":"2026-11-03T13:00:00Z"}`, http.StatusBadRequest, missing},
		{"start only", "9631804", `{"start":"2026-11-03T09:00:00Z"}`, http.StatusBadRequest, missing},
		{"end before start", "9631804", backwards, http.StatusBadRequest, "invalid work order: schedule end must be after start"},
		{"15 days", "9631804", `{"start":"2026-11-03T09:00:00Z","end":"2026-11-18T09:00:00Z"}`, http.StatusBadRequest, "invalid work order: schedule must not span more than 14 days"},
		{"Assigned", "9631810", valid, http.StatusConflict, conflict},
		{"Work Done", "9631818", valid, http.StatusConflict, conflict},
		{"Approved", "9631821", valid, http.StatusConflict, conflict},
		{"Paid", "9631823", valid, http.StatusConflict, conflict},
		{"Cancelled", "9631825", valid, http.StatusConflict, conflict},
		{"Assigned and bad window", "9631810", backwards, http.StatusConflict, conflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			path := "/api/work-orders/" + tt.id
			beforeRec := do(t, s, "GET", path, "")
			before := decode[workorder.WorkOrder](t, beforeRec)
			rec := do(t, s, "POST", path+"/schedule", tt.body)
			if rec.Code != tt.code {
				t.Fatalf("code = %d body %s, want %d", rec.Code, rec.Body, tt.code)
			}
			if got := decode[map[string]string](t, rec); !reflect.DeepEqual(got, map[string]string{"error": tt.err}) {
				t.Errorf("error body = %v, want error %q", got, tt.err)
			}
			if tt.code == http.StatusBadRequest || tt.code == http.StatusConflict {
				assertUnchangedWorkOrderResponse(t, s, path, beforeRec.Code, before)
			}
		})
	}
}
