'use client';

import { useEffect, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { createPortal } from 'react-dom';
import { Lightning, WarningCircle } from '@phosphor-icons/react';

interface User {
  id: number;
  username: string;
  is_admin: number;
  role: 'super_admin' | 'admin' | 'user';
  is_super_admin: number;
  managed_by: number | null;
  created_at: string;
}

export default function ManageUsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [currentUser, setCurrentUser] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [menuUserId, setMenuUserId] = useState<number | null>(null);
  // The actions menu is portaled to <body> and positioned from the button's
  // on-screen rect: inside the table it was clipped by the horizontally
  // scrolling wrapper, and .cyber-card's backdrop-filter would also turn a
  // fixed child's coordinates relative to the card.
  const [menuPos, setMenuPos] = useState<{ top?: number; bottom?: number; right: number } | null>(null);
  const [detailsUser, setDetailsUser] = useState<User | null>(null);
  const [deleteUserTarget, setDeleteUserTarget] = useState<User | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const [passwordUser, setPasswordUser] = useState<User | null>(null);
  const [passwordValue, setPasswordValue] = useState('');
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [passwordError, setPasswordError] = useState('');
  const [createForm, setCreateForm] = useState({
    username: '',
    password: '',
    confirmPassword: '',
    isAdmin: false,
  });
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState('');

  const fetchUsers = async () => {
    try {
      const [usersRes, meRes] = await Promise.all([
        fetch('/api/users'),
        fetch('/api/auth/me'),
      ]);
      if (!usersRes.ok) throw new Error('Failed to fetch users');

      const usersData = await usersRes.json();
      setUsers(usersData.users);
      if (meRes.ok) {
        const meData = await meRes.json();
        setCurrentUser(meData.user);
      }
      setLoading(false);
    } catch (err) {
      setError('Failed to load users');
      setLoading(false);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateError('');

    if (createForm.password !== createForm.confirmPassword) {
      setCreateError('Passwords do not match');
      return;
    }

    setCreateLoading(true);

    try {
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: createForm.username,
          password: createForm.password,
          isAdmin: createForm.isAdmin,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        setCreateError(data.error || 'Failed to create user');
        setCreateLoading(false);
        return;
      }

      setShowCreateModal(false);
      setCreateForm({ username: '', password: '', confirmPassword: '', isAdmin: false });
      fetchUsers();
    } catch (err) {
      setCreateError('An error occurred. Please try again.');
    } finally {
      setCreateLoading(false);
    }
  };

  const handleDeleteUser = async () => {
    if (!deleteUserTarget) return;
    setDeleteLoading(true);
    setDeleteError('');
    try {
      const res = await fetch(`/api/users?id=${deleteUserTarget.id}`, {
        method: 'DELETE',
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to delete user');
      }
      setDeleteUserTarget(null);
      fetchUsers();
    } catch (err: any) {
      setDeleteError(err.message || 'Failed to delete user');
    } finally {
      setDeleteLoading(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!passwordUser) return;
    setPasswordLoading(true);
    setPasswordError('');
    try {
      const res = await fetch('/api/users/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          userId: passwordUser.id,
          newPassword: passwordValue,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to change password');
      }
      setPasswordUser(null);
      setPasswordValue('');
    } catch (err: any) {
      setPasswordError(err.message || 'Failed to change password');
    } finally {
      setPasswordLoading(false);
    }
  };

  const canDeleteUser = (user: User) => {
    if (!currentUser) return false;
    if (currentUser.role === 'super_admin') {
      return user.id !== currentUser.userId;
    }
    if (currentUser.role === 'admin') {
      return user.role === 'user' && user.id !== currentUser.userId;
    }
    return false;
  };

  const managerMap = new Map<number, string>();
  for (const user of users) {
    managerMap.set(user.id, user.username);
  }

  useEffect(() => {
    if (menuUserId === null) return;
    const close = () => setMenuUserId(null);
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') close(); };
    const onPointer = (e: MouseEvent) => {
      if (!(e.target as Element).closest('[data-user-menu]')) close();
    };
    window.addEventListener('scroll', close, true);
    window.addEventListener('resize', close);
    window.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onPointer);
    return () => {
      window.removeEventListener('scroll', close, true);
      window.removeEventListener('resize', close);
      window.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onPointer);
    };
  }, [menuUserId]);

  const toggleMenu = (userId: number, e: ReactMouseEvent<HTMLButtonElement>) => {
    if (menuUserId === userId) {
      setMenuUserId(null);
      return;
    }
    const rect = e.currentTarget.getBoundingClientRect();
    const right = window.innerWidth - rect.right;
    // Open upward when there isn't room for the menu below the button.
    const spaceBelow = window.innerHeight - rect.bottom;
    setMenuPos(spaceBelow < 180
      ? { bottom: window.innerHeight - rect.top + 8, right }
      : { top: rect.bottom + 8, right });
    setMenuUserId(userId);
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  if (loading) {
    return (
      <div className="text-center py-16">
        <div className="flex justify-center mb-4 animate-float">
          <Lightning size={48} weight="duotone" color="var(--neon-cyan)" />
        </div>
        <div className="text-2xl font-bold neon-text animate-pulse" style={{ color: 'var(--neon-cyan)' }}>
          Loading users...
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-16">
        <div className="flex justify-center mb-4">
          <WarningCircle size={48} weight="duotone" color="var(--status-error)" />
        </div>
        <div className="text-xl font-bold mb-4" style={{ color: 'var(--status-error)' }}>
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold neon-text-pink">Manage Users</h2>
          <p className="text-xs font-mono mt-2" style={{ color: 'var(--text-secondary)' }}>
            ▸ Manage system users and permissions ◂
          </p>
        </div>
        {currentUser?.isAdmin && (
          <button
            type="button"
            onClick={() => setShowCreateModal(true)}
            className="mt-4 sm:mt-0 cyber-button inline-flex items-center gap-2"
          >
            Create User
          </button>
        )}
      </div>

      <div className="cyber-card">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-neon-purple/20">
          <thead className="bg-dark-bg/50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-bold uppercase tracking-wider text-neon-cyan">
                Username
              </th>
              <th className="px-6 py-3 text-left text-xs font-bold uppercase tracking-wider text-neon-cyan">
                Role
              </th>
              <th className="px-6 py-3 text-left text-xs font-bold uppercase tracking-wider text-neon-cyan">
                Created
              </th>
              <th className="px-6 py-3 text-right text-xs font-bold uppercase tracking-wider text-neon-cyan">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neon-purple/20">
            {users.map((user) => (
              <tr key={user.id} className="relative">
                <td className="px-6 py-4 whitespace-nowrap font-bold text-neon-cyan">
                  {user.username}
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  {user.is_admin === 1 ? (
                    <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-neon-purple/20 text-neon-pink">
                      {user.role === 'super_admin' ? 'Superadmin' : 'Admin'}
                    </span>
                  ) : (
                    <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-dark-bg/60 text-neon-cyan">
                      User
                    </span>
                  )}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-400">
                  {new Date(user.created_at).toLocaleDateString()}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-right">
                  <div className="relative inline-flex">
                    <button
                      type="button"
                      data-user-menu
                      onClick={(e) => toggleMenu(user.id, e)}
                      className="group inline-flex flex-col items-center justify-center gap-1"
                      aria-label="User actions"
                    >
                      <span className="h-1.5 w-1.5 rounded-full bg-neon-cyan group-hover:bg-neon-pink transition-colors" />
                      <span className="h-1.5 w-1.5 rounded-full bg-neon-cyan group-hover:bg-neon-pink transition-colors" />
                      <span className="h-1.5 w-1.5 rounded-full bg-neon-cyan group-hover:bg-neon-pink transition-colors" />
                    </button>
                    {menuUserId === user.id && menuPos && createPortal(
                      <div
                        data-user-menu
                        style={menuPos}
                        className="fixed w-40 rounded-lg border border-neon-purple/30 bg-dark-bg/95 backdrop-blur-md shadow-lg z-[9000] flex flex-col"
                      >
                        <button
                          type="button"
                          onClick={() => {
                            setDetailsUser(user);
                            setMenuUserId(null);
                          }}
                          className="w-full text-left px-4 py-2 text-sm text-neon-cyan hover:bg-neon-purple/10"
                        >
                          Details
                        </button>
                        {canDeleteUser(user) && (
                          <button
                            type="button"
                            onClick={() => {
                              setDeleteUserTarget(user);
                              setMenuUserId(null);
                            }}
                            className="w-full text-left px-4 py-2 text-sm text-status-error hover:bg-status-error/10"
                          >
                            Delete
                          </button>
                        )}
                        {currentUser?.role === 'super_admin' && (
                          <button
                            type="button"
                            onClick={() => {
                              setPasswordUser(user);
                              setPasswordValue('');
                              setPasswordError('');
                              setMenuUserId(null);
                            }}
                            className="w-full text-left px-4 py-2 text-sm text-neon-cyan hover:bg-neon-purple/10"
                          >
                            Change Password
                          </button>
                        )}
                      </div>,
                      document.body
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
          </table>
        </div>
      </div>

      {detailsUser && (
        <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div
            className="card-vapor max-w-lg w-full p-6 relative"
            style={{
              background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
              border: '1px solid rgba(var(--neon-pink-rgb), 0.3)',
              boxShadow: '0 0 30px rgba(var(--neon-purple-rgb), 0.4)',
            }}
          >
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-2xl font-bold text-neon-cyan">User Details</h2>
                <p className="text-xs font-mono mt-2" style={{ color: 'var(--text-secondary)' }}>
                  ▸ Account information ◂
                </p>
              </div>
              <button
                type="button"
                onClick={() => setDetailsUser(null)}
                className="cyber-button-sm"
              >
                ✕
              </button>
            </div>

            <div className="space-y-3 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Username</span>
                <span className="text-neon-cyan font-bold">{detailsUser.username}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Role</span>
                <span className="text-neon-pink font-bold">
                  {detailsUser.role === 'super_admin' ? 'Superadmin' : detailsUser.role === 'admin' ? 'Admin' : 'User'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Created</span>
                <span className="text-gray-200">
                  {new Date(detailsUser.created_at).toLocaleString()}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Managed By</span>
                <span className="text-gray-200">
                  {detailsUser.managed_by ? managerMap.get(detailsUser.managed_by) || `User #${detailsUser.managed_by}` : '—'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-400">User ID</span>
                <span className="text-gray-200">{detailsUser.id}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {deleteUserTarget && (
        <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div
            className="card-vapor max-w-md w-full p-6 relative"
            style={{
              background: 'linear-gradient(135deg, rgba(var(--status-error-rgb), 0.25) 0%, rgba(var(--status-error-rgb), 0.45) 100%)',
              border: '2px solid var(--status-error)',
              boxShadow: '0 0 30px rgba(var(--status-error-rgb), 0.5)',
            }}
          >
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-2xl font-bold" style={{ color: 'var(--status-error)' }}>
                Delete User
              </h2>
              <button
                type="button"
                onClick={() => setDeleteUserTarget(null)}
                className="cyber-button-sm"
              >
                ✕
              </button>
            </div>
            <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
              Are you sure you want to delete this user? This action cannot be undone.
            </p>
            {deleteUserTarget && (
              <div className="mb-4 text-sm text-gray-300">
                Sites will be transferred to{' '}
                <span className="text-neon-cyan font-bold">
                  {deleteUserTarget.managed_by
                    ? managerMap.get(deleteUserTarget.managed_by) || `User #${deleteUserTarget.managed_by}`
                    : currentUser?.username || 'the deleting user'}
                </span>
                .
              </div>
            )}
            {deleteError && (
              <div
                className="mb-4 px-3 py-2 rounded-lg text-sm"
                style={{
                  background: 'rgba(var(--status-error-rgb), 0.15)',
                  border: '1px solid rgba(var(--status-error-rgb), 0.5)',
                  color: 'var(--status-error)',
                }}
              >
                {deleteError}
              </div>
            )}
            <div className="mb-6 p-3 rounded-lg text-sm" style={{ background: 'rgba(var(--status-error-rgb), 0.1)', color: 'var(--status-error)' }}>
              {deleteUserTarget.username}
            </div>
            <div className="flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setDeleteUserTarget(null)}
                className="cyber-button-sm"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleDeleteUser}
                disabled={deleteLoading}
                className="cyber-button disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {deleteLoading ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}

      {passwordUser && (
        <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div
            className="card-vapor max-w-md w-full p-6 relative"
            style={{
              background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
              border: '1px solid rgba(var(--neon-cyan-rgb), 0.3)',
              boxShadow: '0 0 30px rgba(var(--neon-cyan-rgb), 0.25)',
            }}
          >
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-2xl font-bold text-neon-cyan">Change Password</h2>
              <button
                type="button"
                onClick={() => setPasswordUser(null)}
                className="cyber-button-sm"
              >
                ✕
              </button>
            </div>
            <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
              Set a new password for <span className="text-neon-pink font-bold">{passwordUser.username}</span>.
            </p>
            {passwordError && (
              <div
                className="mb-4 px-3 py-2 rounded-lg text-sm"
                style={{
                  background: 'rgba(var(--status-error-rgb), 0.15)',
                  border: '1px solid rgba(var(--status-error-rgb), 0.5)',
                  color: 'var(--status-error)',
                }}
              >
                {passwordError}
              </div>
            )}
            <form onSubmit={handleChangePassword} className="space-y-4">
              <div>
                <label htmlFor="change-password" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                  New Password
                </label>
                <input
                  id="change-password"
                  type="password"
                  minLength={6}
                  required
                  value={passwordValue}
                  onChange={(e) => setPasswordValue(e.target.value)}
                  className="input-vapor w-full"
                />
              </div>
              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setPasswordUser(null)}
                  className="cyber-button-sm"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={passwordLoading}
                  className="cyber-button disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {passwordLoading ? 'Updating...' : 'Update Password'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showCreateModal && (
        <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div
            className="card-vapor max-w-xl w-full p-6 relative"
            style={{
              background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
              border: '1px solid rgba(var(--neon-pink-rgb), 0.3)',
              boxShadow: '0 0 30px rgba(var(--neon-purple-rgb), 0.4)',
            }}
          >
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-2xl font-bold text-neon-pink">Create New User</h2>
                <p className="text-xs font-mono mt-2" style={{ color: 'var(--text-secondary)' }}>
                  ▸ Add a new account and assign privileges ◂
                </p>
              </div>
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="cyber-button-sm"
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleCreateUser} className="space-y-6">
              {createError && (
                <div
                  className="px-4 py-3 rounded-lg text-sm"
                  style={{
                    background: 'rgba(var(--status-error-rgb), 0.15)',
                    border: '1px solid rgba(var(--status-error-rgb), 0.5)',
                    color: 'var(--status-error)',
                  }}
                >
                  {createError}
                </div>
              )}

              <div>
                <label htmlFor="create-username" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                  Username
                </label>
                <input
                  type="text"
                  id="create-username"
                  required
                  value={createForm.username}
                  onChange={(e) => setCreateForm({ ...createForm, username: e.target.value })}
                  className="input-vapor w-full"
                />
              </div>

              <div>
                <label htmlFor="create-password" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                  Password
                </label>
                <input
                  type="password"
                  id="create-password"
                  required
                  minLength={6}
                  value={createForm.password}
                  onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })}
                  className="input-vapor w-full"
                />
                <p className="mt-2 text-xs font-mono text-gray-400">Minimum 6 characters</p>
              </div>

              <div>
                <label htmlFor="create-confirm-password" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                  Confirm Password
                </label>
                <input
                  type="password"
                  id="create-confirm-password"
                  required
                  value={createForm.confirmPassword}
                  onChange={(e) => setCreateForm({ ...createForm, confirmPassword: e.target.value })}
                  className="input-vapor w-full"
                />
              </div>

              <div className="flex items-start gap-3">
                <input
                  type="checkbox"
                  id="create-is-admin"
                  checked={createForm.isAdmin}
                  onChange={(e) => setCreateForm({ ...createForm, isAdmin: e.target.checked })}
                  className="mt-1 h-4 w-4 accent-cyan-400"
                />
                <label htmlFor="create-is-admin" className="block text-sm" style={{ color: 'var(--text-secondary)' }}>
                  Administrator (full access to all sites and settings)
                </label>
              </div>

              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="cyber-button-sm"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createLoading}
                  className="cyber-button disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {createLoading ? 'Creating...' : 'Create User'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
