package handlers

import "testing"

func TestGuessBoxType(t *testing.T) {
	cases := map[string]string{
		"mariadb:11.4":                           "mariadb",
		"postgres:16-alpine":                     "postgres",
		"docker.io/library/mysql:8.4":            "mysql",
		"mongo:7":                                "mongodb",
		"redis":                                  "redis",
		"coturn/coturn:4.6.2":                    "",
		"node:20-alpine":                         "",
		"registry.example.com/x/mariadb-tools:1": "",
	}
	for image, want := range cases {
		if got := guessBoxType(image); got != want {
			t.Errorf("guessBoxType(%q) = %q, want %q", image, got, want)
		}
	}
}
