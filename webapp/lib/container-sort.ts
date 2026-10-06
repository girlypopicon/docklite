import type { ContainerInfo } from '@/types';

// How containers are ordered on the Containers page: running ones first,
// then by kind, and within the same group in the folder's own (manual)
// order. To add a kind later, give it a rank in KIND_ORDER.

export type ContainerKind = 'site' | 'database' | 'other';

/** Lower comes first. */
export const KIND_ORDER: Record<ContainerKind, number> = {
  site: 0, // websites and web servers
  database: 1,
  other: 2, // everything else
};

export function containerKind(container: ContainerInfo): ContainerKind {
  const labels = container.labels || {};
  const type = labels['docklite.type'];
  if (type === 'static' || type === 'php' || type === 'node') return 'site';
  if (type === 'postgres' || labels['docklite.database']) return 'database';
  return 'other';
}

/** A new array: running first, then site → database → other; ties keep their incoming order. */
export function sortContainers(containers: ContainerInfo[]): ContainerInfo[] {
  return containers
    .map((container, index) => ({ container, index }))
    .sort((a, b) => {
      const activeA = a.container.state === 'running' ? 0 : 1;
      const activeB = b.container.state === 'running' ? 0 : 1;
      if (activeA !== activeB) return activeA - activeB;
      const kindDiff = KIND_ORDER[containerKind(a.container)] - KIND_ORDER[containerKind(b.container)];
      if (kindDiff !== 0) return kindDiff;
      return a.index - b.index;
    })
    .map((entry) => entry.container);
}

interface TreeNode {
  containers: ContainerInfo[];
  children: TreeNode[];
}

/** Sorts the containers of every folder in the tree (folders themselves keep their order). */
export function sortFolderTree<T extends TreeNode>(nodes: T[]): T[] {
  return nodes.map((node) => ({
    ...node,
    containers: sortContainers(node.containers || []),
    children: sortFolderTree((node.children || []) as T[]),
  }));
}
