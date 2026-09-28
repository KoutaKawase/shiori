package app

import "testing"

func TestToFTSMatchQuery(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"docker", `"docker"`, false},
		{"docker compose", `"docker" AND "compose"`, false},
		{`a"b`, `"a""b"`, false},
		{"  ", "", true},
		{"OR", `"OR"`, false},
	}
	for _, c := range cases {
		got, err := ToFTSMatchQuery(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ToFTSMatchQuery(%q) err=%v", c.in, err)
		}
		if err == nil && got != c.want {
			t.Fatalf("ToFTSMatchQuery(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}
