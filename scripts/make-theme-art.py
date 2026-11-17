#!/usr/bin/env python3
"""Draws the background art for the Vapor, Matrices and Girlypop themes into webapp/public/themes/.
Run it again after changing a drawing:  python3 scripts/make-theme-art.py"""
import os, random
OUT = os.path.join(os.path.dirname(__file__), '..', 'webapp', 'public', 'themes')
os.makedirs(OUT, exist_ok=True)
def w(name, s):
    open(os.path.join(OUT, name), 'w').write(s)

# ── Vapor: striped sunset over a neon grid, palm trees, an iced tea can ───────────────────────────────────────────────
random.seed(7)
stars = ''.join(f'<circle cx="{random.randint(0,1600)}" cy="{random.randint(0,420)}" r="{random.choice([1,1.5,2])}" fill="#fff" opacity="{random.choice([.4,.6,.9])}"/>' for _ in range(70))
bands = ''.join(f'<rect x="560" y="{y}" width="480" height="{h}" fill="#000"/>' for y, h in [(560,6),(590,9),(618,13),(646,17),(676,22)])
vlines = ''.join(f'<line x1="800" y1="700" x2="{x}" y2="900" />' for x in range(-1400, 3000, 160))
hlines = ''.join(f'<line x1="0" y1="{y}" x2="1600" y2="{y}" />' for y in [712, 728, 748, 774, 808, 852])
w('vapor-scene.svg', f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1600 900" preserveAspectRatio="xMidYMax slice">
<defs>
 <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#0d0426"/><stop offset=".5" stop-color="#3a1070"/><stop offset=".82" stop-color="#c2308f"/><stop offset="1" stop-color="#ff7a9c"/></linearGradient>
 <linearGradient id="sun" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ffe66d"/><stop offset=".55" stop-color="#ff8a5c"/><stop offset="1" stop-color="#ff3fa4"/></linearGradient>
 <mask id="cut"><rect width="1600" height="900" fill="#fff"/>{bands}</mask>
 <linearGradient id="floor" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#2a0a52"/><stop offset="1" stop-color="#0d0426"/></linearGradient>
 <path id="leaf" d="M0 0 Q70 -62 150 -8 Q80 -34 0 8 Z"/>
 <g id="palm"><path d="M0 330 Q40 170 -6 0" fill="none" stroke="#08011a" stroke-width="15" stroke-linecap="round"/>
  <g fill="#08011a"><use href="#leaf" transform="rotate(-170)"/><use href="#leaf" transform="rotate(-135)"/><use href="#leaf" transform="rotate(-100)"/><use href="#leaf" transform="rotate(-55)"/><use href="#leaf" transform="rotate(-15)"/><use href="#leaf" transform="rotate(25)"/><use href="#leaf" transform="rotate(60)"/><use href="#leaf" transform="rotate(150) scale(.8)"/></g></g>
</defs>
<rect width="1600" height="900" fill="url(#sky)"/>{stars}
<circle cx="800" cy="560" r="235" fill="url(#sun)" mask="url(#cut)"/>
<rect y="700" width="1600" height="200" fill="url(#floor)"/>
<g stroke="#ff4fd8" stroke-width="2" opacity=".7">{vlines}{hlines}</g>
<line x1="0" y1="700" x2="1600" y2="700" stroke="#7ff3ff" stroke-width="3"/>
<use href="#palm" transform="translate(190 470)"/><use href="#palm" transform="translate(1440 500) scale(-1.15 1.15)"/>
<use href="#palm" transform="translate(320 560) scale(.55)" opacity=".85"/>
<g transform="translate(1160 640) rotate(-6)"><rect width="58" height="130" rx="10" fill="#0a5e66" stroke="#7ff3ff" stroke-width="3"/><rect y="30" width="58" height="62" fill="#07343a"/>
 <path d="M6 61l11-14 11 14-11 14zM30 61l11-14 11 14-11 14z" fill="#ff4fd8"/><path d="M0 100h58" stroke="#ffe66d" stroke-width="4"/><ellipse cx="29" cy="6" rx="22" ry="5" fill="#9ef6ff"/></g>
</svg>''')

# Southwestern iced tea can pattern: black strip with turquoise and pink diamonds and zigzags, tiles sideways.
w('vapor-tea.svg', '''<svg xmlns="http://www.w3.org/2000/svg" width="96" height="24" viewBox="0 0 96 24">
<rect width="96" height="24" fill="#07101c"/>
<path d="M0 3h96M0 21h96" stroke="#ffe66d" stroke-width="2"/>
<path d="M0 12l8-6 8 6-8 6zM32 12l8-6 8 6-8 6zM64 12l8-6 8 6-8 6z" fill="#18c9c0"/>
<path d="M16 12l8-6 8 6-8 6zM48 12l8-6 8 6-8 6zM80 12l8-6 8 6-8 6z" fill="#ff4fb0"/>
<path d="M8 12l4-3 4 3-4 3zM24 12l4-3 4 3-4 3zM40 12l4-3 4 3-4 3zM56 12l4-3 4 3-4 3zM72 12l4-3 4 3-4 3zM88 12l4-3 4 3-4 3z" fill="#07101c"/>
</svg>''')

# ── Matrices: two tiles of faint falling code, scrolled at different speeds ───────────────────────────────────────────
def rain(name, cols, rows, colw, step, seed):
    random.seed(seed)
    glyphs = '01ABCDEFGHJKLMNPRSTUVWXYZ#$%&*+=<>:;|/\\\\'.replace('\\\\', '\\')
    texts = []
    for c in range(cols):
        x = c * colw + colw // 2
        start = random.randint(0, rows - 1)
        run = random.randint(8, 18)
        for i in range(run):
            y = ((start + i) % rows) * step + step
            op = round(0.08 + 0.55 * (i / run) ** 2, 2)
            g = random.choice(glyphs).replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;')
            fill = '#d6ffe6' if i == run - 1 else '#00ff6a'
            texts.append(f'<text x="{x}" y="{y}" fill="{fill}" opacity="{op}">{g}</text>')
    w(name, f'<svg xmlns="http://www.w3.org/2000/svg" width="{cols*colw}" height="{rows*step}" font-family="monospace" font-size="{step-4}" text-anchor="middle">{"".join(texts)}</svg>')
rain('matrix-a.svg', 12, 25, 40, 28, 3)
rain('matrix-b.svg', 9, 20, 64, 36, 11)

# ── Girlypop: hearts, sparkles and little stars in lots of pinks ──────────────────────────────────────────────────────
def heart(x, y, s, fill, op, rot=0):
    return f'<path transform="translate({x} {y}) rotate({rot}) scale({s})" d="M0 9 C-14 -1 -9 -13 -3 -13 C-1 -13 0 -11 0 -9 C0 -11 1 -13 3 -13 C9 -13 14 -1 0 9Z" fill="{fill}" opacity="{op}"/>'
def sparkle(x, y, s, fill, op):
    return f'<path transform="translate({x} {y}) scale({s})" d="M0 -10 Q1 -1 10 0 Q1 1 0 10 Q-1 1 -10 0 Q-1 -1 0 -10Z" fill="{fill}" opacity="{op}"/>'
random.seed(21)
pinks = ['#ff4fc3', '#ff8fd8', '#ffc2ee', '#ff2e9a', '#ff75c8', '#ffe0f6']
items = []
for _ in range(26):
    x, y = random.randint(10, 390), random.randint(10, 390)
    items.append(heart(x, y, random.choice([.8, 1.1, 1.5, 2]), random.choice(pinks), random.choice([.18, .25, .35]), random.randint(-25, 25)))
for _ in range(14):
    items.append(sparkle(random.randint(10, 390), random.randint(10, 390), random.choice([.7, 1, 1.4]), random.choice(['#ffffff', '#ffe0f6', '#fff6a8']), random.choice([.3, .5, .7])))
w('girlypop-hearts.svg', f'<svg xmlns="http://www.w3.org/2000/svg" width="400" height="400" viewBox="0 0 400 400">{"".join(items)}</svg>')
w('girlypop-corner.svg', f'<svg xmlns="http://www.w3.org/2000/svg" width="64" height="30" viewBox="0 0 64 30">{heart(14,15,1,"#ff4fc3",1,-12)}{sparkle(36,9,.8,"#fff6a8",1)}{heart(50,17,.7,"#ffc2ee",1,10)}{sparkle(26,22,.5,"#ffffff",.9)}</svg>')
print('themes art written to', os.path.abspath(OUT))
