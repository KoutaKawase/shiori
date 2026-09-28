package domain

import "testing"

func TestValidateURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"https ok", "https://example.com", false},
		{"http ok", "http://example.com/path?q=1", false},
		{"empty", "", true},
		{"spaces", "   ", true},
		{"ftp rejected", "ftp://example.com", true},
		{"no scheme", "example.com", true},
		{"no host", "https://", true},
		{"trims spaces", "  https://example.com  ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ValidateURL(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateURL(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"Go", "go", false},
		{"  Docker  ", "docker", false},
		{"my-tag_1", "my-tag_1", false},
		{"", "", true},
		{"a/b", "", true},
		{"a:b", "", true},
		{"a.b", "", true},
		{"日本語", "", true},
		{"with space", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeTag(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("NormalizeTag(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
		}
		if err == nil && got != c.want {
			t.Fatalf("NormalizeTag(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeTagsDedup(t *testing.T) {
	got, err := NormalizeTags([]string{"Go", "go", "  GO "})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "go" {
		t.Fatalf("got %v", got)
	}
}

func TestValidateLimitOffset(t *testing.T) {
	l, o, err := ValidateLimitOffset(0, 0)
	if err != nil || l != DefaultLimit || o != 0 {
		t.Fatalf("got %d %d %v", l, o, err)
	}
	if _, _, err := ValidateLimitOffset(1000, 0); err == nil {
		t.Fatal("expected limit error")
	}
	if _, _, err := ValidateLimitOffset(10, -1); err == nil {
		t.Fatal("expected offset error")
	}
}

func TestValidateFTSQuery(t *testing.T) {
	if _, err := ValidateFTSQuery("   "); err == nil {
		t.Fatal("expected empty error")
	}
	if _, err := ValidateFTSQuery("docker"); err != nil {
		t.Fatal(err)
	}
}
