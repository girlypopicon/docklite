'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ContainerInfo, BoxGroup } from '@/types';
import ContainerDetailsModal from './components/ContainerDetailsModal';
import AllContainersModal from './components/AllContainersModal';
import ContainerCard from './components/ContainerCard';
import GroupCard from './components/GroupCard';
import GroupModal from './components/GroupModal';
import GroupNameModal from './components/GroupNameModal';
import MoveToGroupModal from './components/MoveToGroupModal';
import AssignBoxModal from './components/AssignBoxModal';
import NewBoxModal from './components/NewBoxModal';
import DatabaseBoxModal from './components/DatabaseBoxModal';
import ConfirmModal from './components/ConfirmModal';
import { ContainersPageSkeleton } from './components/PageSkeletons';
import ContainerFilterTabs, { type StatusFilter, type TypeFilter } from './components/ContainerFilterTabs';
import { useToast } from '@/lib/hooks/useToast';
import { containerKind, sortContainers } from '@/lib/container-sort';
import { formatBytes, type DatabaseRecord } from '@/lib/database-engines';
import { PlusCircle, FolderPlus, Package, ArrowsClockwise, WarningCircle } from '@phosphor-icons/react';
import {
  DndContext,
  DragOverlay,
  MouseSensor,
  TouchSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core';

const boxKey = (id: string) => `box:${id}`;
const groupKey = (id: number) => `group:${id}`;

/**
 * One card on the desktop. A box can be picked up and dropped on another box (they become a group) or on a group
 * (it goes in). Groups are only drop targets. A SQLite file card is neither: it lives inside a site.
 */
function Tile({
  id,
  draggable,
  droppable,
  children,
}: {
  id: string;
  draggable: boolean;
  droppable: boolean;
  children: (state: { isOver: boolean; isDragging: boolean }) => React.ReactNode;
}) {
  const drag = useDraggable({ id, disabled: !draggable });
  const drop = useDroppable({ id, disabled: !droppable });
  return (
    <div
      ref={(el) => {
        drag.setNodeRef(el);
        drop.setNodeRef(el);
      }}
      {...(draggable ? drag.listeners : {})}
      style={{ opacity: drag.isDragging ? 0.35 : 1 }}
    >
      {children({ isOver: drop.isOver && !drag.isDragging, isDragging: drag.isDragging })}
    </div>
  );
}

type Me = { userId: number; username: string; isAdmin: boolean; role: string };

export default function DashboardPage() {
  const [boxes, setBoxes] = useState<ContainerInfo[]>([]);
  const [groups, setGroups] = useState<BoxGroup[]>([]);
  const [databases, setDatabases] = useState<DatabaseRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [me, setMe] = useState<Me | null>(null);
  const [filterType, setFilterType] = useState<TypeFilter>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('any');

  // windows
  const [details, setDetails] = useState<{ id: string; name: string } | null>(null);
  const [dbOpen, setDbOpen] = useState<{ db: DatabaseRecord; box?: ContainerInfo } | null>(null);
  const [openGroupId, setOpenGroupId] = useState<number | null>(null);
  const [showAll, setShowAll] = useState(false);
  const [showNewBox, setShowNewBox] = useState(false);
  const [nameDialog, setNameDialog] = useState<
    | { kind: 'new' }
    | { kind: 'rename'; group: BoxGroup }
    | { kind: 'make'; ids: string[]; names: string[] }
    | null
  >(null);
  const [nameBusy, setNameBusy] = useState(false);
  const [nameError, setNameError] = useState('');
  const [moveTarget, setMoveTarget] = useState<{ id: string; name: string } | null>(null);
  const [moveBusy, setMoveBusy] = useState(false);
  const [moveError, setMoveError] = useState('');
  const [deleteGroupTarget, setDeleteGroupTarget] = useState<BoxGroup | null>(null);
  const [assignTarget, setAssignTarget] = useState<{ id: string; name: string } | null>(null);
  const [assignUsers, setAssignUsers] = useState<Array<{ id: number; username: string }>>([]);
  const [assignUserId, setAssignUserId] = useState('');
  const [assignBusy, setAssignBusy] = useState(false);
  const [assignError, setAssignError] = useState('');

  // dragging
  const [activeId, setActiveId] = useState<string | null>(null);
  const [overId, setOverId] = useState<string | null>(null);
  // Boxes the person has collapsed to a thin bar (kept in this browser, by box name).
  const [collapsed, setCollapsed] = useState<string[]>([]);
  useEffect(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('docklite-collapsed-boxes') || '[]');
      if (Array.isArray(saved)) setCollapsed(saved.filter((x) => typeof x === 'string'));
    } catch {
      /* no saved list: nothing is collapsed */
    }
  }, []);
  const toggleCollapsed = (id: string) => {
    const box = boxes.find((b) => b.id === id) || allBoxes.find((b) => b.id === id);
    if (!box) return;
    setCollapsed((cur) => {
      const next = cur.includes(box.name) ? cur.filter((n) => n !== box.name) : [...cur, box.name];
      try {
        localStorage.setItem('docklite-collapsed-boxes', JSON.stringify(next));
      } catch {
        /* private mode: it stays collapsed until the page is reloaded */
      }
      return next;
    });
  };

  const toast = useToast();

  const fetchData = useCallback(async () => {
    try {
      const res = await fetch('/api/containers');
      if (!res.ok) throw new Error('Failed to fetch data');
      const data = await res.json();
      setBoxes(data.boxes || []);
      setGroups(data.groups || []);
      setError('');
    } catch {
      setError('Failed to load your boxes');
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchDatabases = useCallback(async () => {
    try {
      const res = await fetch('/api/databases/stats');
      if (!res.ok) return;
      const data = await res.json();
      setDatabases(data.databases || []);
    } catch {
      /* database details just stay as they were */
    }
  }, []);

  useEffect(() => {
    fetchData();
    fetchDatabases();
    fetch('/api/auth/me')
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => setMe(d?.user || null))
      .catch(() => setMe(null));
    const a = setInterval(fetchData, 10000);
    const b = setInterval(fetchDatabases, 30000);
    return () => {
      clearInterval(a);
      clearInterval(b);
    };
  }, [fetchData, fetchDatabases]);

  const refresh = () => {
    fetchData();
    fetchDatabases();
  };

  // ---- what is on the desktop ------------------------------------------------------------------------------------

  // A SQLite database is a file inside a site, not a container: it gets a card of its own so it can still be opened.
  const sqliteBoxes: ContainerInfo[] = useMemo(
    () =>
      databases
        .filter((d) => d.type === 'sqlite')
        .map((d) => ({
          id: `sqlite-${d.id}`,
          name: `docklite-sqlite-${d.name}`,
          image: 'sqlite',
          status: '',
          state: 'running',
          uptime: formatBytes(d.size),
          ports: '',
          labels: { 'docklite.type': 'sqlite', 'docklite.database': d.name },
        })),
    [databases],
  );

  const allBoxes = useMemo(() => [...boxes, ...sqliteBoxes], [boxes, sqliteBoxes]);
  const boxByName = useMemo(() => new Map(allBoxes.map((b) => [b.name, b])), [allBoxes]);
  const groupOfBox = useMemo(() => {
    const m = new Map<string, BoxGroup>();
    for (const g of groups) for (const n of g.members) m.set(n, g);
    return m;
  }, [groups]);
  const dbByContainer = useMemo(() => new Map(databases.filter((d) => d.container_id).map((d) => [d.container_id as string, d])), [databases]);

  const matchesStatus = (b: ContainerInfo) => statusFilter === 'any' || (statusFilter === 'running' ? b.state === 'running' : b.state !== 'running');
  const matchesFilters = (b: ContainerInfo) => {
    if (!matchesStatus(b)) return false;
    const kind = containerKind(b);
    if (filterType === 'sites') return kind === 'site';
    if (filterType === 'databases') return kind === 'database';
    if (filterType === 'other') return kind === 'other';
    return true;
  };
  const filtering = filterType !== 'all' || statusFilter !== 'any';

  const typeCounts = useMemo(() => {
    const c = { all: 0, sites: 0, databases: 0, other: 0 };
    for (const b of allBoxes) {
      if (!matchesStatus(b)) continue;
      c.all += 1;
      const k = containerKind(b);
      if (k === 'site') c.sites += 1;
      else if (k === 'database') c.databases += 1;
      else c.other += 1;
    }
    return c;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allBoxes, statusFilter]);

  const membersOf = (g: BoxGroup) => g.members.map((n) => boxByName.get(n)).filter(Boolean) as ContainerInfo[];

  const visibleGroups = groups.filter((g) => !filtering || membersOf(g).some(matchesFilters));
  const looseBoxes = sortContainers(allBoxes.filter((b) => !groupOfBox.has(b.name) && matchesFilters(b)));
  const nothingAtAll = allBoxes.length === 0 && groups.length === 0;
  const openBoxes = looseBoxes.filter((b) => !collapsed.includes(b.name));
  const collapsedBoxes = looseBoxes.filter((b) => collapsed.includes(b.name));
  const collapsedStacks: ContainerInfo[][] = [];
  for (let i = 0; i < collapsedBoxes.length; i += 6) collapsedStacks.push(collapsedBoxes.slice(i, i + 6));
  const nothingMatches = !nothingAtAll && visibleGroups.length === 0 && looseBoxes.length === 0;
  const openGroup = groups.find((g) => g.id === openGroupId) || null;

  // ---- actions on boxes -----------------------------------------------------------------------------------------

  const handleAction = async (id: string, action: 'start' | 'stop' | 'restart') => {
    try {
      const res = await fetch(`/api/containers/${id}/${action}`, { method: 'POST' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || `Could not ${action} the box`);
      toast.success(`Box ${action === 'stop' ? 'stopped' : action === 'start' ? 'started' : 'restarted'}`);
      fetchData();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete "${name}"? This stops and removes the box.`)) return;
    try {
      const res = await fetch(`/api/containers/${id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not delete the box');
      toast.success(`Deleted ${name}`);
      refresh();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const handleToggleTracking = async (id: string, tracked: boolean) => {
    try {
      const res = await fetch(`/api/containers/${id}/${tracked ? 'untrack' : 'track'}`, { method: 'POST' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not change tracking');
      toast.success(tracked ? 'Box untracked' : 'Box tracked');
      fetchData();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const handleBoxStyle = async (id: string, style: { color: string; icon: string }) => {
    try {
      const res = await fetch(`/api/containers/${id}/style`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(style) });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not save the card style');
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
      fetchData();
    }
  };

  const openAssign = async (id: string, name: string) => {
    setAssignTarget({ id, name });
    setAssignError('');
    setAssignUserId('');
    if (assignUsers.length === 0) {
      try {
        const res = await fetch('/api/users');
        if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not load users');
        setAssignUsers((await res.json()).users || []);
      } catch (err: any) {
        setAssignError(err.message || 'Could not load users');
      }
    }
  };

  const submitAssign = async () => {
    if (!assignTarget || !assignUserId) return;
    setAssignBusy(true);
    setAssignError('');
    try {
      const res = await fetch(`/api/containers/${assignTarget.id}/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: Number(assignUserId) }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not assign the box');
      if (data.warning) toast.warning(`Assigned ${assignTarget.name}, but: ${data.warning}`, 12000);
      else toast.success(data.old_folder_moved_to ? `Assigned ${assignTarget.name}; the old folder is in the trash` : `Assigned ${assignTarget.name}`);
      setAssignTarget(null);
      refresh();
    } catch (err: any) {
      setAssignError(err.message || 'Could not assign the box');
    } finally {
      setAssignBusy(false);
    }
  };

  const openBox = (id: string, name: string) => {
    const box = allBoxes.find((b) => b.id === id);
    const sqlite = id.startsWith('sqlite-') ? databases.find((d) => `sqlite-${d.id}` === id) : undefined;
    if (sqlite) {
      setDbOpen({ db: sqlite, box: undefined });
      return;
    }
    const db = dbByContainer.get(id);
    if (db) {
      setDbOpen({ db, box });
      return;
    }
    setDetails({ id, name });
  };

  // ---- groups ------------------------------------------------------------------------------------------------------

  const api = async (path: string, method: string, body?: unknown) => {
    const res = await fetch(path, { method, headers: body ? { 'Content-Type': 'application/json' } : undefined, body: body ? JSON.stringify(body) : undefined });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || 'Something went wrong');
    return data;
  };

  const submitName = async (name: string) => {
    if (!nameDialog) return;
    setNameBusy(true);
    setNameError('');
    try {
      if (nameDialog.kind === 'rename') await api(`/api/groups/${nameDialog.group.id}`, 'PUT', { name });
      else await api('/api/groups', 'POST', { name, container_ids: nameDialog.kind === 'make' ? nameDialog.ids : [] });
      setNameDialog(null);
      fetchData();
    } catch (err: any) {
      setNameError(err.message);
    } finally {
      setNameBusy(false);
    }
  };

  const setLinked = async (g: BoxGroup, linked: boolean) => {
    setGroups((prev) => prev.map((x) => (x.id === g.id ? { ...x, linked } : x)));
    try {
      await api(`/api/groups/${g.id}`, 'PUT', { linked });
    } catch (err: any) {
      toast.error(`Error: ${err.message}`);
      fetchData();
    }
  };

  const styleGroup = async (g: BoxGroup, style: { color: string; icon: string }) => {
    try {
      await api(`/api/groups/${g.id}`, 'PUT', style);
    } catch (err: any) {
      toast.error(`Error: ${err.message}`);
      fetchData();
    }
  };

  const deleteGroup = async () => {
    if (!deleteGroupTarget) return;
    try {
      await api(`/api/groups/${deleteGroupTarget.id}`, 'DELETE');
      toast.success(`Group "${deleteGroupTarget.name}" deleted. Its boxes are back on the desktop`);
      if (openGroupId === deleteGroupTarget.id) setOpenGroupId(null);
    } catch (err: any) {
      toast.error(`Error: ${err.message}`);
    }
    setDeleteGroupTarget(null);
    fetchData();
  };

  const addToGroup = async (g: BoxGroup, box: ContainerInfo) => {
    try {
      await api(`/api/groups/${g.id}/members`, 'POST', { container_id: box.id });
      toast.success(`${box.labels?.['docklite.domain'] || box.name} is in ${g.name}`);
    } catch (err: any) {
      toast.error(`Error: ${err.message}`);
    }
    fetchData();
  };

  const removeFromGroup = async (id: string) => {
    const box = boxes.find((b) => b.id === id);
    const g = box && groupOfBox.get(box.name);
    if (!g) return;
    try {
      await api(`/api/groups/${g.id}/members?container_id=${encodeURIComponent(id)}`, 'DELETE');
    } catch (err: any) {
      toast.error(`Error: ${err.message}`);
    }
    fetchData();
  };

  const submitMove = async (target: { groupId: number | null } | { newName: string }) => {
    if (!moveTarget) return;
    setMoveBusy(true);
    setMoveError('');
    try {
      if ('newName' in target) {
        await api('/api/groups', 'POST', { name: target.newName, container_ids: [moveTarget.id] });
      } else if (target.groupId === null) {
        const box = boxes.find((b) => b.id === moveTarget.id);
        const g = box && groupOfBox.get(box.name);
        if (g) await api(`/api/groups/${g.id}/members?container_id=${encodeURIComponent(moveTarget.id)}`, 'DELETE');
      } else {
        await api(`/api/groups/${target.groupId}/members`, 'POST', { container_id: moveTarget.id });
      }
      setMoveTarget(null);
      fetchData();
    } catch (err: any) {
      setMoveError(err.message);
    } finally {
      setMoveBusy(false);
    }
  };

  // ---- dragging ----------------------------------------------------------------------------------------------------

  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 8 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 300, tolerance: 6 } }),
  );

  const handleDragStart = (e: DragStartEvent) => setActiveId(String(e.active.id));
  const handleDragOver = (e: DragOverEvent) => setOverId(e.over ? String(e.over.id) : null);
  const handleDragEnd = (e: DragEndEvent) => {
    const from = String(e.active.id);
    const to = e.over ? String(e.over.id) : null;
    setActiveId(null);
    setOverId(null);
    if (!to || to === from || !from.startsWith('box:')) return;
    const fromBox = allBoxes.find((b) => boxKey(b.id) === from);
    if (!fromBox) return;
    if (to.startsWith('group:')) {
      const g = groups.find((x) => groupKey(x.id) === to);
      if (g) addToGroup(g, fromBox);
      return;
    }
    if (to.startsWith('box:')) {
      const other = allBoxes.find((b) => boxKey(b.id) === to);
      if (other) setNameDialog({ kind: 'make', ids: [fromBox.id, other.id], names: [fromBox.labels?.['docklite.domain'] || fromBox.name, other.labels?.['docklite.domain'] || other.name] });
    }
  };
  const activeBox = activeId ? allBoxes.find((b) => boxKey(b.id) === activeId) : undefined;

  // ---- drawing -----------------------------------------------------------------------------------------------------

  const card = (b: ContainerInfo, opts?: { inGroup?: boolean; collapsed?: boolean }) => (
    <ContainerCard
      container={b}
      onAction={handleAction}
      onViewDetails={openBox}
      onDelete={handleDelete}
      onAssign={openAssign}
      canAssign={Boolean(me?.isAdmin)}
      onMoveGroup={(id, name) => {
        setMoveTarget({ id, name });
        setMoveError('');
      }}
      onRemoveFromGroup={opts?.inGroup ? (id) => removeFromGroup(id) : undefined}
      onToggleTracking={me?.isAdmin ? handleToggleTracking : undefined}
      onStyle={handleBoxStyle}
      collapsed={Boolean(opts?.collapsed)}
      onToggleCollapse={opts?.inGroup ? undefined : toggleCollapsed}
    />
  );

  if (loading) return <ContainersPageSkeleton />;

  if (error && allBoxes.length === 0) {
    return (
      <div className="text-center py-16">
        <div className="flex justify-center mb-4 animate-pulse">
          <WarningCircle size={48} weight="regular" color="var(--status-error)" />
        </div>
        <div className="text-xl font-bold mb-4" style={{ color: 'var(--status-error)' }}>{error}</div>
        <button onClick={refresh} className="btn-neon px-6 py-3 font-bold inline-flex items-center gap-2">
          <ArrowsClockwise size={20} weight="regular" />
          Try again
        </button>
      </div>
    );
  }

  const gridClass = 'grid gap-3 sm:gap-4 p-2 sm:p-3 grid-cols-[repeat(auto-fill,minmax(min(100%,148px),1fr))] sm:grid-cols-[repeat(auto-fill,minmax(172px,1fr))]';

  return (
    <div className="max-w-[1400px] mx-auto">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between mb-6 gap-4">
        <div>
          <h1 className="docklite-containers-title text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
            <Package size={26} weight="regular" />
            Boxes
          </h1>
          <div className="flex items-center gap-3">
            <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>System status: online</p>
            <span className="text-xs font-mono px-2 py-0.5 rounded-full" style={{ background: 'rgba(var(--status-success-rgb), 0.2)', color: 'var(--text-green)', border: '1px solid var(--neon-green)' }}>
              {allBoxes.length} {allBoxes.length === 1 ? 'box' : 'boxes'}
              {groups.length > 0 ? ` · ${groups.length} ${groups.length === 1 ? 'group' : 'groups'}` : ''}
            </span>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 sm:gap-3">
          <button onClick={() => setShowNewBox(true)} className="btn-neon inline-flex items-center gap-2 whitespace-nowrap">
            <PlusCircle size={20} weight="regular" />
            New box
          </button>
          <button
            onClick={() => {
              setNameDialog({ kind: 'new' });
              setNameError('');
            }}
            className="inline-flex items-center gap-2 whitespace-nowrap px-4 py-2 text-sm font-bold rounded-lg transition-all hover:scale-105"
            style={{ background: 'linear-gradient(135deg, var(--neon-pink) 0%, var(--neon-purple) 100%)', color: 'var(--button-text)', boxShadow: '0 0 12px rgba(var(--neon-pink-rgb), 0.4)' }}
          >
            <FolderPlus size={20} weight="regular" />
            New group
          </button>
          <button onClick={() => setShowAll(true)} className="btn-neon inline-flex items-center gap-2 whitespace-nowrap">
            <Package size={20} weight="regular" />
            All containers
          </button>
        </div>
      </div>

      <ContainerFilterTabs type={filterType} onType={setFilterType} status={statusFilter} onStatus={setStatusFilter} counts={typeCounts} />

      {nothingAtAll ? (
        <div className="mt-12 text-center py-20 card-vapor max-w-3xl mx-auto animate-fade-in">
          <div className="flex justify-center mb-6 animate-float">
            <Package size={80} weight="regular" style={{ color: 'var(--text-cyan)', filter: 'drop-shadow(0 0 20px var(--neon-cyan))' }} />
          </div>
          <h2 className="text-3xl font-bold neon-text mb-3" style={{ color: 'var(--text-pink)' }}>Welcome to DockLite!</h2>
          <p className="text-lg mb-8" style={{ color: 'var(--text-secondary)' }}>No boxes yet. Make a website or a database to get started.</p>
          <div className="flex gap-4 justify-center flex-wrap">
            <button onClick={() => setShowNewBox(true)} className="btn-neon inline-flex items-center gap-2 text-base px-6 py-3">
              <PlusCircle size={22} weight="regular" />
              New box
            </button>
            <button onClick={() => setShowAll(true)} className="btn-neon inline-flex items-center gap-2 text-base px-6 py-3">
              <Package size={22} weight="regular" />
              See all containers
            </button>
          </div>
        </div>
      ) : nothingMatches ? (
        <div className="mt-12 text-center py-16 card-vapor max-w-2xl mx-auto animate-fade-in">
          <div className="flex justify-center mb-4 opacity-50">
            <WarningCircle size={64} weight="regular" style={{ color: 'var(--text-yellow)' }} />
          </div>
          <p className="text-xl font-bold neon-text mb-3" style={{ color: 'var(--text-pink)' }}>No boxes match this filter</p>
          <button
            onClick={() => {
              setFilterType('all');
              setStatusFilter('any');
            }}
            className="btn-neon inline-flex items-center gap-2"
          >
            <ArrowsClockwise size={20} weight="regular" />
            Show everything
          </button>
        </div>
      ) : (
        <DndContext sensors={sensors} onDragStart={handleDragStart} onDragOver={handleDragOver} onDragEnd={handleDragEnd} onDragCancel={() => { setActiveId(null); setOverId(null); }}>
          <div className={gridClass}>
            {visibleGroups.map((g) => (
              <Tile key={groupKey(g.id)} id={groupKey(g.id)} draggable={false} droppable>
                {({ isOver }) => (
                  <GroupCard
                    group={g}
                    boxes={membersOf(g)}
                    highlight={isOver && Boolean(activeId)}
                    showOwner={Boolean(me?.isAdmin)}
                    onOpen={(x) => setOpenGroupId(x.id)}
                    onRename={(x) => {
                      setNameDialog({ kind: 'rename', group: x });
                      setNameError('');
                    }}
                    onToggleLinked={setLinked}
                    onStyle={styleGroup}
                    onDelete={setDeleteGroupTarget}
                  />
                )}
              </Tile>
            ))}
            {openBoxes.map((b) => {
              const isFile = b.id.startsWith('sqlite-');
              return (
                <Tile key={boxKey(b.id)} id={boxKey(b.id)} draggable={!isFile} droppable={!isFile}>
                  {({ isOver }) => {
                    const hovering = isOver && Boolean(activeId) && activeId !== boxKey(b.id);
                    return (
                      <div
                        className="relative rounded-xl"
                        style={hovering ? { outline: '3px solid rgba(255,255,255,0.9)', outlineOffset: 4, transform: 'scale(1.03)', transition: 'transform .15s' } : undefined}
                      >
                        {card(b)}
                        {hovering && (
                          <div className="absolute inset-0 rounded-xl grid place-items-center pointer-events-none text-center px-3 font-bold text-sm" style={{ background: 'rgba(11,11,22,0.78)', color: 'var(--text-primary)' }}>
                            Let go to make a group
                          </div>
                        )}
                      </div>
                    );
                  }}
                </Tile>
              );
            })}
            {collapsedStacks.map((stack, i) => (
              // Six collapsed boxes take the space of one open card.
              <div key={`stack-${i}`} className="grid gap-1" style={{ aspectRatio: '5 / 7', gridTemplateRows: 'repeat(6, minmax(0, 1fr))' }}>
                {stack.map((b) => {
                  const isFile = b.id.startsWith('sqlite-');
                  return (
                    <Tile key={boxKey(b.id)} id={boxKey(b.id)} draggable={!isFile} droppable={!isFile}>
                      {({ isOver }) => {
                        const hovering = isOver && Boolean(activeId) && activeId !== boxKey(b.id);
                        return (
                          <div className="relative h-full rounded-lg" style={hovering ? { outline: '3px solid rgba(255,255,255,0.9)', outlineOffset: 2 } : undefined}>
                            {card(b, { collapsed: true })}
                          </div>
                        );
                      }}
                    </Tile>
                  );
                })}
              </div>
            ))}
          </div>
          <DragOverlay dropAnimation={null}>
            {activeBox ? (
              <div style={{ pointerEvents: 'none', opacity: 0.92, transform: 'rotate(2deg)' }}>
                <ContainerCard container={activeBox} onAction={() => undefined} />
              </div>
            ) : null}
          </DragOverlay>
        </DndContext>
      )}

      {details && <ContainerDetailsModal containerId={details.id} containerName={details.name} onClose={() => setDetails(null)} />}
      {dbOpen && (
        <DatabaseBoxModal
          db={dbOpen.db}
          container={dbOpen.box}
          onClose={() => setDbOpen(null)}
          onChanged={refresh}
          onOpenContainer={(b) => {
            setDbOpen(null);
            setDetails({ id: b.id, name: b.name });
          }}
        />
      )}
      {showAll && <AllContainersModal onClose={() => setShowAll(false)} />}
      {showNewBox && (
        <NewBoxModal
          sites={boxes.filter((b) => containerKind(b) === 'site')}
          onClose={() => setShowNewBox(false)}
          onCreated={refresh}
        />
      )}

      {openGroup && (
        <GroupModal
          group={openGroup}
          boxes={membersOf(openGroup)}
          addable={boxes.filter((b) => !groupOfBox.has(b.name))}
          renderBox={(b) => card(b, { inGroup: true })}
          onAdd={(b) => addToGroup(openGroup, b)}
          onClose={() => setOpenGroupId(null)}
        />
      )}

      {nameDialog && (
        <GroupNameModal
          title={nameDialog.kind === 'rename' ? 'Rename group' : nameDialog.kind === 'make' ? 'Make a group' : 'New group'}
          intro={nameDialog.kind === 'make' ? `${nameDialog.names.join(' and ')} will go in it. Linked groups are for boxes that belong together.` : undefined}
          initial={nameDialog.kind === 'rename' ? nameDialog.group.name : ''}
          confirmLabel={nameDialog.kind === 'rename' ? 'Rename' : 'Make group'}
          busy={nameBusy}
          error={nameError}
          onSubmit={submitName}
          onClose={() => setNameDialog(null)}
        />
      )}

      {moveTarget && (
        <MoveToGroupModal
          boxName={moveTarget.name}
          groups={groups}
          currentGroupId={groupOfBox.get(moveTarget.name)?.id ?? null}
          busy={moveBusy}
          error={moveError}
          onMove={submitMove}
          onClose={() => setMoveTarget(null)}
        />
      )}

      {deleteGroupTarget && (
        <ConfirmModal
          title="Delete group"
          message={`Delete the group "${deleteGroupTarget.name}"? Its ${deleteGroupTarget.members.length} ${deleteGroupTarget.members.length === 1 ? 'box goes' : 'boxes go'} back on the desktop. Nothing is stopped or deleted.`}
          confirmText="Delete group"
          onConfirm={deleteGroup}
          onCancel={() => setDeleteGroupTarget(null)}
        />
      )}

      {assignTarget && (
        <AssignBoxModal
          boxName={assignTarget.name}
          users={assignUsers}
          userId={assignUserId}
          onUserId={setAssignUserId}
          error={assignError}
          busy={assignBusy}
          onSubmit={submitAssign}
          onClose={() => setAssignTarget(null)}
        />
      )}

      <toast.ToastContainer />
    </div>
  );
}
