package skillmarket

import (
	"path/filepath"
	"testing"
)

func seedMarket() *Market {
	m := NewMarket("")
	m.Publish(SkillMeta{Name: "web_search", Version: "1.0.0", Description: "Search the web", Author: "core", Category: CatSearch, Tags: []string{"search", "web"}})
	m.Publish(SkillMeta{Name: "code_gen", Version: "2.1.0", Description: "Generate code", Author: "core", Category: CatCoding, Tags: []string{"code", "generate"}})
	m.Publish(SkillMeta{Name: "translate", Version: "1.3.0", Description: "Translate text between languages", Author: "community", Category: CatLanguage, Tags: []string{"translate", "i18n"}})
	m.Publish(SkillMeta{Name: "doc_parse", Version: "1.0.0", Description: "Parse documents", Author: "core", Category: CatData, Tags: []string{"pdf", "docx", "parse"}})
	m.Publish(SkillMeta{Name: "image_gen", Version: "1.0.0", Description: "AI image generation", Author: "community", Category: CatMedia, Tags: []string{"image", "dalle", "ai"}})
	return m
}

func TestPublishAndGet(t *testing.T) {
	m := seedMarket()
	s, ok := m.Get("web_search")
	if !ok {
		t.Fatal("expected web_search")
	}
	if s.Version != "1.0.0" || s.Category != CatSearch {
		t.Error("metadata mismatch")
	}
}

func TestPublishUpdate(t *testing.T) {
	m := seedMarket()
	m.RecordInstall("web_search")
	m.RecordInstall("web_search")
	m.Rate("web_search", 4.5)

	// Update version
	m.Publish(SkillMeta{Name: "web_search", Version: "1.1.0", Description: "Improved search", Author: "core", Category: CatSearch})
	s, _ := m.Get("web_search")
	if s.Version != "1.1.0" {
		t.Error("version not updated")
	}
	if s.Installs != 2 {
		t.Errorf("installs should be preserved, got %d", s.Installs)
	}
	if s.RatingCount != 1 {
		t.Error("ratings should be preserved")
	}
}

func TestPublishValidation(t *testing.T) {
	m := NewMarket("")
	if err := m.Publish(SkillMeta{Version: "1.0.0"}); err == nil {
		t.Error("expected error for empty name")
	}
	if err := m.Publish(SkillMeta{Name: "x"}); err == nil {
		t.Error("expected error for empty version")
	}
}

func TestRecordInstall(t *testing.T) {
	m := seedMarket()
	m.RecordInstall("code_gen")
	m.RecordInstall("code_gen")
	m.RecordInstall("code_gen")
	s, _ := m.Get("code_gen")
	if s.Installs != 3 {
		t.Errorf("expected 3 installs, got %d", s.Installs)
	}
}

func TestRate(t *testing.T) {
	m := seedMarket()
	m.Rate("translate", 5.0)
	m.Rate("translate", 3.0)
	s, _ := m.Get("translate")
	if s.RatingCount != 2 {
		t.Errorf("expected 2 ratings, got %d", s.RatingCount)
	}
	if s.Rating != 4.0 {
		t.Errorf("expected rating 4.0, got %.1f", s.Rating)
	}
}

func TestRateInvalid(t *testing.T) {
	m := seedMarket()
	if err := m.Rate("translate", 6.0); err == nil {
		t.Error("expected error for score > 5")
	}
	if err := m.Rate("nonexistent", 3.0); err == nil {
		t.Error("expected error for unknown skill")
	}
}

func TestSearch(t *testing.T) {
	m := seedMarket()
	results := m.Search("search")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'search', got %d", len(results))
	}

	results = m.Search("code")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'code', got %d", len(results))
	}

	// Search by tag
	results = m.Search("pdf")
	if len(results) != 1 {
		t.Errorf("expected 1 result for tag 'pdf', got %d", len(results))
	}
}

func TestSearchSkipsDeprecated(t *testing.T) {
	m := seedMarket()
	m.Deprecate("web_search")
	results := m.Search("search")
	if len(results) != 0 {
		t.Errorf("deprecated skill should not appear in search, got %d", len(results))
	}
}

func TestFindByCategory(t *testing.T) {
	m := seedMarket()
	coding := m.FindByCategory(CatCoding)
	if len(coding) != 1 {
		t.Errorf("expected 1 coding skill, got %d", len(coding))
	}
}

func TestFindByTag(t *testing.T) {
	m := seedMarket()
	results := m.FindByTag("ai")
	if len(results) != 1 {
		t.Errorf("expected 1 AI-tagged skill, got %d", len(results))
	}
}

func TestTopRated(t *testing.T) {
	m := seedMarket()
	m.Rate("web_search", 4.0)
	m.Rate("code_gen", 5.0)
	m.Rate("translate", 3.0)

	top := m.TopRated(2)
	if len(top) != 2 {
		t.Fatalf("expected 2 top rated, got %d", len(top))
	}
	if top[0].Name != "code_gen" {
		t.Errorf("expected code_gen first, got %s", top[0].Name)
	}
}

func TestMostPopular(t *testing.T) {
	m := seedMarket()
	for i := 0; i < 10; i++ {
		m.RecordInstall("translate")
	}
	for i := 0; i < 5; i++ {
		m.RecordInstall("web_search")
	}

	popular := m.MostPopular(2)
	if len(popular) != 2 {
		t.Fatalf("expected 2, got %d", len(popular))
	}
	if popular[0].Name != "translate" {
		t.Errorf("expected translate first, got %s", popular[0].Name)
	}
}

func TestDeprecateAndRemove(t *testing.T) {
	m := seedMarket()
	if !m.Deprecate("doc_parse") {
		t.Error("expected true for deprecate")
	}
	s, _ := m.Get("doc_parse")
	if !s.Deprecated {
		t.Error("expected deprecated=true")
	}
	all := m.All()
	for _, s := range all {
		if s.Name == "doc_parse" {
			t.Error("deprecated should not appear in All()")
		}
	}

	if !m.Remove("doc_parse") {
		t.Error("expected true for remove")
	}
	if m.Remove("nonexistent") {
		t.Error("expected false for nonexistent")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "market.json")

	m1 := seedMarket()
	m1.RecordInstall("web_search")
	m1.Rate("web_search", 4.5)
	if err := m1.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	m2 := NewMarket("")
	if err := m2.LoadFrom(path); err != nil {
		t.Fatal(err)
	}
	if m2.Count() != m1.Count() {
		t.Errorf("count mismatch: %d vs %d", m2.Count(), m1.Count())
	}
	s, ok := m2.Get("web_search")
	if !ok {
		t.Fatal("web_search not found after reload")
	}
	if s.Installs != 1 {
		t.Errorf("expected 1 install after reload, got %d", s.Installs)
	}
}

func TestLoadNonexistent(t *testing.T) {
	m := NewMarket("")
	if err := m.LoadFrom("/nonexistent/path.json"); err != nil {
		t.Error("expected nil error for nonexistent file")
	}
}

func TestStats(t *testing.T) {
	m := seedMarket()
	stats := m.Stats()
	if stats["total"].(int) != 5 {
		t.Errorf("expected 5 total, got %v", stats["total"])
	}
	cats := stats["categories"].(map[Category]int)
	if cats[CatSearch] != 1 {
		t.Error("expected 1 search category")
	}
}
