'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useInSettingsModal, useSettingsModal } from '@/lib/settings-modal';

export default function ChangePasswordPage() {
  const router = useRouter();
  const inModal = useInSettingsModal();
  const { closeSettings } = useSettingsModal();
  const [formData, setFormData] = useState({
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess(false);

    if (formData.newPassword !== formData.confirmPassword) {
      setError('New passwords do not match');
      return;
    }

    setLoading(true);

    try {
      const res = await fetch('/api/users/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          currentPassword: formData.currentPassword,
          newPassword: formData.newPassword,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || 'Failed to change password');
        setLoading(false);
        return;
      }

      setSuccess(true);
      setFormData({ currentPassword: '', newPassword: '', confirmPassword: '' });
      setTimeout(() => (inModal ? closeSettings() : router.push('/')), 2000);
    } catch (err) {
      setError('An error occurred. Please try again.');
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold neon-text-pink">Change Password</h2>
        <p className="text-xs font-mono mt-2" style={{ color: 'var(--text-secondary)' }}>
          ▸ Update your account credentials ◂
        </p>
      </div>

      <div className="cyber-card p-6">
        <form onSubmit={handleSubmit} className="space-y-6">
          {error && (
            <div
              className="px-4 py-3 rounded-lg text-sm"
              style={{
                background: 'rgba(var(--status-error-rgb), 0.15)',
                border: '1px solid rgba(var(--status-error-rgb), 0.5)',
                color: 'var(--status-error)',
              }}
            >
              {error}
            </div>
          )}

          {success && (
            <div
              className="px-4 py-3 rounded-lg text-sm"
              style={{
                background: 'rgba(var(--status-success-rgb), 0.15)',
                border: '1px solid rgba(var(--status-success-rgb), 0.5)',
                color: 'var(--neon-green)',
              }}
            >
              Password changed successfully! Redirecting...
            </div>
          )}

          <div>
            <label htmlFor="currentPassword" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Current Password
            </label>
            <input
              type="password"
              id="currentPassword"
              required
              value={formData.currentPassword}
              onChange={(e) => setFormData({ ...formData, currentPassword: e.target.value })}
              className="input-vapor w-full"
            />
          </div>

          <div>
            <label htmlFor="newPassword" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              New Password
            </label>
            <input
              type="password"
              id="newPassword"
              required
              minLength={6}
              value={formData.newPassword}
              onChange={(e) => setFormData({ ...formData, newPassword: e.target.value })}
              className="input-vapor w-full"
            />
            <p className="mt-2 text-xs font-mono text-gray-400">Minimum 6 characters</p>
          </div>

          <div>
            <label htmlFor="confirmPassword" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Confirm New Password
            </label>
            <input
              type="password"
              id="confirmPassword"
              required
              value={formData.confirmPassword}
              onChange={(e) => setFormData({ ...formData, confirmPassword: e.target.value })}
              className="input-vapor w-full"
            />
          </div>

          <div className="flex justify-end gap-3">
            <button
              type="submit"
              disabled={loading}
              className="cyber-button disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Changing...' : 'Change Password'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
