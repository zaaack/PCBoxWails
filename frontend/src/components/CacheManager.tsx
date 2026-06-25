import React, { useEffect, useState, useMemo } from 'react';
import { useStore, CacheTask } from '../store';
import { DownloadRecord } from '../lib/api';
import {
  FiTrash2,
  FiSearch,
  FiChevronLeft,
  FiChevronRight,
  FiX,
  FiHardDrive,
  FiClock,
  FiLoader,
  FiCheck,
  FiAlertCircle,
  FiList,
  FiPlay,
  FiRefreshCw,
} from 'react-icons/fi';

const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

const formatDate = (dateStr: string): string => {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '';
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

const STATUS_TABS = [
  { key: 'all', label: 'All', icon: FiList },
  { key: 'completed', label: 'Cached', icon: FiCheck },
  { key: 'downloading', label: 'Downloading', icon: FiLoader },
  { key: 'pending', label: 'Pending', icon: FiClock },
  { key: 'failed', label: 'Failed', icon: FiAlertCircle },
];

export const CacheManager: React.FC = () => {
  const {
    cachedFilesPaged,
    cachedFilesTotal,
    cachePage,
    cacheKeyword,
    cacheStatusFilter,
    cacheStats,
    downloadProgress,
    cacheTasks,
    loadCachedFilesPaged,
    loadCacheStats,
    setCachePage,
    setCacheKeyword,
    setCacheStatusFilter,
    deleteCacheById,
    deleteCacheBatch,
    cancelDownload,
    retryDownload,
    playFromCache,
    addToast,
  } = useStore();

  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [searchInput, setSearchInput] = useState(cacheKeyword);
  const [deleting, setDeleting] = useState(false);

  const activeCacheTasks = useMemo(() => {
    return cacheTasks.filter(
      (t) => t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading'
    );
  }, [cacheTasks]);

  const displayRecords = useMemo(() => {
    if (activeCacheTasks.length === 0) return cachedFilesPaged;
    const backendHashes = new Set(cachedFilesPaged.map((r) => r.urlHash));
    const taskRecords: DownloadRecord[] = activeCacheTasks
      .filter((t) => !t.downloadId || !backendHashes.has(t.downloadId))
      .map((t, i) => ({
        id: -(i + 1),
        urlHash: t.downloadId || t.epKey,
        url: t.episode.url,
        headers: '',
        videoName: t.episode.name,
        filePath: '',
        isHLS: false,
        size: 0,
        status: t.status === 'resolving' ? 'pending' : t.status,
        progress: t.progress,
        error: t.error,
        createdAt: '',
        updatedAt: '',
      }));
    const filter = cacheStatusFilter;
    const filteredTasks = filter === 'all' ? taskRecords : taskRecords.filter((r) => r.status === filter);
    if (filteredTasks.length === 0) return cachedFilesPaged;
    return [...filteredTasks, ...cachedFilesPaged];
  }, [activeCacheTasks, cachedFilesPaged, cacheStatusFilter]);

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(cachedFilesTotal / pageSize));

  useEffect(() => {
    loadCachedFilesPaged(1);
    loadCacheStats();
  }, []);

  const prevTaskCountRef = React.useRef(0);
  useEffect(() => {
    const activeCount = cacheTasks.filter(
      (t) => t.status === 'pending' || t.status === 'resolving' || t.status === 'downloading'
    ).length;
    if (activeCount > 0 && prevTaskCountRef.current === 0) {
      loadCachedFilesPaged(cachePage);
      loadCacheStats();
    }
    prevTaskCountRef.current = activeCount;
  }, [cacheTasks]);

  const handleTabChange = (status: string) => {
    setSelected(new Set());
    setCacheStatusFilter(status);
    setCachePage(1);
    setTimeout(() => loadCachedFilesPaged(1), 0);
  };

  const handleSearch = () => {
    setSelected(new Set());
    setCacheKeyword(searchInput);
    setCachePage(1);
    setTimeout(() => loadCachedFilesPaged(1, searchInput), 0);
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
    if (selected.size === displayRecords.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(displayRecords.map((r) => r.id)));
    }
  };

  const handleDeleteSelected = async () => {
    if (selected.size === 0 || deleting) return;
    if (!window.confirm(`Delete ${selected.size} selected file(s)?`)) return;
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
    if (!window.confirm('Delete this cached file?')) return;
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

  const handleCancelSingle = async (urlHash: string) => {
    await cancelDownload(urlHash);
    loadCachedFilesPaged(cachePage);
    loadCacheStats();
    addToast({ message: 'Download cancelled', type: 'info' });
  };

  const handleRetrySingle = async (urlHash: string) => {
    const result = await retryDownload(urlHash);
    if (result) {
      loadCachedFilesPaged(cachePage);
      loadCacheStats();
      addToast({ message: 'Retrying download...', type: 'info' });
    }
  };

  const handlePlay = (record: any) => {
    if (!record.filePath || record.status !== 'completed') return;
    playFromCache(record.filePath, record.isHLS, record.videoName);
  };

  const getStatusDisplay = (record: any) => {
    switch (record.status) {
      case 'completed':
        return <span className="cache-badge badge-completed"><FiCheck size={10} /> Cached</span>;
      case 'downloading': {
        const prog = downloadProgress.get(record.urlHash);
        const pct = prog ? Math.round(prog.progress) : Math.round(record.progress || 0);
        return (
          <span className="cache-badge badge-downloading">
            <FiLoader size={10} className="spin" /> {pct}%
          </span>
        );
      }
      case 'pending':
        return <span className="cache-badge badge-pending"><FiClock size={10} /> Queued</span>;
      case 'failed':
        return <span className="cache-badge badge-failed"><FiX size={10} /> Failed</span>;
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

      <div className="cache-status-tabs">
        {STATUS_TABS.map((tab) => (
          <button
            key={tab.key}
            className={`cache-tab ${cacheStatusFilter === tab.key ? 'active' : ''}`}
            onClick={() => handleTabChange(tab.key)}
          >
            <tab.icon size={13} className={tab.key === 'downloading' ? 'spin' : ''} />
            {tab.label}
          </button>
        ))}
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
                  checked={displayRecords.length > 0 && selected.size === displayRecords.length}
                  onChange={toggleSelectAll}
                />
              </th>
              <th>Name</th>
              <th>Size</th>
              <th>Type</th>
              <th>Status</th>
              <th>Date</th>
              <th className="cache-th-actions"></th>
            </tr>
          </thead>
          <tbody>
            {displayRecords.length === 0 ? (
              <tr>
                <td colSpan={7} className="cache-empty">
                  No records found
                </td>
              </tr>
            ) : (
              displayRecords.map((record) => (
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
                  </td>
                  <td className="cache-td-size">{formatSize(record.size)}</td>
                  <td className="cache-td-type">{record.isHLS ? 'HLS' : 'MP4'}</td>
                  <td className="cache-td-status">{getStatusDisplay(record)}</td>
                  <td className="cache-td-date">{formatDate(record.updatedAt)}</td>
                  <td className="cache-td-actions">
                    {record.status === 'completed' && (
                      <button
                        className="btn btn-xs btn-icon btn-play-icon"
                        onClick={() => handlePlay(record)}
                        title="Play"
                      >
                        <FiPlay size={12} />
                      </button>
                    )}
                    {(record.status === 'downloading' || record.status === 'pending') ? (
                      <button
                        className="btn btn-xs btn-icon btn-cancel-icon"
                        onClick={() => handleCancelSingle(record.urlHash)}
                        title="Cancel"
                      >
                        <FiX size={12} />
                      </button>
                    ) : record.status === 'failed' ? (
                      <>
                        <button
                          className="btn btn-xs btn-icon btn-retry-icon"
                          onClick={() => handleRetrySingle(record.urlHash)}
                          title="Retry"
                        >
                          <FiRefreshCw size={12} />
                        </button>
                        <button
                          className="btn btn-xs btn-icon btn-danger-icon"
                          onClick={() => handleDeleteSingle(record.id)}
                          disabled={deleting}
                          title="Delete"
                        >
                          <FiTrash2 size={12} />
                        </button>
                      </>
                    ) : (
                      <button
                        className="btn btn-xs btn-icon btn-danger-icon"
                        onClick={() => handleDeleteSingle(record.id)}
                        disabled={deleting}
                        title="Delete"
                      >
                        <FiTrash2 size={12} />
                      </button>
                    )}
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
