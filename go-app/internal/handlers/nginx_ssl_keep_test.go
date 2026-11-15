package handlers

import (
	"strings"
	"testing"

	"docklite-agent/internal/testhelpers"
)

// certbot --nginx writes the HTTPS block into the site's own vhost file. Re-pointing the site at a new container must
// change the port in every block and keep everything else, and a vhost already on the right port is not an error.
func TestSetProxyPortKeepsCertbotBlocks(t *testing.T) {
	vhost := nginxVhostConfig("shop.example.com", true, 32001) + `
server {
    listen 443 ssl; # managed by Certbot
    server_name shop.example.com www.shop.example.com;
    ssl_certificate /etc/letsencrypt/live/shop.example.com/fullchain.pem; # managed by Certbot
    ssl_certificate_key /etc/letsencrypt/live/shop.example.com/privkey.pem; # managed by Certbot
    location / {
        proxy_pass http://127.0.0.1:32001;
    }
}
`
	testhelpers.AssertEqual(t, nginxConfigHasSSL(vhost), true)
	testhelpers.AssertEqual(t, nginxConfigHasSSL(nginxVhostConfig("shop.example.com", true, 32001)), false)

	updated, found := setProxyPort(vhost, 32005)
	testhelpers.AssertEqual(t, found, true)
	testhelpers.AssertEqual(t, strings.Count(updated, "proxy_pass http://127.0.0.1:32005"), 2)
	testhelpers.AssertEqual(t, strings.Contains(updated, ":32001"), false)
	if !strings.Contains(updated, "ssl_certificate_key /etc/letsencrypt/live/shop.example.com/privkey.pem") || !strings.Contains(updated, "listen 443 ssl") {
		t.Fatal("the HTTPS block must survive a port change")
	}

	same, found := setProxyPort(vhost, 32001)
	testhelpers.AssertEqual(t, found, true)
	testhelpers.AssertEqual(t, same, vhost) // same port: nothing to write, and (in updateNginxProxyPort) not an error

	_, found = setProxyPort("server { listen 80; root /var/www/html; }", 1)
	testhelpers.AssertEqual(t, found, false)

	local, _ := setProxyPort("location / { proxy_pass http://localhost:3000; }", 4000)
	testhelpers.AssertEqual(t, local, "location / { proxy_pass http://localhost:4000; }")
}
