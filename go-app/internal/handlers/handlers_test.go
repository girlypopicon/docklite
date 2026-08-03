package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"

	"github.com/docker/go-connections/nat"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestHealth(t *testing.T) {
	t.Run("returns ok status", func(t *testing.T) {
		h := &Handlers{}

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		h.Health(rec, req)

		testhelpers.AssertEqual(t, http.StatusOK, rec.Code)

		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "ok", response["status"])
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		h := &Handlers{}

		methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		for _, method := range methods {
			req := httptest.NewRequest(method, "/health", nil)
			rec := httptest.NewRecorder()

			h.Health(rec, req)

			testhelpers.AssertEqual(t, http.StatusMethodNotAllowed, rec.Code)
		}
	})
}

func TestWriteJSON(t *testing.T) {
	t.Run("writes JSON response", func(t *testing.T) {
		rec := httptest.NewRecorder()

		writeJSON(rec, http.StatusOK, map[string]string{"message": "hello"})

		testhelpers.AssertEqual(t, http.StatusOK, rec.Code)
		testhelpers.AssertEqual(t, "application/json", rec.Header().Get("Content-Type"))

		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "hello", response["message"])
	})
}

func TestWriteError(t *testing.T) {
	t.Run("writes error response", func(t *testing.T) {
		rec := httptest.NewRecorder()

		writeError(rec, http.StatusBadRequest, "invalid request")

		testhelpers.AssertEqual(t, http.StatusBadRequest, rec.Code)
		testhelpers.AssertEqual(t, "application/json", rec.Header().Get("Content-Type"))

		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "invalid request", response["error"])
	})
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minute", 90 * time.Second, "1m"},
		{"minutes", 150 * time.Second, "2m"},
		{"hour", 90 * time.Minute, "1h 30m"},
		{"hours", 150 * time.Minute, "2h 30m"},
		{"day", 30 * time.Hour, "1d 6h"},
		{"days", 50 * time.Hour, "2d 2h"},
		{"zero", 0, "0s"},
		{"negative", -10 * time.Second, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUptime(tt.duration)
			testhelpers.AssertEqual(t, tt.expected, result)
		})
	}
}

func TestFormatPortsFromInspect(t *testing.T) {
	t.Run("formats single port", func(t *testing.T) {
		ports := nat.PortMap{
			"80/tcp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: "8080"},
			},
		}

		result := formatPortsFromInspect(ports)
		testhelpers.AssertEqual(t, "8080->80", result)
	})

	t.Run("handles nil ports", func(t *testing.T) {
		result := formatPortsFromInspect(nil)
		testhelpers.AssertEqual(t, "-", result)
	})

	t.Run("handles empty ports", func(t *testing.T) {
		ports := nat.PortMap{}
		result := formatPortsFromInspect(ports)
		testhelpers.AssertEqual(t, "-", result)
	})
}

func TestStaticTemplate(t *testing.T) {
	t.Run("generates HTML with domain and path", func(t *testing.T) {
		result := staticTemplate("/var/www/testuser/example.com", "example.com")

		testhelpers.AssertTrue(t, len(result) > 0, "Should generate content")
		testhelpers.AssertTrue(t, contains(result, "example.com"), "Should contain domain")
		testhelpers.AssertTrue(t, contains(result, "DockLite"), "Should mention DockLite")
	})
}

func TestPhpTemplate(t *testing.T) {
	t.Run("generates PHP with domain and path", func(t *testing.T) {
		result := phpTemplate("/var/www/testuser/phpsite.com", "phpsite.com")

		testhelpers.AssertTrue(t, len(result) > 0, "Should generate content")
		testhelpers.AssertTrue(t, contains(result, "<?php"), "Should contain PHP opening tag")
		testhelpers.AssertTrue(t, contains(result, "phpsite.com"), "Should contain domain")
	})
}

func TestNodePackageJSON(t *testing.T) {
	t.Run("generates valid JSON", func(t *testing.T) {
		result := nodePackageJSON("my-node-site.com")

		testhelpers.AssertTrue(t, len(result) > 0, "Should generate content")

		var pkg map[string]interface{}
		err := json.Unmarshal([]byte(result), &pkg)
		testhelpers.AssertNoError(t, err)

		testhelpers.AssertEqual(t, "my-node-site-com", pkg["name"])
		testhelpers.AssertEqual(t, "1.0.0", pkg["version"])
	})
}

func TestNodeTemplate(t *testing.T) {
	t.Run("generates Node.js server code", func(t *testing.T) {
		result := nodeTemplate("/var/www/testuser/nodeapp.com", "nodeapp.com")

		testhelpers.AssertTrue(t, len(result) > 0, "Should generate content")
		testhelpers.AssertTrue(t, contains(result, "require('http')"), "Should require http module")
		testhelpers.AssertTrue(t, contains(result, "nodeapp.com"), "Should contain domain")
		testhelpers.AssertTrue(t, contains(result, "createServer"), "Should create server")
	})
}

func TestGetSitePath(t *testing.T) {
	t.Run("constructs site path", func(t *testing.T) {
		result := getSitePath("testuser", "example.com")
		testhelpers.AssertEqual(t, "/var/www/sites/testuser/example.com", result)
	})

	t.Run("handles subdomains", func(t *testing.T) {
		result := getSitePath("admin", "sub.example.com")
		testhelpers.AssertEqual(t, "/var/www/sites/admin/sub.example.com", result)
	})
}

func TestSortByPosition(t *testing.T) {
	t.Run("sorts nodes by position", func(t *testing.T) {
		nodes := []*folderNode{
			{Folder: store.Folder{Position: 2}},
			{Folder: store.Folder{Position: 0}},
			{Folder: store.Folder{Position: 1}},
		}

		sortByPosition(nodes)

		testhelpers.AssertEqual(t, 0, nodes[0].Position)
		testhelpers.AssertEqual(t, 1, nodes[1].Position)
		testhelpers.AssertEqual(t, 2, nodes[2].Position)
	})
}
