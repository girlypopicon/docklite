package handlers

import "testing"

func TestCurrentSiteType(t *testing.T) {
	cases := []struct{ saved, label, want string }{
		{"node", "static", "static"}, // a rolled-back switch left "node" saved while a static container runs
		{"static", "node", "node"},
		{"php", "", "php"},
		{"static", "postgres", "static"},
	}
	for _, c := range cases {
		if got := currentSiteType(c.saved, c.label); got != c.want {
			t.Errorf("saved %q label %q: got %q want %q", c.saved, c.label, got, c.want)
		}
	}
}
