import React, { useEffect, useState } from 'react';
import { useStore } from '../store';
import {
  FiTrash2,
  FiSearch,
  FiChevronLeft,
  FiChevronRight,
  FiX,
  FiHardDrive,
  FiClock,
  FiLoader,
} from 'react-icons/fi';
import { DownloadRecord } from '../lib/api';

const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

const formatDate = (dateStr: string): string => {
  const d = new Date(dateStr);
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

export const CacheManager: React.FC = () => {
  const {
    cachedFilesPaged,
    cachedFilesTotal,
    cachePage,
    cacheKeyword,
    cacheStats,
    loadCachedFilesPaged,
    loadCacheStats,
    setCachePage,
    setCacheKeyword,
    deleteCacheById,
    deleteCacheBatch,
    addToast,
  } = useStore();

  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [searchInput, setSearchInput] = useState(cacheKeyword);
  const [deleting, setDeleting] = useState(false);

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(cachedFilesTotal / pageSize));

  useEffect(() => {
    loadCachedFilesPaged(1);
    loadCacheStats();
  }, []);

  const handleSearch = () => {
    setSelected(new Set());
    loadCachedFilesPaged(1, searchInput);
  };

  const handlePageChange = (page: number) => {
    setSelected(new Set());
    loadCachedFilesPaged(page);
  };

  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selected.size === cachedFilesPaged.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(cachedFilesPaged.map((r) => r.id)));
    }
  };

  const handleDeleteSelected = async () => {
    if (selected.size === 0 || deleting) return;
    setDeleting(true);
    const count = await deleteCacheBatch(Array.from(selected));
    setSelected(new Set());
    setDeleting(false);
    if (count > 0) {
      addToast({ message: `Deleted ${count} file(s)`, type: 'success' });
    }
  };

  const handleDeleteSingle = async (id: number) => {
    if (deleting) return;
    setDeleting(true);
    await deleteCacheById(id);
    setSelected((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    setDeleting(false);
    addToast({ message: 'File deleted', type: 'success' });
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <span className="cache-status completed">✓</span>;
      case 'downloading':
        return <FiLoader size={12} className="spin" />;
      case 'pending':
        return <FiClock size={12} />;
      case 'failed':
        return <FiX size={12} className="cache-status failed" />;
      default:
        return null;
    }
  };

  return (
    <div className="cache-manager">
      <div className="cache-manager-header">
        <h2>Cache Manager</h2>
        <div className="cache-stats-row">
          <span className="cache-stat">
            <FiHardDrive size={14} /> {cacheStats.total} files · {formatSize(cacheStats.totalSize)}
          </span>
          {cacheStats.pending > 0 && (
            <span className="cache-stat pending">
              <FiClock size={14} /> {cacheStats.pending} in queue
            </span>
          )}
        </div>
      </div>

      <div className="cache-manager-toolbar">
        <div className="cache-search">
          <input
            type="text"
            className="cache-search-input"
            placeholder="Search by name..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          />
          <button className="btn btn-sm btn-secondary" onClick={handleSearch}>
            <FiSearch size={14} />
          </button>
        </div>
        <div className="cache-toolbar-actions">
          {selected.size > 0 && (
            <button
              className="btn btn-sm btn-danger"
              onClick={handleDeleteSelected}
              disabled={deleting}
            >
              <FiTrash2 size={14} /> Delete ({selected.size})
            </button>
          )}
        </div>
      </div>

      <div className="cache-table-wrapper">
        <table className="cache-table">
          <thead>
            <tr>
              <th className="cache-th-check">
                <input
                  type="checkbox"
                  checked={cachedFilesPaged.length > 0 && selected.size === cachedFilesPaged.length}
                  onChange={toggleSelectAll}
                />
              </th>
              <th>Name</th>
              <th>Size</th>
              <th>Type</th>
              <th>Date</th>
              <th className="cache-th-actions"></th>
            </tr>
          </thead>
          <tbody>
            {cachedFilesPaged.length === 0 ? (
              <tr>
                <td colSpan={6} className="cache-empty">
                  No cached files
                </td>
              </tr>
            ) : (
              cachedFilesPaged.map((record) => (
                <tr key={record.id} className={selected.has(record.id) ? 'selected' : ''}>
                  <td className="cache-td-check">
                    <input
                      type="checkbox"
                      checked={selected.has(record.id)}
                      onChange={() => toggleSelect(record.id)}
                    />
                  </td>
                  <td className="cache-td-name">
                    <span className="cache-name-text" title={record.videoName}>
                      {record.videoName}
                    </span>
                    <span className="cache-status-icon">{getStatusIcon(record.status)}</span>
                  </td>
                  <td className="cache-td-size">{formatSize(record.size)}</td>
                  <td className="cache-td-type">{record.isHLS ? 'HLS' : 'MP4'}</td>
                  <td className="cache-td-date">{formatDate(record.updatedAt)}</td>
                  <td className="cache-td-actions">
                    <button
                      className="btn btn-xs btn-icon btn-danger-icon"
                      onClick={() => handleDeleteSingle(record.id)}
                      disabled={deleting}
                      title="Delete"
                    >
                      <FiTrash2 size={12} />
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="cache-pagination">
          <button
            className="btn btn-sm btn-secondary"
            onClick={() => handlePageChange(cachePage - 1)}
            disabled={cachePage <= 1}
          >
            <FiChevronLeft size={14} />
          </button>
          <span className="cache-page-info">
            {cachePage} / {totalPages}
          </span>
          <button
            className="btn btn-sm btn-secondary"
            onClick={() => handlePageChange(cachePage + 1)}
            disabled={cachePage >= totalPages}
          >
            <FiChevronRight size={14} />
          </button>
        </div>
      )}
    </div>
  );
};
