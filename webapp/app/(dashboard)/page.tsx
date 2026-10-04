'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { ContainerInfo, FolderNode } from '@/types';
import ContainerDetailsModal from './components/ContainerDetailsModal';
import AllContainersModal from './components/AllContainersModal';
import AddFolderModal from './components/AddFolderModal';
import FolderSection from './components/FolderSection';
import SkeletonLoader from './components/SkeletonLoader';
import { useToast } from '@/lib/hooks/useToast';
import { Database, Lightning, Package, ArrowsClockwise, FolderPlus, PlusCircle, WarningCircle, SpinnerGap } from '@phosphor-icons/react';
import AddContainerModal from './components/AddContainerModal';
import ContainerFilterTabs, { type StatusFilter } from './components/ContainerFilterTabs';
import { containerKind, sortFolderTree } from '@/lib/container-sort';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent,
} from '@dnd-kit/core';
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable';

type ContainerType = 'all' | 'sites' | 'databases' | 'other';

export default function DashboardPage() {
  const [foldersData, setFoldersData] = useState<FolderNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [currentUser, setCurrentUser] = useState<{ userId: number; username: string; isAdmin: boolean; role: string } | null>(null);
  const [selectedContainerId, setSelectedContainerId] = useState<string | null>(null);
  const [selectedContainerName, setSelectedContainerName] = useState<string>('');
  const [showAllContainersModal, setShowAllContainersModal] = useState(false);
  const [showAddFolderModal, setShowAddFolderModal] = useState(false);
  const [subfolderParent, setSubfolderParent] = useState<{ id: number; name: string } | null>(null);
  const [showAddContainerModal, setShowAddContainerModal] = useState(false);
  const [filterType, setFilterType] = useState<ContainerType>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('any');
  const [assignTarget, setAssignTarget] = useState<{ id: string; name: string } | null>(null);
  const [assignUsers, setAssignUsers] = useState<Array<{ id: number; username: string }>>([]);
  const [assignUserId, setAssignUserId] = useState<string>('');
  const [assignLoading, setAssignLoading] = useState(false);
  const [assignError, setAssignError] = useState('');
  const [moveTarget, setMoveTarget] = useState<{ id: string; name: string } | null>(null);
  const [moveFolderId, setMoveFolderId] = useState<string>('');
  const [moveError, setMoveError] = useState('');
  const toast = useToast();

  const fetchData = async () => {
    try {
      const res = await fetch('/api/containers');
      if (!res.ok) {
        throw new Error('Failed to fetch data');
      }
      const data = await res.json();
      // Running before stopped, then site → database → other.
      setFoldersData(sortFolderTree<FolderNode>(data.folders || []));
    } catch (err) {
      setError('Failed to load containers');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const loadUser = async () => {
      try {
        const res = await fetch('/api/auth/me');
        if (!res.ok) return;
        const data = await res.json();
        setCurrentUser(data.user || null);
      } catch {
        setCurrentUser(null);
      }
    };
    loadUser();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, []);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const handleAction = async (containerId: string, action: 'start' | 'stop' | 'restart') => {
    try {
      const res = await fetch(`/api/containers/${containerId}/${action}`, {
        method: 'POST',
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `Failed to ${action} container`);
      }
      toast.success(`Container ${action}ed successfully!`);
      fetchData();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const handleDeleteContainer = async (containerId: string, containerName: string) => {
    if (!confirm(`Delete "${containerName}"? This will stop and remove the container.`)) {
      return;
    }

    try {
      const res = await fetch(`/api/containers/${containerId}`, {
        method: 'DELETE',
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to delete container');
      }
      toast.success(`Deleted ${containerName}`);
      fetchData();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const loadAssignUsers = async () => {
    try {
      const res = await fetch('/api/users');
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to load users');
      }
      const data = await res.json();
      setAssignUsers(data.users || []);
    } catch (err: any) {
      setAssignError(err.message || 'Failed to load users');
    }
  };

  const openAssignModal = async (containerId: string, containerName: string) => {
    setAssignTarget({ id: containerId, name: containerName });
    setAssignError('');
    setAssignUserId('');
    if (assignUsers.length === 0) {
      await loadAssignUsers();
    }
  };

  const handleAssignSubmit = async () => {
    if (!assignTarget) return;
    if (!assignUserId) {
      setAssignError('Please select a user to assign this container to.');
      return;
    }
    setAssignLoading(true);
    setAssignError('');

    try {
      const res = await fetch(`/api/containers/${assignTarget.id}/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: Number(assignUserId) }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to assign container');
      }
      toast.success(`Assigned ${assignTarget.name}`);
      setAssignTarget(null);
      setAssignUserId('');
      fetchData();
    } catch (err: any) {
      setAssignError(err.message || 'Failed to assign container');
    } finally {
      setAssignLoading(false);
    }
  };

  const handleToggleTracking = async (containerId: string, tracked: boolean) => {
    try {
      const endpoint = tracked ? 'untrack' : 'track';
      const res = await fetch(`/api/containers/${containerId}/${endpoint}`, {
        method: 'POST',
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `Failed to ${endpoint} container`);
      }
      toast.success(tracked ? 'Container untracked' : 'Container tracked');
      fetchData();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const getContainerType = containerKind;

  const matchesStatus = (container: ContainerInfo) =>
    statusFilter === 'any' || (statusFilter === 'running' ? container.state === 'running' : container.state !== 'running');

  const filterContainers = (containers: ContainerInfo[]): ContainerInfo[] => {
    return containers.filter((container) => {
      if (!matchesStatus(container)) return false;
      const type = getContainerType(container);
      if (filterType === 'sites') return type === 'site';
      if (filterType === 'databases') return type === 'database';
      if (filterType === 'other') return type === 'other';
      return true;
    });
  };

  // Recursively count all containers in the tree
  const countContainers = (nodes: FolderNode[]): number => {
    return nodes.reduce((total, node) => {
      return total + node.containers.length + countContainers(node.children);
    }, 0);
  };

  // Recursively filter folders and their containers
  const filterFolderTree = (nodes: FolderNode[]): FolderNode[] => {
    return nodes.map(node => ({
      ...node,
      containers: filterContainers(node.containers),
      children: filterFolderTree(node.children)
    })).filter(node => {
      // Only hide folders if we're actively filtering AND they have no matches
      if (filterType === 'all' && statusFilter === 'any') {
        return true; // Show all folders when not filtering
      }
      return node.containers.length > 0 || node.children.length > 0;
    });
  };

  const totalContainers = countContainers(foldersData);

  // Tab counts follow the state switch, so each number matches what that tab will show.
  const typeCounts = useMemo(() => {
    const counts = { all: 0, sites: 0, databases: 0, other: 0 };
    const walk = (nodes: FolderNode[]) => {
      for (const node of nodes) {
        for (const container of node.containers) {
          if (!matchesStatus(container)) continue;
          counts.all += 1;
          const type = getContainerType(container);
          if (type === 'site') counts.sites += 1;
          else if (type === 'database') counts.databases += 1;
          else counts.other += 1;
        }
        walk(node.children);
      }
    };
    walk(foldersData);
    return counts;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [foldersData, statusFilter]);
  const filteredFolders = filterFolderTree(foldersData);

  const flattenedFolders = useMemo(() => {
    const result: Array<{ id: number; name: string; depth: number }> = [];
    const walk = (nodes: FolderNode[]) => {
      for (const node of nodes) {
        result.push({ id: node.id, name: node.name, depth: node.depth });
        if (node.children?.length) walk(node.children);
      }
    };
    walk(foldersData);
    return result;
  }, [foldersData]);

  type ContainerLocation = { folderId: number; index: number; container: ContainerInfo };

  const findContainerLocation = (nodes: FolderNode[], containerId: string): ContainerLocation | null => {
    for (const node of nodes) {
      const index = node.containers.findIndex(container => container.id === containerId);
      if (index !== -1) {
        return { folderId: node.id, index, container: node.containers[index] };
      }
      if (node.children?.length) {
        const childResult = findContainerLocation(node.children, containerId);
        if (childResult) return childResult;
      }
    }
    return null;
  };

  const findFolderNode = (nodes: FolderNode[], folderId: number): FolderNode | null => {
    for (const node of nodes) {
      if (node.id === folderId) return node;
      if (node.children?.length) {
        const found = findFolderNode(node.children, folderId);
        if (found) return found;
      }
    }
    return null;
  };

  const updateFolderTree = (
    nodes: FolderNode[],
    folderId: number,
    updater: (containers: ContainerInfo[]) => ContainerInfo[]
  ): FolderNode[] => {
    return nodes.map(node => {
      if (node.id === folderId) {
        return { ...node, containers: updater(node.containers) };
      }
      if (node.children?.length) {
        return { ...node, children: updateFolderTree(node.children, folderId, updater) };
      }
      return node;
    });
  };

  const handleMoveSubmit = async () => {
    if (!moveTarget) return;
    if (!moveFolderId) {
      setMoveError('Please select a folder.');
      return;
    }
    setMoveError('');
    try {
      const res = await fetch(`/api/folders/${moveFolderId}/containers`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ containerId: moveTarget.id }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to move container');
      }
      toast.success(`Moved ${moveTarget.name}`);
      setMoveTarget(null);
      setMoveFolderId('');
      fetchData();
    } catch (err: any) {
      setMoveError(err.message || 'Failed to move container');
    }
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;

    if (!over || active.id === over.id) return;

    const activeId = String(active.id);
    const overId = String(over.id);
    const activeLocation = findContainerLocation(foldersData, activeId);
    if (!activeLocation) return;

    let targetFolderId = activeLocation.folderId;
    let targetIndex = activeLocation.index;

    if (overId.startsWith('folder-')) {
      targetFolderId = Number(overId.replace('folder-', ''));
      const targetFolderData = findFolderNode(foldersData, targetFolderId);
      targetIndex = targetFolderData ? targetFolderData.containers.length : 0;
    } else {
      const overLocation = findContainerLocation(foldersData, overId);
      if (!overLocation) return;
      targetFolderId = overLocation.folderId;
      targetIndex = overLocation.index;
    }

    if (activeLocation.folderId === targetFolderId) {
      if (activeLocation.index === targetIndex) return;
      const adjustedIndex = targetIndex > activeLocation.index ? targetIndex - 1 : targetIndex;
      setFoldersData(prev =>
        updateFolderTree(prev, activeLocation.folderId, containers => {
          const next = [...containers];
          const [moved] = next.splice(activeLocation.index, 1);
          next.splice(adjustedIndex, 0, moved);
          return next;
        })
      );
      try {
        await handleContainerReorder(activeLocation.folderId, activeId, adjustedIndex);
      } catch {
        fetchData();
      }
      return;
    }

    setFoldersData(prev => {
      let moving: ContainerInfo | null = null;
      let next = updateFolderTree(prev, activeLocation.folderId, containers => {
        const remaining = [];
        for (const container of containers) {
          if (container.id === activeId) {
            moving = container;
          } else {
            remaining.push(container);
          }
        }
        return remaining;
      });
      if (!moving) return prev;
      next = updateFolderTree(next, targetFolderId, containers => {
        const nextContainers = [...containers];
        nextContainers.splice(targetIndex, 0, moving!);
        return nextContainers;
      });
      return next;
    });

    try {
      await handleContainerDrop(activeId, targetFolderId);
    } catch {
      fetchData();
    }
  };

  const handleContainerDrop = async (containerId: string, targetFolderId: number) => {
    try {
      // Add container to target folder
      const res = await fetch(`/api/folders/${targetFolderId}/containers`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ containerId }),
      });

      if (!res.ok) throw new Error('Failed to move container to folder');

      toast.success('Container moved successfully!');
      fetchData(); // Refresh to show new organization
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  const handleContainerReorder = async (folderId: number, containerId: string, newPosition: number) => {
    try {
      const res = await fetch(`/api/folders/${folderId}/containers/reorder`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ containerId, newPosition }),
      });

      if (!res.ok) throw new Error('Failed to reorder container');

      toast.success('Container reordered!');
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
      throw err; // Re-throw so FolderSection can revert
    }
  };

  const handleAddSubfolder = (parentId: number, parentName: string) => {
    setSubfolderParent({ id: parentId, name: parentName });
    setShowAddFolderModal(true);
  };

  const handleDeleteFolder = async (folderId: number, folderName: string) => {
    if (!confirm(`Are you sure you want to delete the folder "${folderName}"? This will also delete all subfolders and their container assignments.`)) {
      return;
    }

    try {
      const res = await fetch(`/api/folders/${folderId}`, {
        method: 'DELETE',
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to delete folder');
      }

      toast.success(`Folder "${folderName}" deleted successfully!`);
      fetchData();
    } catch (err: any) {
      toast.error(`Error: ${err.message || err}`);
    }
  };

  if (loading) {
    return (
      <div className="max-w-[1400px] mx-auto">
        <div className="mb-6">
          <h1 className="docklite-containers-title text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Package size={26} weight="duotone" />
            Containers
          </h1>
          <p className="text-xs font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
            <SpinnerGap size={14} weight="duotone" className="animate-spin" />
            Loading...
          </p>
        </div>
        <SkeletonLoader type="card" count={6} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-16">
        <div className="mb-8">
          <div className="flex justify-center mb-4 animate-pulse">
            <WarningCircle size={48} weight="duotone" color="var(--status-error)" />
          </div>
          <div className="text-xl font-bold mb-2" style={{ color: 'var(--status-error)' }}>
            System Error Detected
          </div>
          <button onClick={fetchData} className="btn-neon px-6 py-3 font-bold inline-flex items-center gap-2">
            <ArrowsClockwise size={20} weight="duotone" />
            Retry Connection
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-[1400px] mx-auto">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between mb-6 gap-4">
        <div>
          <h1 className="docklite-containers-title text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Package size={26} weight="duotone" />
            Containers
          </h1>
          <div className="flex items-center gap-3">
            <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
              System status: online
            </p>
            <span className="text-xs font-mono px-2 py-0.5 rounded-full" style={{
              background: 'rgba(var(--status-success-rgb), 0.2)',
              color: 'var(--neon-green)',
              border: '1px solid var(--neon-green)'
            }}>
              {totalContainers} containers
            </span>
          </div>
        </div>
        <div className="flex gap-3">
          <button
            onClick={() => setShowAddContainerModal(true)}
            className="btn-neon inline-flex items-center gap-2"
          >
            <PlusCircle size={20} weight="duotone" />
            New Container
          </button>
          <button
            onClick={() => setShowAddFolderModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 text-sm font-bold rounded-lg transition-all hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, var(--neon-pink) 0%, var(--neon-purple) 100%)',
              color: 'var(--button-text)',
              boxShadow: '0 0 12px rgba(var(--neon-pink-rgb), 0.4)',
            }}
          >
            <FolderPlus size={20} weight="duotone" />
            New Folder
          </button>
          <button
            onClick={() => setShowAllContainersModal(true)}
            className="btn-neon inline-flex items-center gap-2"
          >
            <Package size={20} weight="duotone" />
            All Containers
          </button>
        </div>
      </div>

      {/* Filter: type tabs with counts, and a Running/Stopped switch */}
      <ContainerFilterTabs
        type={filterType}
        onType={setFilterType}
        status={statusFilter}
        onStatus={setStatusFilter}
        counts={typeCounts}
      />

      {totalContainers === 0 ? (
        <div className="mt-12 text-center py-20 card-vapor max-w-3xl mx-auto animate-fade-in">
          <div className="flex justify-center mb-6 animate-float">
            <Package size={80} weight="duotone" style={{ color: 'var(--neon-cyan)', filter: 'drop-shadow(0 0 20px var(--neon-cyan))' }} />
          </div>
          <h2 className="text-3xl font-bold neon-text mb-3" style={{ color: 'var(--neon-pink)' }}>
            Welcome to DockLite!
          </h2>
          <p className="text-lg mb-2" style={{ color: 'var(--text-secondary)' }}>
            No containers detected yet
          </p>
          <p className="text-sm mb-8 max-w-md mx-auto" style={{ color: 'var(--text-muted)' }}>
            Get started by viewing all running containers on your system, or create a new one from our templates.
          </p>
          <div className="flex gap-4 justify-center flex-wrap">
            <button
              onClick={() => setShowAllContainersModal(true)}
              className="btn-neon inline-flex items-center gap-2 text-base px-6 py-3"
            >
              <Package size={22} weight="duotone" />
              View All Containers
            </button>
            <button
              onClick={() => setShowAddContainerModal(true)}
              className="inline-flex items-center gap-2 text-base px-6 py-3 rounded-xl font-bold transition-all hover:scale-105"
              style={{
                background: 'linear-gradient(135deg, var(--neon-purple) 0%, var(--neon-pink) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 20px rgba(var(--neon-purple-rgb), 0.4)'
              }}
            >
              <PlusCircle size={22} weight="duotone" />
              Create Container
            </button>
          </div>
        </div>
      ) : filteredFolders.length === 0 ? (
        <div className="mt-12 text-center py-16 card-vapor max-w-2xl mx-auto animate-fade-in">
          <div className="flex justify-center mb-4 opacity-50">
            <WarningCircle size={64} weight="duotone" style={{ color: 'var(--neon-yellow)' }} />
          </div>
          <p className="text-xl font-bold neon-text mb-3" style={{ color: 'var(--neon-pink)' }}>
            No containers match this filter
          </p>
          <p className="text-sm mb-6" style={{ color: 'var(--text-muted)' }}>
            Try selecting a different filter to see your containers
          </p>
          <button
            onClick={() => {
              setFilterType('all');
              setStatusFilter('any');
            }}
            className="btn-neon inline-flex items-center gap-2"
          >
            <ArrowsClockwise size={20} weight="duotone" />
            Show All Containers
          </button>
        </div>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <div className="space-y-8">
            {filteredFolders.map((folderNode) => (
              <FolderSection
                key={folderNode.id}
                folderNode={folderNode}
                onAction={handleAction}
                onViewDetails={(id, name) => {
                  setSelectedContainerId(id);
                  setSelectedContainerName(name);
                }}
                onDelete={handleDeleteContainer}
                onAssign={openAssignModal}
                canAssign={Boolean(currentUser?.isAdmin)}
                onRefresh={fetchData}
                onMoveFolder={(id, name) => {
                  setMoveTarget({ id, name });
                  setMoveFolderId('');
                  setMoveError('');
                }}
                onToggleTracking={currentUser?.isAdmin ? handleToggleTracking : undefined}
                onAddSubfolder={handleAddSubfolder}
                onDeleteFolder={handleDeleteFolder}
              />
            ))}
          </div>
        </DndContext>
      )}

      {selectedContainerId && (
        <ContainerDetailsModal
          containerId={selectedContainerId}
          containerName={selectedContainerName}
          onClose={() => {
            setSelectedContainerId(null);
            setSelectedContainerName('');
          }}
        />
      )}


      {showAllContainersModal && (
        <AllContainersModal onClose={() => setShowAllContainersModal(false)} />
      )}
      {showAddContainerModal && (
        <AddContainerModal
          onClose={() => setShowAddContainerModal(false)}
          onCreated={() => {
            fetchData();
          }}
        />
      )}

      {assignTarget && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[10000] p-4">
          <div className="cyber-card max-w-md w-full p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-2xl font-bold neon-text-pink">
                Assign Container
              </h2>
              <button
                onClick={() => setAssignTarget(null)}
                className="transition-colors"
                style={{ color: 'var(--text-secondary)' }}
              >
                ✕
              </button>
            </div>
            <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
              Assign <span className="font-bold" style={{ color: 'var(--neon-cyan)' }}>{assignTarget.name}</span> to a user.
            </p>
            <div className="mb-4">
              <label className="block text-sm font-bold mb-2 text-neon-cyan">
                Select User
              </label>
              <select
                value={assignUserId}
                onChange={(e) => setAssignUserId(e.target.value)}
                className="input-vapor w-full"
              >
                <option value="">Choose a user...</option>
                {assignUsers.map((user) => (
                  <option key={user.id} value={String(user.id)}>
                    {user.username}
                  </option>
                ))}
              </select>
            </div>
            {assignError && (
              <div className="mb-4 p-3 rounded-lg" style={{
                background: 'rgba(var(--status-error-rgb), 0.2)',
                border: '1px solid rgba(var(--status-error-rgb), 0.5)'
              }}>
                <p className="text-sm" style={{ color: 'var(--status-error)' }}>{assignError}</p>
              </div>
            )}
            <div className="flex gap-3">
              <button
                type="button"
                onClick={() => setAssignTarget(null)}
                className="flex-1 px-4 py-2 rounded-lg font-bold border-2 transition-colors"
                style={{
                  borderColor: 'var(--surface-dim)',
                  color: 'var(--text-primary)'
                }}
                disabled={assignLoading}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleAssignSubmit}
                className="flex-1 cyber-button"
                disabled={assignLoading || !assignUserId}
              >
                {assignLoading ? 'Assigning...' : 'Assign'}
              </button>
            </div>
          </div>
        </div>
      )}

      {moveTarget && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[10000] p-4">
          <div className="cyber-card max-w-md w-full p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-2xl font-bold neon-text-pink">
                Move Container
              </h2>
              <button
                onClick={() => setMoveTarget(null)}
                className="transition-colors"
                style={{ color: 'var(--text-secondary)' }}
              >
                ✕
              </button>
            </div>
            <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
              Move <span className="font-bold" style={{ color: 'var(--neon-cyan)' }}>{moveTarget.name}</span> to a folder.
            </p>
            <div className="mb-4">
              <label className="block text-sm font-bold mb-2 text-neon-cyan">
                Select Folder
              </label>
              <select
                value={moveFolderId}
                onChange={(e) => setMoveFolderId(e.target.value)}
                className="input-vapor w-full"
              >
                <option value="">Choose a folder...</option>
                {flattenedFolders.map((folder) => (
                  <option key={folder.id} value={String(folder.id)}>
                    {`${'—'.repeat(folder.depth)} ${folder.name}`}
                  </option>
                ))}
              </select>
            </div>
            {moveError && (
              <div className="mb-4 p-3 rounded-lg" style={{
                background: 'rgba(var(--status-error-rgb), 0.2)',
                border: '1px solid rgba(var(--status-error-rgb), 0.5)'
              }}>
                <p className="text-sm" style={{ color: 'var(--status-error)' }}>{moveError}</p>
              </div>
            )}
            <div className="flex gap-3">
              <button
                type="button"
                onClick={() => setMoveTarget(null)}
                className="flex-1 px-4 py-2 rounded-lg font-bold border-2 transition-colors"
                style={{
                  borderColor: 'var(--surface-dim)',
                  color: 'var(--text-primary)'
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleMoveSubmit}
                className="flex-1 cyber-button"
                disabled={!moveFolderId}
              >
                Move
              </button>
            </div>
          </div>
        </div>
      )}

      {showAddFolderModal && (
        <AddFolderModal
          onClose={() => {
            setShowAddFolderModal(false);
            setSubfolderParent(null);
          }}
          onSuccess={() => {
            fetchData();
            toast.success(subfolderParent ? 'Subfolder created successfully!' : 'Folder created successfully!');
            setSubfolderParent(null);
          }}
          parentFolderId={subfolderParent?.id}
          parentFolderName={subfolderParent?.name}
        />
      )}

      <toast.ToastContainer />
    </div>
  );
}
