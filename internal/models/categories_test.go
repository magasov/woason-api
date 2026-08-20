package models

import "testing"

func TestGoodsCategoriesContract(t *testing.T) {
	if len(Categories) != 22 {
		t.Fatalf("want 22 goods categories, got %d", len(Categories))
	}
	seen := map[string]bool{}
	for _, c := range Categories {
		if c.Slug == "" || c.Name == "" || c.Group == "" {
			t.Fatalf("incomplete category: %+v", c)
		}
		if seen[c.Slug] {
			t.Fatalf("duplicate slug %s", c.Slug)
		}
		seen[c.Slug] = true
		if IsBannedCategory(c.Slug) {
			t.Fatalf("goods category marked banned: %s", c.Slug)
		}
		if !IsGoodsCategory(c.Slug) {
			t.Fatalf("slug not indexed: %s", c.Slug)
		}
	}
	for _, banned := range []string{"avto", "nedvizhimost", "transport", "uslugi", "rabota", "travel"} {
		if IsGoodsCategory(banned) {
			t.Fatalf("%s must not be a goods category", banned)
		}
		if !IsBannedCategory(banned) {
			t.Fatalf("%s must be banned", banned)
		}
	}
	if IsGoodsCategory("AVTO") || IsGoodsCategory("  Odezhda ") == false {
		t.Fatal("normalization failed")
	}
}
