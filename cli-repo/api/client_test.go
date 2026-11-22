package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetContainersReadsBoxesAndGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/containers" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"folders":[],"boxes":[{"name":"a","labels":{}},{"name":"b","labels":{}}],"groups":[{"id":1,"name":"G","members":["a","b"]}]}`))
	}))
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	got, err := c.GetContainers()
	if err != nil || len(got.Containers) != 2 || len(got.Groups) != 1 || got.Groups[0].Members[1] != "b" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestGetContainersFallsBackOnOldAgents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers" {
			w.Write([]byte(`{"folders":[],"totalContainers":1}`))
			return
		}
		w.Write([]byte(`{"containers":[{"name":"old","labels":{}}]}`))
	}))
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	got, err := c.GetContainers()
	if err != nil || len(got.Containers) != 1 || got.Containers[0].Name != "old" {
		t.Fatalf("got %+v err %v", got, err)
	}
}
