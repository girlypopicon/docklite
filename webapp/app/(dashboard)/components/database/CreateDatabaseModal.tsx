'use client';

import React, { useState } from 'react';
import { Database, UserCircle, Key, Sparkle, SpinnerGap } from '@phosphor-icons/react';

interface CreateDatabaseModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (connectionInfo: any) => void;
  onError: (error: string) => void;
}

export default function CreateDatabaseModal({ isOpen, onClose, onSuccess, onError }: CreateDatabaseModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    username: 'docklite',
    password: '',
  });
  const [creating, setCreating] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    onError('');

    try {
      const res = await fetch('/api/databases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: formData.name,
          username: formData.username,
          password: formData.password || undefined, // Let backend generate if empty
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to create database');
      }

      const data = await res.json();

      // Reset form
      setFormData({ name: '', username: 'docklite', password: '' });
      onClose();
      onSuccess(data.connection);
    } catch (err: any) {
      onError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleClose = () => {
    if (!creating) {
      setFormData({ name: '', username: 'docklite', password: '' });
      onClose();
    }
  };

  return (
    <div className="mt-6 card-vapor p-6 rounded-xl">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="name" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
            <Database size={16} weight="duotone" />
            DATABASE NAME
          </label>
          <input
            type="text"
            id="name"
            required
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            className="input-vapor w-full"
            placeholder="my_awesome_database"
          />
          <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            Only alphanumeric characters and underscores
          </p>
        </div>

        <div>
          <label htmlFor="username" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--text-purple)' }}>
            <UserCircle size={16} weight="duotone" />
            USERNAME
          </label>
          <input
            type="text"
            id="username"
            required
            value={formData.username}
            onChange={(e) => setFormData({ ...formData, username: e.target.value })}
            className="input-vapor w-full"
            placeholder="docklite"
          />
          <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            Default: docklite
          </p>
        </div>

        <div>
          <label htmlFor="password" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
            <Key size={16} weight="duotone" />
            PASSWORD
          </label>
          <input
            type="password"
            id="password"
            value={formData.password}
            onChange={(e) => setFormData({ ...formData, password: e.target.value })}
            className="input-vapor w-full"
            placeholder="Leave empty for auto-generated password"
          />
          <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            Leave empty to auto-generate a secure password
          </p>
        </div>

        <button
          type="submit"
          disabled={creating}
          className="btn-neon w-full disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {creating ? (
            <span className="inline-flex items-center gap-2">
              <SpinnerGap size={16} weight="duotone" className="animate-spin" />
              Creating...
            </span>
          ) : (
            <span className="inline-flex items-center gap-2">
              <Sparkle size={16} weight="duotone" />
              Create PostgreSQL Database
            </span>
          )}
        </button>
      </form>
    </div>
  );
}
