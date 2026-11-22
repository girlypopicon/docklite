import type { Icon } from '@phosphor-icons/react';
import {
  Package, FolderSimple, Globe, Heart, Sparkle, RocketLaunch, Lightning, Planet, Cloud, Flower, Moon, Star, GameController, Cat, Camera,
  Storefront, MusicNotes,
} from '@phosphor-icons/react';

/**
 * How a container's card can look: its colour (the glow and the big icon behind it) and that icon. The ids are what the agent
 * stores (go-app/internal/handlers/box_style.go lists the same ones, and rejects anything else). "" is the default: a colour
 * by what the box is (sites pink, databases green, the rest cyan) and the box icon.
 */
export type BoxColorId = '' | 'pink' | 'coral' | 'orange' | 'gold' | 'lime' | 'mint' | 'teal' | 'sky' | 'blue' | 'lilac' | 'rose' | 'slate';

// "r, g, b" so the card can use them as `rgba(var(--c), 0.4)`, like the theme's own neon colours.
export const BOX_COLORS: { id: Exclude<BoxColorId, ''>; label: string; rgb: string }[] = [
  { id: 'pink', label: 'Pink', rgb: '255, 106, 213' },
  { id: 'coral', label: 'Coral', rgb: '255, 154, 139' },
  { id: 'orange', label: 'Orange', rgb: '255, 159, 67' },
  { id: 'gold', label: 'Gold', rgb: '255, 209, 102' },
  { id: 'lime', label: 'Lime', rgb: '190, 235, 80' },
  { id: 'mint', label: 'Mint', rgb: '184, 242, 162' },
  { id: 'teal', label: 'Teal', rgb: '64, 224, 208' },
  { id: 'sky', label: 'Sky', rgb: '154, 208, 255' },
  { id: 'blue', label: 'Blue', rgb: '90, 130, 255' },
  { id: 'lilac', label: 'Lilac', rgb: '199, 164, 255' },
  { id: 'rose', label: 'Rose', rgb: '255, 77, 124' },
  { id: 'slate', label: 'Slate', rgb: '160, 168, 190' },
];

export const BOX_COLOR_RGB: Record<string, string> = Object.fromEntries(BOX_COLORS.map((c) => [c.id, c.rgb]));

export type BoxIconId = '' | 'globe' | 'heart' | 'sparkle' | 'rocket' | 'lightning' | 'planet' | 'cloud' | 'flower' | 'moon' | 'star' | 'gamepad' | 'cat' | 'camera' | 'storefront' | 'music';

export const BOX_ICONS: { id: BoxIconId; label: string; Icon: Icon }[] = [
  { id: '', label: 'Container', Icon: Package },
  { id: 'globe', label: 'Globe', Icon: Globe },
  { id: 'heart', label: 'Heart', Icon: Heart },
  { id: 'sparkle', label: 'Sparkle', Icon: Sparkle },
  { id: 'rocket', label: 'Rocket', Icon: RocketLaunch },
  { id: 'lightning', label: 'Lightning', Icon: Lightning },
  { id: 'planet', label: 'Planet', Icon: Planet },
  { id: 'cloud', label: 'Cloud', Icon: Cloud },
  { id: 'flower', label: 'Flower', Icon: Flower },
  { id: 'moon', label: 'Moon', Icon: Moon },
  { id: 'star', label: 'Star', Icon: Star },
  { id: 'gamepad', label: 'Game', Icon: GameController },
  { id: 'cat', label: 'Cat', Icon: Cat },
  { id: 'camera', label: 'Camera', Icon: Camera },
  { id: 'storefront', label: 'Shop', Icon: Storefront },
  { id: 'music', label: 'Music', Icon: MusicNotes },
];

export function boxIconFor(id: string | undefined): Icon {
  return BOX_ICONS.find((i) => i.id === (id || ''))?.Icon || Package;
}

/**
 * The font size for a card's name, so it fits on ONE line: as big as the card's width allows (up to 19px), stepping down
 * for longer names to a floor of 10px. `cqw` is a percent of the card's own width (the card is a container, see
 * ContainerCard), so it holds at any card size. A name still too long at the floor is cut with "…" by CSS (truncate) and
 * the full name is the card's tooltip. Assumes about 0.6em per character; narrower fonts simply fit with room to spare.
 */
export function boxNameFontSize(name: string): string {
  const n = Math.max(name.length, 1);
  return `clamp(10px, calc(122cqw / ${n}), 19px)`;
}

/** A group's big icon: the folder unless one was picked. */
export function groupIconFor(id: string | undefined): Icon {
  return id ? boxIconFor(id) : FolderSimple;
}
