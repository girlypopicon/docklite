// Theme contrast audit: loads pages of a RUNNING demo (scripts/demo.sh up) in each theme, reads every visible piece of text with
// its real colour, finds the real background from a screenshot, and reports text below WCAG contrast (4.5, or 3 for large text).
//
//   node scripts/contrast-audit.js [themes] [pages]
//   node scripts/contrast-audit.js cyberpunk,corpo,corpo-blue,unicorn "/,/databases,/backups"
//
// Needs playwright-core (npm i playwright-core somewhere) and a Chromium: set CHROME_PATH to a chrome-headless-shell binary.
// The demo login password is read from ~/docklite-demo/.demo-password.
const { chromium } = require(process.env.PLAYWRIGHT_CORE || 'playwright-core'); const fs = require('fs'); const H = process.env.HOME;
const THEMES = (process.argv[2] || 'cyberpunk,corpo,corpo-blue,unicorn').split(',');
const PAGES = (process.argv[3] || '/,/databases,/backups,/settings/users,/settings/cloudflare').split(',');
const OUT = process.argv[4] || null;
(async () => {
  const b = await chromium.launch({ headless: true, args: ['--no-sandbox'], executablePath: process.env.CHROME_PATH });
  const pass = fs.readFileSync(H + '/docklite-demo/.demo-password', 'utf8').trim();
  const summary = {};
  for (const theme of THEMES) {
    const ctx = await b.newContext({ viewport: { width: 1600, height: 1000 } });
    await ctx.addInitScript((t) => { try { localStorage.setItem('docklite-theme', t); } catch (e) {} }, theme);
    const page = await ctx.newPage();
    await page.goto('http://127.0.0.1:3100/login'); await page.fill('input[type=text]', 'demo'); await page.fill('input[type=password]', pass);
    await page.keyboard.press('Enter'); await page.waitForTimeout(3500);
    for (const path of PAGES) {
      await page.goto('http://127.0.0.1:3100' + path); await page.waitForTimeout(3000);
      const items = await page.evaluate(() => {
        const out = []; const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
        while (w.nextNode()) {
          const n = w.currentNode; const txt = (n.textContent || '').trim(); if (txt.length < 2) continue;
          const el = n.parentElement; if (!el || ['SCRIPT','STYLE','NOSCRIPT'].includes(el.tagName)) continue;
          const cs = getComputedStyle(el); if (cs.visibility === 'hidden' || cs.display === 'none' || parseFloat(cs.opacity) === 0) continue;
          const r = document.createRange(); r.selectNodeContents(n); const rect = r.getBoundingClientRect();
          if (rect.width < 4 || rect.height < 4 || rect.bottom < 0 || rect.top > innerHeight || rect.right < 0 || rect.left > innerWidth) continue;
          out.push({ text: txt.slice(0, 40), color: cs.color, size: parseFloat(cs.fontSize), weight: parseInt(cs.fontWeight) || 400, op: parseFloat(cs.opacity), x: rect.left, y: rect.top, w: rect.width, h: rect.height });
        }
        return out;
      });
      const shot = (await page.screenshot()).toString('base64');
      const results = await page.evaluate(async ({ items, shot }) => {
        const img = new Image(); img.src = 'data:image/png;base64,' + shot; await img.decode();
        const c = document.createElement('canvas'); c.width = img.width; c.height = img.height; const g = c.getContext('2d', { willReadFrequently: true }); g.drawImage(img, 0, 0);
        const lum = (r, gg, b) => { const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(r) + 0.7152 * f(gg) + 0.0722 * f(b); };
        const parse = (s) => { const m = s.match(/[\d.]+/g).map(Number); return { r: m[0], g: m[1], b: m[2], a: m.length > 3 ? m[3] : 1 }; };
        const res = [];
        for (const it of items) {
          // background = the median colour of a thin ring just outside the text box
          const px = []; const pad = 3;
          const pts = []; for (let i = 0; i <= 8; i++) { const fx = it.x + (it.w * i) / 8; pts.push([fx, it.y - pad], [fx, it.y + it.h + pad]); }
          for (let i = 0; i <= 3; i++) { const fy = it.y + (it.h * i) / 3; pts.push([it.x - pad, fy], [it.x + it.w + pad, fy]); }
          for (const [px_, py] of pts) { const X = Math.round(px_), Y = Math.round(py); if (X < 0 || Y < 0 || X >= c.width || Y >= c.height) continue; const d = g.getImageData(X, Y, 1, 1).data; px.push([d[0], d[1], d[2]]); }
          if (px.length < 4) continue;
          const med = [0, 1, 2].map((k) => px.map((p) => p[k]).sort((a, b) => a - b)[Math.floor(px.length / 2)]);
          const fg = parse(it.color); if (fg.a === 0) continue;
          // blend the text colour with the background by its alpha
          const fr = fg.r * fg.a + med[0] * (1 - fg.a), fgG = fg.g * fg.a + med[1] * (1 - fg.a), fb = fg.b * fg.a + med[2] * (1 - fg.a);
          const L1 = lum(fr, fgG, fb), L2 = lum(med[0], med[1], med[2]); const ratio = (Math.max(L1, L2) + 0.05) / (Math.min(L1, L2) + 0.05);
          const large = it.size >= 24 || (it.size >= 18.66 && it.weight >= 700);
          res.push({ text: it.text, ratio: Math.round(ratio * 100) / 100, need: large ? 3 : 4.5, fg: `rgb(${Math.round(fr)},${Math.round(fgG)},${Math.round(fb)})`, bg: `rgb(${med.join(',')})`, size: it.size });
        }
        return res;
      }, { items, shot });
      const bad = results.filter((r) => r.ratio < r.need);
      summary[`${theme} ${path}`] = { total: results.length, bad: bad.length, worst: bad.sort((a, b) => a.ratio - b.ratio).slice(0, 4) };
    }
    await ctx.close();
  }
  await b.close();
  for (const [k, v] of Object.entries(summary)) {
    console.log(`${k.padEnd(34)} ${String(v.bad).padStart(3)} of ${String(v.total).padStart(3)} below the limit`);
    for (const w of v.worst) console.log(`      ${w.ratio} (needs ${w.need})  "${w.text}"  ${w.fg} on ${w.bg}`);
  }
  if (OUT) fs.writeFileSync(OUT, JSON.stringify(summary, null, 1));
})().catch((e) => { console.error('ERR', e.message.split('\n')[0]); process.exit(1); });
