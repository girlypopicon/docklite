package handlers

import (
	"encoding/json"
	"html"
	"strings"
)

// The page a brand-new site shows until its owner puts something there. Visitors see it, so it names no server paths
// and loads nothing from elsewhere (no fonts, scripts or images from other sites): one self-contained file in DockLite's colours.
const starterPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{DOMAIN}}</title>
<style>
  :root {
    --pink: #FF6AD5; --coral: #FF9A8B; --gold: #FFD166; --mint: #B8F2A2; --sky: #9AD0FF; --lilac: #C7A4FF;
    --rainbow: linear-gradient(110deg, var(--pink), var(--coral), var(--gold), var(--mint), var(--sky), var(--lilac));
  }
  * { box-sizing: border-box; }
  html, body { height: 100%; margin: 0; }
  body {
    display: flex; align-items: center; justify-content: center; padding: 24px;
    font-family: ui-monospace, "SFMono-Regular", Menlo, Consolas, "Liberation Mono", monospace;
    color: #e9e4f5;
    background:
      radial-gradient(60rem 40rem at 15% 10%, rgba(255,106,213,.18), transparent 60%),
      radial-gradient(50rem 40rem at 90% 90%, rgba(154,208,255,.16), transparent 60%),
      linear-gradient(160deg, #0b0b16 0%, #140b24 55%, #0b1426 100%);
  }
  .card {
    width: 100%; max-width: 640px; padding: 40px 36px; border-radius: 22px; position: relative;
    background: rgba(22, 14, 34, .72); backdrop-filter: blur(14px); -webkit-backdrop-filter: blur(14px);
    box-shadow: 0 20px 60px rgba(0,0,0,.55), 0 0 40px rgba(199,164,255,.18);
  }
  .card::before {              /* a thin rainbow edge */
    content: ""; position: absolute; inset: 0; border-radius: inherit; padding: 1.5px; background: var(--rainbow);
    -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); -webkit-mask-composite: xor;
    mask: linear-gradient(#000 0 0) content-box exclude, linear-gradient(#000 0 0); opacity: .7; pointer-events: none;
  }
  .live { display: inline-flex; align-items: center; gap: 8px; font-size: 12px; letter-spacing: .18em; text-transform: uppercase; color: var(--mint); }
  .live i { width: 9px; height: 9px; border-radius: 50%; background: var(--mint); box-shadow: 0 0 10px var(--mint); animation: pulse 2.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: .35; } }
  h1 {
    margin: 18px 0 14px; font-size: clamp(26px, 6vw, 42px); line-height: 1.15; word-break: break-word;
    background: var(--rainbow); -webkit-background-clip: text; background-clip: text; color: transparent;
    filter: drop-shadow(0 0 14px rgba(255,106,213,.35));
  }
  .lead { margin: 0 0 26px; color: #c9c1de; font-size: 15px; line-height: 1.6; }
  ol { margin: 0 0 28px; padding: 0; list-style: none; counter-reset: s; display: grid; gap: 10px; }
  li { counter-increment: s; display: flex; gap: 12px; align-items: baseline; font-size: 14px; line-height: 1.5; color: #d9d2ec; }
  li::before {
    content: counter(s); flex: none; width: 24px; height: 24px; border-radius: 50%; display: inline-grid; place-items: center;
    font-size: 12px; font-weight: 700; color: #160a1f; background: var(--rainbow);
  }
  code { padding: 1px 7px; border-radius: 6px; color: var(--sky); background: rgba(154,208,255,.12); }
  footer { padding-top: 18px; border-top: 1px solid rgba(199,164,255,.2); font-size: 12px; color: #9c93b4; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
  footer a { color: var(--lilac); text-decoration: none; }
  footer a:hover { text-decoration: underline; }
  @media (prefers-reduced-motion: reduce) { .live i { animation: none; } }
</style>
</head>
<body>
<main class="card">
  <span class="live"><i></i> Online</span>
  <h1>{{DOMAIN}}</h1>
  <p class="lead">This site is set up and waiting for its first page. If it's yours, here's how to put something here:</p>
  <ol>
    <li><span>Open DockLite and go to this site's files.</span></li>
    <li><span>Replace <code>{{FILE}}</code> with your own page, or upload your whole site.</span></li>
    <li><span>Refresh this page. That's it.</span></li>
  </ol>
  <footer><span>{{KIND}}</span><span>Hosted with <a href="https://docklite.net" rel="noopener">DockLite</a></span></footer>
</main>
</body>
</html>
`

// starterPage fills in the shared page. domain is escaped; kind is a short line for the footer.
func starterPage(domain, file, kind string) string {
	r := strings.NewReplacer("{{DOMAIN}}", html.EscapeString(domain), "{{FILE}}", html.EscapeString(file), "{{KIND}}", html.EscapeString(kind))
	return r.Replace(starterPageHTML)
}

// The site path is accepted for the callers' sake but never printed: the page is public.
func staticTemplate(_ string, domain string) string {
	return starterPage(domain, "index.html", "Static site")
}

func phpTemplate(_ string, domain string) string {
	page := starterPage(domain, "index.php", "PHP site")
	// show that PHP itself is running: the footer's first label becomes "PHP x.y.z"
	return strings.Replace(page, "<span>PHP site</span>", "<span>PHP <?php echo htmlspecialchars(PHP_VERSION); ?></span>", 1)
}

// nodeTemplate serves the same page from a tiny server. The page is embedded as a JSON string, so nothing in it can
// break out of the string.
func nodeTemplate(_ string, domain string) string {
	encoded, _ := json.Marshal(starterPage(domain, "index.js", "Node.js site"))
	return `const http = require('http');

const hostname = '0.0.0.0';
const port = process.env.PORT || 3000;
const page = ` + string(encoded) + `;

const server = http.createServer((req, res) => {
  res.statusCode = 200;
  res.setHeader('Content-Type', 'text/html; charset=utf-8');
  res.end(page);
});

server.listen(port, hostname, () => {
  console.log('Server running on port ' + port);
});
`
}
