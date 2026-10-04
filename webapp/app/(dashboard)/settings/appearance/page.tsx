'use client';

import { useEffect, useState } from 'react';
import { Palette, TextT, Sparkle, Eye } from '@phosphor-icons/react';
import ContainerCard from '../../components/ContainerCard';
import { useToast } from '@/lib/hooks/useToast';
import type { ContainerInfo } from '@/types';
import {
  DEFAULT_APPEARANCE,
  FONT_SCALES,
  applyAppearance,
  loadAppearance,
  saveAppearance,
  type Appearance,
} from '@/lib/appearance';

// Made-up containers for the preview: one running website, one stopped
// database, so both status colors and both card styles are visible.
const SAMPLE_CONTAINERS: ContainerInfo[] = [
  {
    id: 'preview-site',
    name: 'docklite-site-example-com',
    status: 'Up 3 days',
    state: 'running',
    uptime: '3d 4h',
    image: 'nginx:alpine',
    ports: '0.0.0.0:32768->80/tcp',
    labels: { 'docklite.type': 'static', 'docklite.domain': 'example.com' },
    owner_username: 'you',
  },
  {
    id: 'preview-db',
    name: 'docklite-db-example',
    status: 'Exited (0) 2 hours ago',
    state: 'exited',
    uptime: '',
    image: 'postgres:16-alpine',
    ports: '127.0.0.1:5433->5432/tcp',
    labels: { 'docklite.type': 'postgres' },
    owner_username: 'you',
  },
];

export default function AppearanceSettingsPage() {
  const toast = useToast();
  const [appearance, setAppearance] = useState<Appearance>(DEFAULT_APPEARANCE);
  const { theme, fontSize, animations, glow: neonIntensity } = appearance;

  // Every change applies immediately and is remembered, so there is nothing
  // to "save": what you see is what you'll get after a reload too.
  const update = (change: Partial<Appearance>) => {
    const next = { ...appearance, ...change };
    setAppearance(next);
    applyAppearance(next);
    saveAppearance(next);
  };

  const themes = [
    {
      id: 'cyberpunk',
      name: 'Neon',
      description: 'Original DockLite neon cyber theme',
      preview: 'bg-gradient-to-br from-purple-900 via-blue-900 to-cyan-900',
    },
    {
      id: 'corpo',
      name: 'Corpo',
      description: 'Clean greys with soft pink accents',
      preview: 'bg-gradient-to-br from-gray-100 via-gray-200 to-pink-200',
    },
    {
      id: 'corpo-blue',
      name: 'Corpo Blue',
      description: 'Clean greys with cool blue accents',
      preview: 'theme-preview-corpo-blue',
    },
    {
      id: 'unicorn',
      name: 'Unicorn',
      description: 'Spectrum palette with DockLite brand colors',
      preview: 'theme-preview-unicorn',
    },
  ];

  const fontSizes = [
    { id: 'small', name: 'Small', size: '12px' },
    { id: 'medium', name: 'Medium (default)', size: '14px' },
    { id: 'large', name: 'Large', size: '16px' },
    { id: 'xlarge', name: 'Extra Large', size: '18px' },
  ] as const;

  useEffect(() => {
    setAppearance(loadAppearance());
  }, []);

  const resetToDefaults = () => update(DEFAULT_APPEARANCE);

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
          <Palette size={20} weight="duotone" />
          Appearance Settings
        </h2>
        <p className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
          Customize your visual experience
        </p>
      </div>

      {/* Theme Selection */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
          <Palette size={20} weight="duotone" />
          Theme Selection
        </h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {themes.map((themeOption) => (
            <div
              key={themeOption.id}
              className="p-4 rounded-lg border-2 cursor-pointer transition-all"
              style={theme === themeOption.id ? {
                borderColor: 'var(--neon-cyan)',
                background: 'rgba(var(--neon-cyan-rgb), 0.1)'
              } : {
                borderColor: 'rgba(var(--neon-purple-rgb), 0.2)'
              }}
              onClick={() => update({ theme: themeOption.id as Appearance['theme'] })}
            >
              <div className={`w-full h-20 rounded-lg mb-3 ${themeOption.preview}`}></div>
              <h3 className="font-bold mb-1">{themeOption.name}</h3>
              <p className="text-xs opacity-70">{themeOption.description}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Font Settings */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
          <TextT size={20} weight="duotone" />
          Font Settings
        </h2>
        <div className="space-y-4">
          <div>
            <label className="block font-bold mb-1">Text Size</label>
            <p className="text-xs opacity-70 mb-3">Scales all text and spacing in the dashboard ({FONT_SCALES[fontSize]}% of normal).</p>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              {fontSizes.map((size) => (
                <button
                  key={size.id}
                  onClick={() => update({ fontSize: size.id })}
                  className="p-3 rounded-lg border-2 transition-all"
                  style={fontSize === size.id ? {
                    borderColor: 'var(--neon-cyan)',
                    background: 'rgba(var(--neon-cyan-rgb), 0.1)'
                  } : {
                    borderColor: 'rgba(var(--neon-purple-rgb), 0.2)'
                  }}
                >
                  <div className="font-bold" style={{ fontSize: size.size }}>Aa</div>
                  <div className="text-xs mt-1">{size.name}</div>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Visual Effects */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--neon-yellow)' }}>
          <Sparkle size={20} weight="duotone" />
          Visual Effects
        </h2>
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-bold">Animations & Transitions</div>
              <div className="text-sm opacity-70">Enable smooth animations and hover effects</div>
            </div>
            <button
              onClick={() => update({ animations: !animations })}
              className="px-4 py-2 rounded-lg font-bold transition-all"
              style={animations ? {
                background: 'var(--status-success)',
                color: 'var(--button-text)'
              } : {
                background: 'var(--surface-dim)',
                color: 'var(--text-primary)'
              }}
            >
              {animations ? 'ON' : 'OFF'}
            </button>
          </div>

          <div>
            <label className="block font-bold mb-3">Neon Glow Intensity</label>
            <div className="flex items-center gap-4">
              <input
                type="range"
                min="0"
                max="200"
                value={neonIntensity}
                onChange={(e) => update({ glow: Number(e.target.value) })}
                className="flex-1 h-2 rounded-lg appearance-none cursor-pointer"
                style={{ background: 'var(--neon-purple)' }}
              />
              <span className="text-sm font-mono w-12">{neonIntensity}%</span>
            </div>
            <div className="text-xs opacity-70 mt-2">
              Adjust the intensity of neon glow effects
            </div>
          </div>
        </div>
      </div>

      {/* Preview: the real container card with made-up data */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
          <Eye size={20} weight="duotone" />
          Preview
        </h2>
        <p className="text-sm opacity-70 mb-6" style={{ color: 'var(--text-secondary)' }}>
          These are the same cards you see on the Containers page, filled with made-up data. They update live as you
          change the settings above, and their buttons don’t do anything.
        </p>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
          {SAMPLE_CONTAINERS.map((container) => (
            <ContainerCard
              key={container.id}
              container={container}
              onAction={() => toast.info('Preview only — this container isn’t real')}
              onViewDetails={() => toast.info('Preview only — this container isn’t real')}
              isTracked
            />
          ))}
        </div>
      </div>

      {/* Reset */}
      <div className="text-center">
        <button onClick={resetToDefaults} className="btn-neon px-8 py-3 font-bold text-lg">
          Reset to defaults
        </button>
        <div className="text-xs opacity-60 mt-2" style={{ color: 'var(--text-secondary)' }}>
          Changes apply and are saved automatically in this browser.
        </div>
      </div>
    </div>
  );
}
