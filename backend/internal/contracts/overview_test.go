package contracts

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSiteOverviewJSONPreservesMissingValues(t *testing.T) {
	measuredAt := time.Date(2026, time.November, 16, 12, 0, 0, 0, time.UTC)
	overview := SiteOverview{
		Site: SiteSummary{
			ID:       "site-1",
			Name:     "Casa de ejemplo",
			Timezone: "America/Havana",
		},
		Readings: []ReadingSummary{
			{
				Label:      "Potencia solar",
				Value:      nil,
				Unit:       "kW",
				MeasuredAt: &measuredAt,
			},
			{
				Label:      "Estado de red",
				Value:      nil,
				Unit:       "",
				MeasuredAt: nil,
			},
		},
		Devices:   []DeviceSummary{},
		UpdatedAt: nil,
	}

	body, err := json.Marshal(overview)
	if err != nil {
		t.Fatal(err)
	}

	var encoded map[string]any
	if err := json.Unmarshal(body, &encoded); err != nil {
		t.Fatal(err)
	}

	if encoded["updatedAt"] != nil {
		t.Fatalf("updatedAt = %v, want null", encoded["updatedAt"])
	}

	readings, ok := encoded["readings"].([]any)
	if !ok || len(readings) != 2 {
		t.Fatalf("readings = %v, want two items", encoded["readings"])
	}

	first := readings[0].(map[string]any)
	if first["value"] != nil {
		t.Fatalf("first reading value = %v, want null", first["value"])
	}
	if first["measuredAt"] != measuredAt.Format(time.RFC3339Nano) {
		t.Fatalf("first measuredAt = %v, want %s", first["measuredAt"], measuredAt.Format(time.RFC3339Nano))
	}

	second := readings[1].(map[string]any)
	if second["measuredAt"] != nil {
		t.Fatalf("second measuredAt = %v, want null", second["measuredAt"])
	}
}

func TestErrorResponseJSONUsesPublicEnvelope(t *testing.T) {
	body, err := json.Marshal(ErrorResponse{
		Code:      "organization_forbidden",
		Message:   "No tienes acceso a esta organización.",
		RequestID: "request-123",
	})
	if err != nil {
		t.Fatal(err)
	}

	const want = `{"code":"organization_forbidden","message":"No tienes acceso a esta organización.","requestId":"request-123"}`
	if string(body) != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}

func TestListResponsesUseItems(t *testing.T) {
	sites, err := json.Marshal(SiteListResponse{
		Items: []SiteSummary{{ID: "site-1", Name: "Casa de ejemplo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(sites) != `{"items":[{"id":"site-1","name":"Casa de ejemplo","timezone":""}]}` {
		t.Fatalf("sites = %s", sites)
	}

	devices, err := json.Marshal(DeviceListResponse{
		Items: []DeviceSummary{{ID: "device-1", Name: "Batería", Kind: "battery", Status: nil}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(devices) != `{"items":[{"id":"device-1","name":"Batería","kind":"battery","status":null}]}` {
		t.Fatalf("devices = %s", devices)
	}
}
