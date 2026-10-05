'use client';

import { useCallback, useEffect, useState } from 'react';
import { Globe, Plus, ArrowsClockwise, Gear, WarningCircle } from '@phosphor-icons/react';
import AddDnsZoneModal from '../components/AddDnsZoneModal';
import AddDnsRecordModal from '../components/AddDnsRecordModal';
import ConfirmDeleteModal from '../components/ConfirmDeleteModal';
import CloudflareSetup from './CloudflareSetup';
import ZoneSslControls from './ZoneSslControls';

export default function DnsPanel() {
  const [dnsTab, setDnsTab] = useState<'config' | 'zones' | 'records'>('config');
  const [config, setConfig] = useState<any>(null);
  const [zones, setZones] = useState<any[]>([]);
  const [records, setRecords] = useState<any[]>([]);
  const [selectedZone, setSelectedZone] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [showAddZoneModal, setShowAddZoneModal] = useState(false);
  const [showAddRecordModal, setShowAddRecordModal] = useState(false);
  const [deleteZone, setDeleteZone] = useState<{ id: number; domain: string } | null>(null);
  const [deleteRecord, setDeleteRecord] = useState<{ id: number; name: string; type: string } | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const loadConfig = useCallback(async () => {
    try {
      const res = await fetch('/api/dns/config');
      const data = await res.json();
      setConfig(data);
    } catch (error) {
      console.error('Error loading config:', error);
    }
  }, []);

  const loadZones = useCallback(async () => {
    try {
      const res = await fetch('/api/dns/zones');
      const data = await res.json();
      setZones(data.zones || []);
      if (data.zones?.length > 0 && !selectedZone) {
        setSelectedZone(data.zones[0].id);
      }
    } catch (error) {
      console.error('Error loading zones:', error);
    }
  }, [selectedZone]);

  const loadRecords = useCallback(async (zoneId: number) => {
    try {
      const res = await fetch(`/api/dns/records?zone_id=${zoneId}`);
      const data = await res.json();
      setRecords(data.records || []);
    } catch (error) {
      console.error('Error loading records:', error);
    }
  }, []);

  useEffect(() => {
    loadConfig();
    loadZones();
  }, [loadConfig, loadZones]);

  useEffect(() => {
    if (selectedZone && dnsTab === 'records') {
      loadRecords(selectedZone);
    }
  }, [selectedZone, dnsTab, loadRecords]);

  const importZones = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/dns/zones/import', { method: 'POST' });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Import failed');
      alert(
        data.imported?.length
          ? `Imported ${data.imported.length} domain(s): ${data.imported.join(', ')}`
          : `All ${data.total} of your Cloudflare domains are already here.`
      );
      loadZones();
    } catch (error: any) {
      alert(`Error: ${error.message}`);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteZone = async () => {
    if (!deleteZone) return;
    setDeleteLoading(true);
    try {
      const res = await fetch(`/api/dns/zones?id=${deleteZone.id}`, {
        method: 'DELETE'
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to delete zone');
      }

      loadZones();
      setDeleteZone(null);
      alert('✓ DNS zone deleted successfully');
    } catch (error: any) {
      alert(`Error: ${error.message}`);
    } finally {
      setDeleteLoading(false);
    }
  };

  const handleDeleteRecord = async () => {
    if (!deleteRecord) return;
    setDeleteLoading(true);
    try {
      const res = await fetch(`/api/dns/records?id=${deleteRecord.id}`, {
        method: 'DELETE'
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to delete record');
      }

      if (selectedZone) loadRecords(selectedZone);
      setDeleteRecord(null);
      alert('✓ DNS record deleted successfully');
    } catch (error: any) {
      alert(`Error: ${error.message}`);
    } finally {
      setDeleteLoading(false);
    }
  };

  const syncRecords = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/dns/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
      });

      const data = await res.json();
      if (res.ok) {
        alert(`✓ Success: ${data.message}\n\n${data.results?.map((r: any) =>
          `${r.zone}: ${r.records} records (${r.status})`
        ).join('\n') || ''}`);
        loadZones();
        if (selectedZone) loadRecords(selectedZone);
      } else {
        let errorMsg = data.error;
        if (errorMsg.includes('API token not configured')) {
          errorMsg = 'Cloudflare API token not configured.\n\nPlease go to the Configuration tab and add your API token first.';
        } else if (errorMsg.includes('No zones to sync')) {
          errorMsg = 'No DNS zones configured.\n\nPlease add a DNS zone first using the "Add Zone" button in the Zones tab.';
        } else if (errorMsg.includes('integration is disabled')) {
          errorMsg = 'Cloudflare integration is disabled.\n\nPlease enable it in the Configuration tab.';
        }
        alert(errorMsg);
      }
    } catch (error) {
      console.error('Error syncing records:', error);
      alert('Failed to sync records. Please check your network connection and try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Globe size={22} weight="duotone" />
            DNS Management
          </h2>
          <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            Zones, records, and Cloudflare sync.
          </p>
        </div>
        {config?.hasToken && dnsTab !== 'config' && (
          <button
            onClick={syncRecords}
            disabled={loading}
            className="cyber-button flex items-center gap-2"
          >
            <ArrowsClockwise size={20} weight="duotone" />
            Sync from Cloudflare
          </button>
        )}
      </div>

      <div className="flex gap-2 border-b border-neon-purple/30">
        <button
          onClick={() => setDnsTab('config')}
          className={`px-4 py-2 font-bold transition-colors ${
            dnsTab === 'config'
              ? 'border-b-2 border-neon-pink text-neon-pink'
              : 'text-gray-400 hover:text-neon-cyan'
          }`}
        >
          <Gear size={18} weight="duotone" className="inline mr-2" />
          Configuration
        </button>
        <button
          onClick={() => setDnsTab('zones')}
          className={`px-4 py-2 font-bold transition-colors ${
            dnsTab === 'zones'
              ? 'border-b-2 border-neon-pink text-neon-pink'
              : 'text-gray-400 hover:text-neon-cyan'
          }`}
        >
          <Globe size={18} weight="duotone" className="inline mr-2" />
          Zones
        </button>
        <button
          onClick={() => setDnsTab('records')}
          className={`px-4 py-2 font-bold transition-colors ${
            dnsTab === 'records'
              ? 'border-b-2 border-neon-pink text-neon-pink'
              : 'text-gray-400 hover:text-neon-cyan'
          }`}
        >
          DNS Records
        </button>
      </div>

      <div className="cyber-card p-6">
        {dnsTab === 'config' && (
          <CloudflareSetup config={config} onSaved={loadConfig} />
        )}
        {dnsTab === 'zones' && (
          <ZonesTab
            zones={zones}
            canImport={Boolean(config?.hasToken && config?.enabled)}
            importing={loading}
            onImport={importZones}
            onAddZone={() => setShowAddZoneModal(true)}
            onDeleteZone={(id: number, domain: string) => setDeleteZone({ id, domain })}
          />
        )}
        {dnsTab === 'records' && (
          <RecordsTab
            records={records}
            zones={zones}
            selectedZone={selectedZone}
            onZoneChange={setSelectedZone}
            onAddRecord={() => setShowAddRecordModal(true)}
            onDeleteRecord={(id: number, name: string, type: string) => setDeleteRecord({ id, name, type })}
          />
        )}
      </div>

      {showAddZoneModal && (
        <AddDnsZoneModal
          onClose={() => setShowAddZoneModal(false)}
          onSuccess={() => {
            loadZones();
            setDnsTab('zones');
          }}
        />
      )}

      {showAddRecordModal && (
        <AddDnsRecordModal
          zones={zones}
          selectedZone={selectedZone}
          onClose={() => setShowAddRecordModal(false)}
          onSuccess={() => {
            if (selectedZone) loadRecords(selectedZone);
            setDnsTab('records');
          }}
        />
      )}

      {deleteZone && (
        <ConfirmDeleteModal
          title="Delete DNS Zone"
          message="Are you sure you want to delete this DNS zone? All associated DNS records will also be removed."
          itemName={deleteZone.domain}
          onConfirm={handleDeleteZone}
          onCancel={() => setDeleteZone(null)}
          loading={deleteLoading}
        />
      )}

      {deleteRecord && (
        <ConfirmDeleteModal
          title="Delete DNS Record"
          message="Are you sure you want to delete this DNS record?"
          itemName={`${deleteRecord.type} ${deleteRecord.name}`}
          onConfirm={handleDeleteRecord}
          onCancel={() => setDeleteRecord(null)}
          loading={deleteLoading}
        />
      )}
    </div>
  );
}

function ZonesTab({ zones, canImport, importing, onImport, onAddZone, onDeleteZone }: any) {
  const [sslZone, setSslZone] = useState<number | null>(null);
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap justify-between items-center gap-2 mb-4">
        <h2 className="text-xl font-bold text-neon-cyan">Domains</h2>
        <div className="flex gap-2">
          {canImport && (
            <button onClick={onImport} disabled={importing} className="cyber-button-sm flex items-center gap-2">
              <ArrowsClockwise size={16} weight="duotone" />
              {importing ? 'Importing…' : 'Import from Cloudflare'}
            </button>
          )}
          <button onClick={onAddZone} className="cyber-button-sm flex items-center gap-2">
            <Plus size={16} weight="duotone" />
            Add manually
          </button>
        </div>
      </div>

      {zones.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          {canImport
            ? 'No domains yet — click “Import from Cloudflare” to bring them all in.'
            : 'No domains yet — connect Cloudflare in the Configuration tab first.'}
        </div>
      ) : (
        zones.map((zone: any) => (
          <div key={zone.id} className="p-4 bg-dark-bg/50 rounded-lg border border-neon-purple/20 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 className="font-bold text-neon-cyan">{zone.domain}</h3>
              <p className="text-xs text-gray-500 font-mono">Zone ID: {zone.zone_id}</p>
              {zone.last_synced_at && (
                <p className="text-xs text-gray-500">
                  Last synced: {new Date(zone.last_synced_at).toLocaleString()}
                </p>
              )}
            </div>
            <div className="flex gap-2">
              {canImport && (
                <button
                  onClick={() => setSslZone(sslZone === zone.id ? null : zone.id)}
                  className="cyber-button-sm"
                >
                  {sslZone === zone.id ? 'Hide SSL' : 'SSL settings'}
                </button>
              )}
              <button
                onClick={() => onDeleteZone(zone.id, zone.domain)}
                className="cyber-button-sm bg-red-500/20 hover:bg-red-500/30 border border-red-500/30"
                style={{ color: 'var(--status-error)' }}
              >
                Delete
              </button>
            </div>
          </div>
          {sslZone === zone.id && <ZoneSslControls zoneId={zone.id} />}
          </div>
        ))
      )}
    </div>
  );
}

function RecordsTab({ records, zones, selectedZone, onZoneChange, onAddRecord, onDeleteRecord }: any) {
  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center mb-4">
        <div className="flex items-center gap-4">
          <h2 className="text-xl font-bold text-neon-cyan">DNS Records</h2>
          {zones.length > 0 && (
            <select
              value={selectedZone || ''}
              onChange={(e) => onZoneChange(parseInt(e.target.value))}
              className="input-vapor"
            >
              {zones.map((zone: any) => (
                <option key={zone.id} value={zone.id}>
                  {zone.domain}
                </option>
              ))}
            </select>
          )}
        </div>
        {zones.length === 0 ? (
          <div
            className="px-4 py-2 rounded-lg text-sm font-bold"
            style={{
              background: 'rgba(var(--status-warning-rgb), 0.1)',
              border: '1px solid rgba(var(--status-warning-rgb), 0.3)',
              color: 'var(--status-warning)',
            }}
          >
            <span className="inline-flex items-center gap-2">
              <WarningCircle size={14} weight="duotone" />
              Add a zone first to create records
            </span>
          </div>
        ) : (
          <button
            onClick={onAddRecord}
            className="cyber-button-sm flex items-center gap-2"
          >
            <Plus size={16} weight="duotone" />
            Add Record
          </button>
        )}
      </div>

      {zones.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-400 mb-4">
            No zones configured yet.
          </p>
          <p className="text-sm text-gray-500">
            Go to the <span className="text-neon-cyan font-bold">Zones</span> tab to add a DNS zone first.
          </p>
        </div>
      ) : records.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          No DNS records found. Try syncing from Cloudflare or add a record manually.
        </div>
      ) : (
        <div className="space-y-2">
          {records.map((record: any) => (
            <div
              key={record.id}
              className="flex items-center justify-between p-4 bg-dark-bg/50 rounded-lg border border-neon-purple/20"
            >
              <div className="flex-1">
                <div className="flex items-center gap-3">
                  <span className="px-2 py-1 bg-neon-purple/20 text-neon-purple text-xs font-bold rounded">
                    {record.type}
                  </span>
                  <span className="font-bold text-neon-cyan">{record.name}</span>
                  {record.proxied === 1 && (
                    <span className="px-2 py-1 bg-orange-500/20 text-orange-400 text-xs font-bold rounded">
                      PROXIED
                    </span>
                  )}
                </div>
                <p className="text-sm text-gray-400 mt-1">{record.content}</p>
                <p className="text-xs text-gray-500">
                  TTL: {record.ttl === 1 ? 'Auto' : `${record.ttl}s`}
                  {record.priority && ` • Priority: ${record.priority}`}
                </p>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => onDeleteRecord(record.id, record.name, record.type)}
                  className="cyber-button-sm bg-red-500/20 hover:bg-red-500/30 border border-red-500/30"
                  style={{ color: 'var(--status-error)' }}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
