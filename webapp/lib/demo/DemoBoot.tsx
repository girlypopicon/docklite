'use client';

import { useState } from 'react';
import { DEMO } from './flag';
import { resetState } from './state';

// Installed at module load so the fakes are in place before any page code runs a request. The
// require sits behind the build-time flag so normal builds don't contain the demo code at all.
if (DEMO) {
  require('./install').installDemo();
}

export default function DemoBoot() {
  const [hidden, setHidden] = useState(false);
  if (hidden) return null;
  return (
    <div
      role="status"
      style={{
        position: 'fixed', left: '50%', bottom: 12, transform: 'translateX(-50%)', zIndex: 60, maxWidth: 'calc(100vw - 24px)',
        display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'nowrap', whiteSpace: 'nowrap', justifyContent: 'center', padding: '7px 14px', borderRadius: 999,
        background: 'rgba(20, 8, 40, 0.92)', border: '1px solid rgba(255, 106, 213, 0.6)', color: '#fff', fontSize: 12, backdropFilter: 'blur(6px)',
        boxShadow: '0 0 18px rgba(255, 106, 213, 0.35)',
      }}
    >
      <span><b style={{ color: '#ff6ad5' }}>DEMO</b>&nbsp; Nothing is real or saved.</span>
      <button onClick={() => { resetState(); window.location.href = '/'; }} style={{ background: 'transparent', border: '1px solid #9ad0ff', color: '#9ad0ff', borderRadius: 999, padding: '2px 10px', cursor: 'pointer' }}>Reset</button>
      <a href="https://github.com/girlypopicon/docklite" style={{ color: '#b8f2a2', textDecoration: 'underline' }}>Get DockLite</a>
      <button aria-label="Hide banner" onClick={() => setHidden(true)} style={{ background: 'transparent', border: 'none', color: '#fff', cursor: 'pointer', fontSize: 14 }}>×</button>
    </div>
  );
}
