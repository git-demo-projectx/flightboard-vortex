package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/git-demo-projectx/flightboard/api/internal/store"
	"github.com/git-demo-projectx/flightboard/api/internal/workorder"
)

func TestAssignProvider(t *testing.T) {
	s := newTestServer(t)
	path := "/api/work-orders/9631805"
	rec := do(t, s, "POST", path+"/assign", `{"providerId":"p-104"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign code = %d body %s", rec.Code, rec.Body)
	}
	assigned := decode[workOrderView](t, rec)
	if assigned.Status != workorder.StatusAssigned || assigned.ProviderID != "p-104" || assigned.SubStatus != "" || assigned.Provider == nil || assigned.Provider.Name != "Tomas Lindqvist" {
		t.Fatalf("assigned work order = %+v", assigned)
	}
	rec = do(t, s, "GET", path, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get code = %d body %s", rec.Code, rec.Body)
	}
	if got := decode[workOrderView](t, rec); !reflect.DeepEqual(got, assigned) {
		t.Fatalf("detail = %+v, want %+v", got, assigned)
	}
	rec = do(t, s, "GET", path+"/nearby-providers", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("nearby code = %d body %s", rec.Code, rec.Body)
	}
	for _, p := range decode[[]workorder.Provider](t, rec) {
		if p.ID == "p-104" {
			t.Fatal("assigned provider is still listed as nearby")
		}
	}
}

func TestAssignProviderDoubleBooking(t *testing.T) {
	for _, tt := range []struct {
		name, start, end string
		code             int
	}{
		{"overlapping", "2026-10-22T10:00:00-05:00", "2026-10-22T12:00:00-05:00", http.StatusConflict},
		{"different zones", "2026-10-22T18:00:00Z", "2026-10-22T19:00:00Z", http.StatusConflict},
		{"touches end", "2026-10-22T14:00:00-05:00", "2026-10-22T15:00:00-05:00", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			body, err := json.Marshal(struct {
				Title      string             `json:"title"`
				TypeOfWork string             `json:"typeOfWork"`
				Status     workorder.Status   `json:"status"`
				Location   workorder.Location `json:"location"`
				Pay        workorder.Pay      `json:"pay"`
				Schedule   map[string]string  `json:"schedule"`
			}{
				Title: "Install access points", TypeOfWork: "Networking", Status: workorder.StatusPublished,
				Location: workorder.Location{City: "Edina", State: "MN"}, Pay: workorder.Pay{Type: workorder.PayFixed, Rate: 100},
				Schedule: map[string]string{"start": tt.start, "end": tt.end},
			})
			if err != nil {
				t.Fatal(err)
			}
			rec := do(t, s, "POST", "/api/work-orders", string(body))
			if rec.Code != http.StatusCreated {
				t.Fatalf("create code = %d body %s", rec.Code, rec.Body)
			}
			created := decode[workOrderView](t, rec)
			path := fmt.Sprintf("/api/work-orders/%d", created.ID)
			rec = do(t, s, "POST", path+"/assign", `{"providerId":"p-101"}`)
			if rec.Code != tt.code {
				t.Fatalf("assign code = %d body %s, want %d", rec.Code, rec.Body, tt.code)
			}
			created = checkBookingResponse(t, rec, created)
			rec = do(t, s, "GET", path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("get code = %d body %s", rec.Code, rec.Body)
			}
			if got := decode[workOrderView](t, rec); !reflect.DeepEqual(got, created) {
				t.Fatalf("stored work order = %+v, want %+v", got, created)
			}
		})
	}
}

func checkBookingResponse(t *testing.T, rec *httptest.ResponseRecorder, created workOrderView) workOrderView {
	t.Helper()
	if rec.Code == http.StatusConflict {
		want := "conflict: Dana Whitfield is booked on work order 9631817 at that time"
		if got := decode[map[string]string](t, rec)["error"]; got != want {
			t.Errorf("error = %q, want %q", got, want)
		}
		return created
	}
	assigned := decode[workOrderView](t, rec)
	if assigned.Status != workorder.StatusAssigned || assigned.ProviderID != "p-101" || assigned.Provider == nil || assigned.Provider.Name != "Dana Whitfield" {
		t.Fatalf("assigned work order = %+v", assigned)
	}
	return assigned
}

func TestAssignProviderErrors(t *testing.T) {
	for _, tt := range []struct {
		name, path, body string
		code             int
		message          string
	}{
		{"bad id", "/api/work-orders/abc/assign", `{"providerId":"p-101"}`, 400, "id must be a number"},
		{"not JSON", "/api/work-orders/9631805/assign", "not json", 400, "invalid JSON body"},
		{"missing provider", "/api/work-orders/9631805/assign", `{}`, 400, "invalid work order: providerId is required"},
		{"blank provider", "/api/work-orders/9631805/assign", `{"providerId":"  "}`, 400, "invalid work order: providerId is required"},
		{"unknown provider", "/api/work-orders/9631805/assign", `{"providerId":"p-999"}`, 400, `invalid work order: unknown provider "p-999"`},
		{"unknown work order", "/api/work-orders/1/assign", `{"providerId":"p-101"}`, 404, "not found"},
		{"Draft", "/api/work-orders/9631801/assign", `{"providerId":"p-101"}`, 409, "conflict: cannot move from Draft to Assigned"},
		{"Assigned", "/api/work-orders/9631810/assign", `{"providerId":"p-104"}`, 409, "conflict: cannot move from Assigned to Assigned"},
		{"Cancelled", "/api/work-orders/9631825/assign", `{"providerId":"p-101"}`, 409, "conflict: cannot move from Cancelled to Assigned"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t)
			before := s.store.ListWorkOrders(store.Filter{})
			rec := do(t, s, "POST", tt.path, tt.body)
			if rec.Code != tt.code {
				t.Fatalf("code = %d body %s, want %d", rec.Code, rec.Body, tt.code)
			}
			got := decode[map[string]string](t, rec)
			if !reflect.DeepEqual(got, map[string]string{"error": tt.message}) {
				t.Errorf("error body = %v, want error %q", got, tt.message)
			}
			if after := s.store.ListWorkOrders(store.Filter{}); !reflect.DeepEqual(before, after) {
				t.Fatal("failed request changed stored work orders")
			}
		})
	}
}

func TestAssignProviderMissingSchedule(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, "POST", "/api/work-orders", `{"title":"No schedule","status":"Draft"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create code = %d body %s", rec.Code, rec.Body)
	}
	id := decode[woJSON](t, rec).ID
	path := fmt.Sprintf("/api/work-orders/%d", id)
	rec = do(t, s, "POST", path+"/status", `{"status":"Routed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("route code = %d body %s", rec.Code, rec.Body)
	}
	before := decode[workOrderView](t, rec)
	rec = do(t, s, "POST", path+"/assign", `{"providerId":"p-101"}`)
	want := fmt.Sprintf("conflict: work order %d has no schedule", id)
	if rec.Code != http.StatusConflict || decode[map[string]string](t, rec)["error"] != want {
		t.Fatalf("code = %d body %s, want 409 error %q", rec.Code, rec.Body, want)
	}
	rec = do(t, s, "GET", path, "")
	if rec.Code != http.StatusOK || !reflect.DeepEqual(decode[workOrderView](t, rec), before) {
		t.Fatalf("failed assignment changed work order: code %d body %s", rec.Code, rec.Body)
	}
}
