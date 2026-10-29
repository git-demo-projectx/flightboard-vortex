package httpapi

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/git-demo-projectx/flightboard/api/internal/workorder"
)

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
			t.Errorf("row %d id = %q, want %d", i, row[0], 9631825-i)
		}
	}
	assigned := rows[16]
	if assigned[4] != "Checked In" || assigned[6] != "Dana Whitfield" || assigned[12] != "65.00" {
		t.Errorf("assigned row = %q", assigned)
	}
}

func TestExportWorkOrdersFilters(t *testing.T) {
	s := newTestServer(t)
	for _, tt := range []struct {
		name, query string
		wantIDs     []string
	}{
		{"draft", "?status=Draft", []string{"9631803", "9631802", "9631801"}},
		{"type", "?typeOfWork=networking", []string{"9631824", "9631820", "9631810", "9631802"}},
		{"mixed case type", "?typeOfWork=nEtWoRkInG", []string{"9631824", "9631820", "9631810", "9631802"}},
		{"provider", "?providerId=p-101&typeOfWork=Networking", []string{"9631810"}},
		{"combined", "?status=Assigned&providerId=p-101&typeOfWork=networking", []string{"9631810"}},
		{"no matches", "?providerId=unknown", []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := readExport(t, do(t, s, "GET", "/api/work-orders/export"+tt.query, ""))
			gotIDs := make([]string, 0, len(rows)-1)
			for _, row := range rows[1:] {
				gotIDs = append(gotIDs, row[0])
				if tt.name == "draft" && row[3] != "Draft" {
					t.Errorf("status = %q, want Draft", row[3])
				}
			}
			if !slices.Equal(gotIDs, tt.wantIDs) {
				t.Errorf("ids = %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

func TestExportWorkOrdersBadStatus(t *testing.T) {
	rec := do(t, newTestServer(t), "GET", "/api/work-orders/export?status=Lost", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("unexpected attachment header = %q", got)
	}
	if got, want := rec.Body.String(), "{\"error\":\"unknown status \\\"Lost\\\"\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestExportWorkOrdersNeutralizesFormulas(t *testing.T) {
	s := newTestServer(t)
	wo, err := s.store.CreateWorkOrder(workorder.WorkOrder{
		Title: `=HYPERLINK("https://example.test","open")`, Status: workorder.StatusDraft,
		Buyer: "@buyer", TypeOfWork: "-type", Location: workorder.Location{City: "+city", State: "\tstate"},
	})
	if err != nil {
		t.Fatalf("CreateWorkOrder: %v", err)
	}
	rows := readExport(t, do(t, s, "GET", "/api/work-orders/export?status=Draft", ""))
	want := []string{strconv.Itoa(wo.ID), `'=HYPERLINK("https://example.test","open")`, "'-type", "Draft", "", "'@buyer", "",
		"'+city", "'\tstate", "", "", "", "0.00", "On Track"}
	if len(rows) != 5 || !slices.Equal(rows[1], want) {
		t.Fatalf("rows = %q, want first data row %q and 5 records", rows, want)
	}
}

func TestExportWorkOrdersQuotesText(t *testing.T) {
	s := newTestServer(t)
	wo, err := s.store.CreateWorkOrder(workorder.WorkOrder{
		Title: `Rack, stack "and" label`, TypeOfWork: "Line one\nLine two", Status: workorder.StatusDraft,
	})
	if err != nil {
		t.Fatalf("CreateWorkOrder: %v", err)
	}
	rows := readExport(t, do(t, s, "GET", "/api/work-orders/export?status=Draft", ""))
	if len(rows) != 5 || rows[1][0] != strconv.Itoa(wo.ID) || rows[1][1] != wo.Title || rows[1][2] != wo.TypeOfWork {
		t.Fatalf("rows = %q, want created title and type unchanged", rows)
	}
}

func TestExportRowNeutralizesProvider(t *testing.T) {
	row := exportRow(workorder.WorkOrder{}, "\r=provider")
	if row[6] != "'\r=provider" {
		t.Errorf("provider = %q, want %q", row[6], "'\r=provider")
	}
}
