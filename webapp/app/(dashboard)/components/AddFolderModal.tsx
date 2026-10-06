'use client';

import { useState } from 'react';
import { X } from '@phosphor-icons/react';

interface AddFolderModalProps {
  onClose: () => void;
  onSuccess: () => void;
  parentFolderId?: number;
  parentFolderName?: string;
}

export default function AddFolderModal({ onClose, onSuccess, parentFolderId, parentFolderName }: AddFolderModalProps) {
  const [folderName, setFolderName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const res = await fetch('/api/folders', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: folderName,
          parentFolderId: parentFolderId || undefined,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to create folder');
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed to create folder');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[10000] p-4">
      <div className="cyber-card max-w-md w-full p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold neon-text-pink">
            {parentFolderId ? `New Subfolder in "${parentFolderName}"` : 'New Folder'}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-neon-cyan transition-colors"
          >
            <X size={24} weight="bold" />
          </button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="block text-sm font-bold mb-2 text-neon-cyan">
              Folder Name
            </label>
            <input
              type="text"
              value={folderName}
              onChange={(e) => setFolderName(e.target.value)}
              className="input-vapor w-full"
              placeholder="Enter folder name"
              autoFocus
              required
            />
            {parentFolderId && (
              <p className="text-xs text-gray-400 mt-2">
                This folder will be created inside &quot;{parentFolderName}&quot;
              </p>
            )}
          </div>

          {error && (
            <div className="mb-4 p-3 rounded-lg bg-red-500/20 border border-red-500/50">
              <p className="text-sm text-status-error">{error}</p>
            </div>
          )}

          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded-lg font-bold border-2 border-gray-600 text-gray-300 hover:border-gray-500 transition-colors"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="flex-1 cyber-button"
              disabled={loading || !folderName.trim()}
            >
              {loading ? 'Creating...' : 'Create Folder'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
