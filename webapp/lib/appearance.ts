// Appearance settings (theme, text size, animations, glow), kept in
// localStorage and applied as attributes/variables on <html>. Everything in
// the app that reacts to them keys off those, so changes show up instantly
// and survive reloads. The inline script in app/layout.tsx mirrors
// applyAppearance() so the saved look is in place before first paint.

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
  glow: 100,
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

export function saveAppearance(appearance: Appearance) {
  try {
    localStorage.setItem(KEYS.theme, appearance.theme);
    localStorage.setItem(KEYS.fontSize, appearance.fontSize);
    localStorage.setItem(KEYS.animations, String(appearance.animations));
    localStorage.setItem(KEYS.glow, String(appearance.glow));
  } catch {
    /* private mode etc.: the look still applies for this visit */
  }
}

export function applyAppearance(appearance: Appearance) {
  const root = document.documentElement;
  root.setAttribute('data-theme', appearance.theme);
  root.setAttribute('data-animations', appearance.animations ? 'on' : 'off');
  root.style.fontSize = `${FONT_SCALES[appearance.fontSize]}%`;
  root.style.setProperty('--glow', String(appearance.glow / 100));
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
var n=g('docklite-neon-intensity'),v=n===null?100:Number(n);
if(!isFinite(v))v=100;
r.style.setProperty('--glow',String(Math.min(200,Math.max(0,v))/100));
}catch(e){}})();`;
