package auth

import "testing"

func TestValidPhotoURLRejectsBlob(t *testing.T) {
	ok := []string{
		"http://localhost:8080/uploads/product/user-1/a.webp",
		"https://example.com/p.jpg",
	}
	bad := []string{
		"blob:http://localhost:3000/abc",
		"data:image/png;base64,aaa",
		"file:///tmp/x.png",
		"",
		"/relative.png",
	}
	for _, u := range ok {
		if !ValidPhotoURL(u) {
			t.Fatalf("want valid: %s", u)
		}
	}
	for _, u := range bad {
		if ValidPhotoURL(u) {
			t.Fatalf("want invalid: %s", u)
		}
	}
}

func TestValidAssetRefKeepsEmojiLogo(t *testing.T) {
	if !ValidAssetRef("🛍️") {
		t.Fatal("emoji logo must stay valid")
	}
	if ValidAssetRef("blob:http://localhost:3000/x") {
		t.Fatal("blob must be rejected")
	}
	if !ValidAssetRef("http://localhost:8080/uploads/banner/user-1/x.webp") {
		t.Fatal("upload url must be valid")
	}
}
