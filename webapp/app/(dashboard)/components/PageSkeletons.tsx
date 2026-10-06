import { Database, Package } from '@phosphor-icons/react';

// Loading placeholders that mirror the real Containers and Databases pages —
// same header, filter row, grid and card sizes — so nothing jumps when the
// data arrives. They use theme colors only, so they match every theme.

function Bone({ className = '' }: { className?: string }) {
  return <div className={`rounded animate-pulse ${className}`} style={{ background: 'rgba(var(--neon-purple-rgb), 0.2)' }} />;
}

function ContainerCardSkeleton() {
  return (
    <div
      className="p-4 rounded-xl h-[340px] flex flex-col relative"
      style={{ background: 'var(--surface-dim)', border: '2px solid rgba(var(--neon-purple-rgb), 0.3)' }}
      aria-hidden="true"
    >
      <Bone className="absolute top-3 right-3 w-9 h-9 rounded-lg" />
      <Bone className="h-5 w-20 rounded-full" />
      <div className="mt-8 space-y-2">
        <Bone className="h-5 w-3/4 mx-auto" />
        <Bone className="h-5 w-1/2 mx-auto" />
        <Bone className="h-3 w-1/3 mx-auto mt-3" />
      </div>
      <div className="mt-5 space-y-3">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center gap-2">
            <Bone className="w-4 h-4 rounded-full" />
            <Bone className="h-3 w-2/3" />
          </div>
        ))}
      </div>
      <div className="flex gap-2 mt-auto pt-2 border-t" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.2)' }}>
        <Bone className="h-9 flex-1 rounded-lg" />
        <Bone className="h-9 flex-1 rounded-lg" />
        <Bone className="h-9 flex-1 rounded-lg" />
      </div>
    </div>
  );
}

export function ContainersPageSkeleton() {
  return (
    <div className="max-w-[1400px] mx-auto" role="status" aria-label="Loading containers">
      {/* Header: the real title, with the subtitle and action buttons as placeholders */}
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <div>
          <h1 className="docklite-containers-title text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Package size={26} weight="duotone" />
            Containers
          </h1>
          <div className="flex items-center gap-3">
            <Bone className="h-4 w-36" />
            <Bone className="h-5 w-24 rounded-full" />
          </div>
        </div>
        <div className="flex gap-3">
          <Bone className="h-10 w-40 rounded-lg" />
          <Bone className="h-10 w-32 rounded-lg" />
          <Bone className="h-10 w-36 rounded-lg" />
        </div>
      </div>

      {/* Filter tabs and state switch */}
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-2">
          {['w-20', 'w-24', 'w-32', 'w-24'].map((width, i) => (
            <Bone key={i} className={`h-10 rounded-xl ${width}`} />
          ))}
        </div>
        <Bone className="h-9 w-56 rounded-xl" />
      </div>

      {/* One folder with a grid of cards, in the real grid */}
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Bone className="w-7 h-7 rounded-full" />
          <Bone className="h-8 w-48" />
          <Bone className="h-6 w-20 rounded-full" />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-6 p-4 rounded-xl border-2 border-transparent">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <ContainerCardSkeleton key={i} />
          ))}
        </div>
      </div>
    </div>
  );
}

function DatabaseCardSkeleton() {
  return (
    <div
      className="p-6 rounded-xl relative"
      style={{ background: 'var(--surface-dim)', border: '2px solid rgba(var(--neon-green-rgb), 0.35)' }}
      aria-hidden="true"
    >
      <Bone className="absolute top-4 right-4 w-9 h-9 rounded-lg" />
      <div className="flex items-center gap-3 mb-5">
        <Bone className="w-10 h-10 rounded-full" />
        <Bone className="h-6 w-40" />
      </div>
      <div className="space-y-3 mb-5">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center justify-between">
            <Bone className="h-4 w-20" />
            <Bone className="h-4 w-32" />
          </div>
        ))}
      </div>
      <div className="flex gap-2">
        <Bone className="h-9 flex-1 rounded-lg" />
        <Bone className="h-9 flex-1 rounded-lg" />
      </div>
    </div>
  );
}

export function DatabasesPageSkeleton() {
  return (
    <div className="max-w-[1400px] mx-auto" role="status" aria-label="Loading databases">
      {/* The "DockLite System Database" card that sits above the header */}
      <div className="w-full mb-6 card-vapor p-6 rounded-xl border border-cyan-500/30">
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <Bone className="h-6 w-64" />
            <Bone className="h-3 w-80 max-w-full" />
          </div>
          <Bone className="w-7 h-7 rounded-full" />
        </div>
      </div>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
            <Database size={24} weight="duotone" />
            Databases
          </h1>
          <Bone className="h-4 w-48" />
        </div>
        <Bone className="h-10 w-44 rounded-lg" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {[0, 1, 2].map((i) => (
          <DatabaseCardSkeleton key={i} />
        ))}
      </div>
    </div>
  );
}
