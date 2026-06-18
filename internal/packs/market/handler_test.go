package marketpack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/skillmarket"
)

func TestMarketPackSearchUsesLocalMarket(t *testing.T) {
	market := skillmarket.NewMarket(t.TempDir())
	if err := market.Publish(skillmarket.SkillMeta{
		Name:        "browser-helper",
		Description: "Help with browser work",
		Version:     "0.1.0",
		Author:      "Yunque",
		Rating:      4.8,
	}); err != nil {
		t.Fatal(err)
	}
	h := NewProvider(func() *skillmarket.Market { return market })

	req := httptest.NewRequest(http.MethodGet, "/v1/market/search?q=browser", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Skills []skillmarket.SkillMeta `json:"skills"`
		Count  int                     `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Skills) != 1 || body.Skills[0].Name != "browser-helper" {
		t.Fatalf("unexpected search body: %+v", body)
	}
}

func TestMarketPackTopByRating(t *testing.T) {
	market := skillmarket.NewMarket(t.TempDir())
	if err := market.Publish(skillmarket.SkillMeta{Name: "low", Version: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := market.Publish(skillmarket.SkillMeta{Name: "high", Version: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := market.Rate("low", 3.0); err != nil {
		t.Fatal(err)
	}
	if err := market.Rate("high", 5.0); err != nil {
		t.Fatal(err)
	}
	h := NewProvider(func() *skillmarket.Market { return market })

	req := httptest.NewRequest(http.MethodGet, "/v1/market/top?by=rating&n=1", nil)
	rec := httptest.NewRecorder()

	h.Top(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Skills []skillmarket.SkillMeta `json:"skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Skills) != 1 || body.Skills[0].Name != "high" {
		t.Fatalf("unexpected top body: %+v", body)
	}
}

func TestMarketPackStatsNilMarket(t *testing.T) {
	h := NewProvider(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/market/stats", nil)
	rec := httptest.NewRecorder()

	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "skill market not configured" {
		t.Fatalf("unexpected nil-market response: %+v", body)
	}
}
