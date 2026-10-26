// Appearance settings (theme, text size, animations, glow), kept in
// localStorage and applied as attributes/variables on <html>. Everything in
// the app that reacts to them keys off those, so changes show up instantly
// and survive reloads. The inline script in app/layout.tsx mirrors
// applyAppearance() so the saved look is in place before first paint.
// The lasting copy is saved per user by the agent (/api/auth/appearance), so
// it follows them to other browsers and survives cleared storage and updates.

export type ThemeId = 'cyberpunk' | 'corpo' | 'corpo-blue' | 'unicorn';
export type FontSizeId = 'small' | 'medium' | 'large' | 'xlarge';

export interface Appearance {
  theme: ThemeId;
  fontSize: FontSizeId;
  animations: boolean;
  glow: number; // percent, 0–200
}

export const DEFAULT_APPEARANCE: Appearance = {
  theme: 'cyberpunk',
  fontSize: 'medium',
  animations: true,
  glow: 0,
};

// Root font-size as a share of the browser default, so "medium" leaves the
// app exactly as it was before this setting existed.
export const FONT_SCALES: Record<FontSizeId, number> = {
  small: 87.5,
  medium: 100,
  large: 112.5,
  xlarge: 125,
};

const KEYS = {
  theme: 'docklite-theme',
  fontSize: 'docklite-font-size',
  animations: 'docklite-animations',
  glow: 'docklite-neon-intensity',
} as const;

const THEMES: ThemeId[] = ['cyberpunk', 'corpo', 'corpo-blue', 'unicorn'];

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function loadAppearance(): Appearance {
  let theme = read(KEYS.theme) || DEFAULT_APPEARANCE.theme;
  if (theme === 'new') theme = 'unicorn'; // old name for the unicorn theme

  const font = read(KEYS.fontSize);
  const glow = Number(read(KEYS.glow));
  const storedAnimations = read(KEYS.animations);

  // Animations stay on unless they were explicitly turned off in Settings.
  const animations = storedAnimations === null ? DEFAULT_APPEARANCE.animations : storedAnimations === 'true';

  return {
    theme: (THEMES as string[]).includes(theme) ? (theme as ThemeId) : DEFAULT_APPEARANCE.theme,
    fontSize: font && font in FONT_SCALES ? (font as FontSizeId) : DEFAULT_APPEARANCE.fontSize,
    animations,
    glow: read(KEYS.glow) !== null && Number.isFinite(glow) ? Math.min(200, Math.max(0, glow)) : DEFAULT_APPEARANCE.glow,
  };
}

/** Fired on window when the saved look arrives from the server, so open settings can show it. */
export const APPEARANCE_EVENT = 'docklite-appearance';

let pushTimer: ReturnType<typeof setTimeout> | undefined;

function pushAppearance(appearance: Appearance) {
  // The glow slider fires on every step: send only where it settles.
  clearTimeout(pushTimer);
  pushTimer = setTimeout(() => {
    fetch('/api/auth/appearance', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(appearance),
    }).catch(() => {
      /* offline or restarting: this browser keeps it, and the next change sends it */
    });
  }, 500);
}

export function saveAppearance(appearance: Appearance, { remote = true } = {}) {
  try {
    localStorage.setItem(KEYS.theme, appearance.theme);
    localStorage.setItem(KEYS.fontSize, appearance.fontSize);
    localStorage.setItem(KEYS.animations, String(appearance.animations));
    localStorage.setItem(KEYS.glow, String(appearance.glow));
  } catch {
    /* private mode etc.: the look still applies for this visit */
  }
  if (remote) pushAppearance(appearance);
}

/**
 * Brings in the signed-in user's saved look and applies it. A user who has
 * never saved one on the server gets this browser's choice saved for them,
 * so it lasts from now on.
 */
export async function syncAppearance(): Promise<void> {
  try {
    const res = await fetch('/api/auth/appearance', { cache: 'no-store' });
    if (!res.ok) return;
    const data = await res.json();
    const saved = data?.appearance;
    if (!saved) {
      if (read(KEYS.theme) !== null) pushAppearance(loadAppearance());
      return;
    }
    const next: Appearance = {
      theme: (THEMES as string[]).includes(saved.theme) ? saved.theme : DEFAULT_APPEARANCE.theme,
      fontSize: saved.fontSize in FONT_SCALES ? saved.fontSize : DEFAULT_APPEARANCE.fontSize,
      animations: saved.animations !== false,
      glow: Number.isFinite(saved.glow) ? Math.min(200, Math.max(0, saved.glow)) : DEFAULT_APPEARANCE.glow,
    };
    saveAppearance(next, { remote: false });
    applyAppearance(next);
    window.dispatchEvent(new CustomEvent(APPEARANCE_EVENT, { detail: next }));
  } catch {
    /* the agent is restarting, or this is the demo: keep this browser's look */
  }
}

export function applyAppearance(appearance: Appearance) {
  const root = document.documentElement;
  root.setAttribute('data-theme', appearance.theme);
  root.setAttribute('data-animations', appearance.animations ? 'on' : 'off');
  root.style.fontSize = `${FONT_SCALES[appearance.fontSize]}%`;
  // glow 0 = how DockLite looks by default; 200 = the strong neon look. --glow scales text/tab glows (1x to 2x);
  // --glow-boost (0 to 1) adds the big halo around cards.
  const boost = Math.min(200, Math.max(0, appearance.glow)) / 200;
  root.style.setProperty('--glow', String(1 + boost));
  root.style.setProperty('--glow-boost', String(boost));
}

/** Source of the pre-paint script: same reading rules as loadAppearance(). */
export const APPEARANCE_BOOT_SCRIPT = `(function(){try{
var g=function(k){try{return localStorage.getItem(k)}catch(e){return null}};
var r=document.documentElement,t=g('docklite-theme')||'cyberpunk';
if(t==='new')t='unicorn';
var T=['cyberpunk','corpo','corpo-blue','unicorn'];if(T.indexOf(t)<0)t='cyberpunk';
r.setAttribute('data-theme',t);
var S={small:87.5,medium:100,large:112.5,xlarge:125},f=g('docklite-font-size');
r.style.fontSize=(S[f]||100)+'%';
var a=g('docklite-animations');
if(a===null)a='true';
r.setAttribute('data-animations',a==='true'?'on':'off');
var n=g('docklite-neon-intensity'),v=n===null?0:Number(n);
if(!isFinite(v))v=0;
var b=Math.min(200,Math.max(0,v))/200;
r.style.setProperty('--glow',String(1+b));r.style.setProperty('--glow-boost',String(b));
}catch(e){}})();`;
