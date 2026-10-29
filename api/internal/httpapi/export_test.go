package httpapi

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/git-demo-projectx/flightboard/api/internal/store"
	"github.com/git-demo-projectx/flightboard/api/internal/workorder"
)

func TestCSVText(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"", ""}, {"Networking", "Networking"}, {"=1+1", "'=1+1"},
		{"+1", "'+1"}, {"-1", "'-1"}, {"@SUM(A1)", "'@SUM(A1)"},
		{"\t=1", "'\t=1"}, {"\r=1", "'\r=1"}, {"a=b", "a=b"}, {"'quoted", "'quoted"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			if got := csvText(tt.input); got != tt.want {
				t.Errorf("csvText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func readExport(t *testing.T, rec *httptest.ResponseRecorder) [][]string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d body %s", rec.Code, rec.Body)
	}
	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	return rows
}

func TestExportWorkOrders(t *testing.T) {
	rec := do(t, newTestServer(t), "GET", "/api/work-orders/export", "")
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="work-orders.csv"` {
		t.Errorf("Content-Disposition = %q", got)
	}
	rows := readExport(t, rec)
	if len(rows) != 26 {
		t.Fatalf("records = %d, want 26", len(rows))
	}
	wantHeader := []string{"id", "title", "typeOfWork", "status", "subStatus", "buyer", "provider", "city", "state", "scheduleStart", "scheduleEnd", "payType", "payRate", "health"}
	if !slices.Equal(rows[0], wantHeader) {
		t.Errorf("header = %q, want %q", rows[0], wantHeader)
	}
	wantFirst := []string{"9631825", "Self-checkout lane install", "Point of Sale", "Cancelled", "", "Northwind Retail", "",
		"Plymouth", "MN", "2026-10-29T08:00:00-05:00", "2026-10-29T16:00:00-05:00", "PerDevice", "60.00", "On Track"}
	if !slices.Equal(rows[1], wantFirst) {
		t.Errorf("first row = %q, want %q", rows[1], wantFirst)
	}
	for i, row := range rows[1:] {
		if row[0] != strconv.Itoa(9631825-i) {
			t.Errorf("row %d id = %s, want %d", i, row[0], 9631825-i)
		}
		if row[0] == "9631810" && (row[4] != "Checked In" || row[6] != "Dana Whitfield" || row[12] != "65.00") {
			t.Errorf("work order 9631810 = %q", row)
		}
	}
}

func TestExportWorkOrdersFilters(t *testing.T) {
	s := newTestServer(t)
	assigned, err := s.store.GetWorkOrder(9631810)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, query string
		count       int
	}{
		{"Draft", "?status=Draft", 3},
		{"type of work", "?typeOfWork=networking", 4},
		{"provider", "?providerId=" + url.QueryEscape(assigned.ProviderID), len(s.store.ListWorkOrders(store.Filter{ProviderID: assigned.ProviderID}))},
		{"combined", "?status=Assigned&typeOfWork=" + url.QueryEscape(assigned.TypeOfWork) + "&providerId=" + url.QueryEscape(assigned.ProviderID), len(s.store.ListWorkOrders(store.Filter{Status: workorder.StatusAssigned, TypeOfWork: assigned.TypeOfWork, ProviderID: assigned.ProviderID}))},
		{"empty status", "?status=", 25},
		{"no matches", "?providerId=unknown", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := readExport(t, do(t, s, "GET", "/api/work-orders/export"+tt.query, ""))
			if len(rows) != tt.count+1 {
				t.Fatalf("data rows = %d, want %d", len(rows)-1, tt.count)
			}
			checkExportFilters(t, rows[1:], tt.query)
		})
	}
}

func checkExportFilters(t *testing.T, rows [][]string, rawQuery string) {
	t.Helper()
	query, err := url.ParseQuery(strings.TrimPrefix(rawQuery, "?"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if status := query.Get("status"); status != "" && row[3] != status {
			t.Errorf("status = %q, want %q", row[3], status)
		}
		if typ := query.Get("typeOfWork"); typ != "" && !strings.EqualFold(row[2], typ) {
			t.Errorf("typeOfWork = %q, want %q", row[2], typ)
		}
		if query.Get("providerId") != "" && row[6] != "Dana Whitfield" {
			t.Errorf("provider = %q, want Dana Whitfield", row[6])
		}
	}
}

func TestExportWorkOrdersBadStatus(t *testing.T) {
	rec := do(t, newTestServer(t), "GET", "/api/work-orders/export?status=Lost", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("unexpected attachment header = %q", got)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"error":"unknown status \"Lost\""}` {
		t.Errorf("error body = %q", got)
	}
}

func TestExportWorkOrdersNeutralizesFormulas(t *testing.T) {
	s := newTestServer(t)
	wo, err := s.store.CreateWorkOrder(workorder.WorkOrder{
		Title: `=HYPERLINK("https://example.test","open")`, Status: workorder.StatusDraft,
		TypeOfWork: "\t=type", Buyer: "@buyer", Location: workorder.Location{City: "+city", State: "-state"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := readExport(t, do(t, s, "GET", "/api/work-orders/export?status=Draft", ""))
	want := []string{strconv.Itoa(wo.ID), `'=HYPERLINK("https://example.test","open")`, "'\t=type", "Draft", "", "'@buyer", "", "'+city", "'-state", "", "", "", "0.00", "On Track"}
	if len(rows) != 5 || !slices.Equal(rows[1], want) {
		t.Fatalf("export = %q, want first data row %q", rows, want)
	}
	if got := exportRow(wo, "\r=provider")[6]; got != "'\r=provider" {
		t.Errorf("provider cell = %q", got)
	}
}

func TestExportWorkOrdersQuotesText(t *testing.T) {
	s := newTestServer(t)
	wo, err := s.store.CreateWorkOrder(workorder.WorkOrder{
		Title: `Rack, stack "and" label`, TypeOfWork: "Line one\nLine two", Status: workorder.StatusDraft,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := readExport(t, do(t, s, "GET", "/api/work-orders/export?status=Draft", ""))
	if len(rows) != 5 || rows[1][0] != strconv.Itoa(wo.ID) || rows[1][1] != wo.Title || rows[1][2] != wo.TypeOfWork {
		t.Errorf("quoted export = %q", rows)
	}
}

func TestExportWorkOrdersEmpty(t *testing.T) {
	rows := readExport(t, do(t, New(store.New()), "GET", "/api/work-orders/export", ""))
	if len(rows) != 1 || !slices.Equal(rows[0], exportHeader) {
		t.Errorf("empty export = %q, want header only", rows)
	}
}
