package handlers

import "testing"

func TestSummarizeIngress(t *testing.T) {
	conf := `
server {
    listen 80 default_server;
    server_name _;
    location / { proxy_pass http://127.0.0.1:3000; }
}
server {
    listen 80;
    server_name a.example.com;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl;
    server_name a.example.com;
    add_header Strict-Transport-Security "max-age=63072000" always;
    location / { proxy_pass http://127.0.0.1:20000; }
}
server {
    listen 80;
    server_name b.example.com;
    location / { proxy_pass http://127.0.0.1:20001; }
}`
	got := summarizeIngress([]string{conf})
	want := ingressSites{Sites: 3, HTTPS: 1, Redirecting: 1, HSTS: 1}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if describeCount(1, 3) != "1 of 3" || describeCount(3, 3) != "all 3" || describeCount(0, 3) != "none" || describeCount(0, 0) != "no sites yet" {
		t.Error("describeCount wording changed")
	}
}
