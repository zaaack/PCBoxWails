import React, { useState, useEffect, useCallback } from 'react';
import { useStore } from '../store';
import { FiX, FiDownload, FiCheck, FiLoader, FiInfo } from 'react-icons/fi';
import { EpisodeInfo } from '../types';

interface CacheModalProps {
  videoName: string;
  playFlags: { flag: string; beanList: EpisodeInfo[] }[];
  onClose: () => void;
}

interface DownloadTask {
  epKey: string;
  episode: EpisodeInfo;
  playFlag: string;
  status: 'pending' | 'resolving' | 'downloading' | 'completed' | 'failed';
  progress: number;
  error?: string;
}

export const CacheModal: React.FC<CacheModalProps> = ({ videoName, playFlags, onClose }) => {
  const {
    downloadVideo,
    loadPlayerContent,
    downloadProgress,
    cachedVideos,
    loadCachedFiles,
    currentSource,
  } = useStore();

  const [selectedEpisodes, setSelectedEpisodes] = useState<Set<string>>(new Set());
  const [tasks, setTasks] = useState<DownloadTask[]>([]);
  const [isDownloading, setIsDownloading] = useState(false);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' | 'info' } | null>(null);

  useEffect(() => {
    loadCachedFiles();
  }, []);

  const showToast = useCallback((message: string, type: 'success' | 'error' | 'info' = 'info') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const isEpisodeCached = (episodeUrl: string, playFlag: string): boolean => {
    return cachedVideos.some((v) => v.url && v.videoName.includes(episodeUrl));
  };

  const toggleEpisode = (epKey: string) => {
    setSelectedEpisodes((prev) => {
      const next = new Set(prev);
      if (next.has(epKey)) {
        next.delete(epKey);
      } else {
        next.add(epKey);
      }
      return next;
    });
  };

  const toggleAll = () => {
    const allKeys = playFlags.flatMap((f) =>
      f.beanList.map((ep) => `${f.flag}:${ep.url}`)
    );
    const allSelected = allKeys.every((k) => selectedEpisodes.has(k));
    setSelectedEpisodes(allSelected ? new Set() : new Set(allKeys));
  };

  const startDownload = async () => {
    if (selectedEpisodes.size === 0 || isDownloading) return;

    setIsDownloading(true);
    const selected = Array.from(selectedEpisodes);

    const initialTasks: DownloadTask[] = selected.map((epKey) => {
      const [flag, url] = epKey.split(/:(.*)/);
      const ep = playFlags
        .flatMap((f) => f.beanList)
        .find((e) => e.url === url);
      return {
        epKey,
        episode: ep || { name: url, url },
        playFlag: flag,
        status: 'pending',
        progress: 0,
      };
    });

    setTasks(initialTasks);
    showToast(`Starting download of ${selected.length} episodes...`, 'info');

    const source = currentSource;
    if (!source) {
      showToast('No source available', 'error');
      setIsDownloading(false);
      return;
    }

    for (let i = 0; i < initialTasks.length; i++) {
      const task = initialTasks[i];
      const taskIndex = i;

      setTasks((prev) =>
        prev.map((t, idx) =>
          idx === taskIndex ? { ...t, status: 'resolving' } : t
        )
      );

      try {
        const result = await loadPlayerContent(source.key, task.playFlag, task.episode.url);
        if (!result || !result.url) {
          setTasks((prev) =>
            prev.map((t, idx) =>
              idx === taskIndex ? { ...t, status: 'failed', error: 'Failed to resolve URL' } : t
            )
          );
          continue;
        }

        const downloadId = await downloadVideo(result.url, result.headers || {}, `${videoName} - ${task.episode.name}`);

        if (!downloadId) {
          setTasks((prev) =>
            prev.map((t, idx) =>
              idx === taskIndex ? { ...t, status: 'failed', error: 'Failed to start download' } : t
            )
          );
          continue;
        }

        setTasks((prev) =>
          prev.map((t, idx) =>
            idx === taskIndex ? { ...t, status: 'downloading' } : t
          )
        );

        await new Promise<void>((resolve) => {
          const checkProgress = () => {
            const progress = useStore.getState().downloadProgress.get(downloadId);
            if (progress) {
              setTasks((prev) =>
                prev.map((t, idx) =>
                  idx === taskIndex ? { ...t, progress: progress.progress } : t
                )
              );

              if (progress.status === 'completed') {
                setTasks((prev) =>
                  prev.map((t, idx) =>
                    idx === taskIndex ? { ...t, status: 'completed', progress: 100 } : t
                  )
                );
                resolve();
              } else if (progress.status === 'failed') {
                setTasks((prev) =>
                  prev.map((t, idx) =>
                    idx === taskIndex ? { ...t, status: 'failed', error: progress.error || 'Download failed' } : t
                  )
                );
                resolve();
              } else {
                setTimeout(checkProgress, 300);
              }
            } else {
              setTimeout(checkProgress, 300);
            }
          };
          checkProgress();
        });
      } catch (e) {
        setTasks((prev) =>
          prev.map((t, idx) =>
            idx === taskIndex ? { ...t, status: 'failed', error: String(e) } : t
          )
        );
      }
    }

    const finalTasks = useStore.getState().downloadProgress;
    setIsDownloading(false);
    loadCachedFiles();

    const completedCount = tasks.filter((t) => t.status === 'completed').length;
    const failedCount = tasks.filter((t) => t.status === 'failed').length;
    if (failedCount === 0) {
      showToast(`All ${completedCount} episodes downloaded successfully!`, 'success');
    } else {
      showToast(`Downloaded ${completedCount}, failed ${failedCount}`, failedCount > 0 ? 'error' : 'success');
    }
  };

  const totalProgress = tasks.length > 0
    ? tasks.reduce((sum, t) => sum + t.progress, 0) / tasks.length
    : 0;

  const completedCount = tasks.filter((t) => t.status === 'completed').length;
  const activeTask = tasks.find((t) => t.status === 'resolving' || t.status === 'downloading');

  return (
    <div className="cache-overlay" onClick={onClose}>
      <div className="cache-panel" onClick={(e) => e.stopPropagation()}>
        <div className="cache-panel-header">
          <h3>Cache Videos</h3>
          <button className="btn-close" onClick={onClose}>
            <FiX size={18} />
          </button>
        </div>

        <div className="cache-panel-subheader">
          <span className="cache-video-name">{videoName}</span>
          <button className="btn btn-sm btn-secondary" onClick={toggleAll}>
            {playFlags.flatMap((f) => f.beanList).every((ep) =>
              selectedEpisodes.has(`${playFlags[0]?.flag}:${ep.url}`)
            )
              ? 'Deselect All'
              : 'Select All'}
          </button>
        </div>

        <div className="cache-episode-grid">
          {playFlags.map((flagInfo) => (
            <div key={flagInfo.flag} className="cache-flag-group">
              {playFlags.length > 1 && (
                <div className="cache-flag-label">{flagInfo.flag}</div>
              )}
              <div className="cache-episodes">
                {flagInfo.beanList.map((episode, epIndex) => {
                  const epKey = `${flagInfo.flag}:${episode.url}`;
                  const isSelected = selectedEpisodes.has(epKey);
                  const task = tasks.find((t) => t.epKey === epKey);
                  const cached = isEpisodeCached(episode.url, flagInfo.flag);

                  return (
                    <button
                      key={epIndex}
                      className={`cache-ep-btn ${isSelected ? 'selected' : ''} ${task?.status === 'completed' ? 'completed' : ''} ${task?.status === 'failed' ? 'failed' : ''} ${cached ? 'cached' : ''}`}
                      onClick={() => !isDownloading && toggleEpisode(epKey)}
                      disabled={isDownloading}
                    >
                      <span className="cache-ep-name">{episode.name}</span>
                      {task?.status === 'completed' && (
                        <FiCheck size={10} className="cache-ep-icon" />
                      )}
                      {task?.status === 'failed' && (
                        <FiX size={10} className="cache-ep-icon" />
                      )}
                      {cached && !task && (
                        <FiCheck size={10} className="cache-ep-icon cached-icon" />
                      )}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>

        {(isDownloading || tasks.length > 0) && (
          <div className="cache-progress-section">
            <div className="cache-progress-bar-wrapper">
              <div
                className="cache-progress-bar"
                style={{ width: `${totalProgress}%` }}
              />
            </div>
            <div className="cache-progress-info">
              {isDownloading && activeTask ? (
                <span>
                  {activeTask.status === 'resolving'
                    ? `Resolving: ${activeTask.episode.name}...`
                    : `Downloading: ${activeTask.episode.name} (${Math.round(activeTask.progress)}%)`}
                </span>
              ) : tasks.length > 0 ? (
                <span>
                  {completedCount}/{tasks.length} completed
                </span>
              ) : null}
            </div>
          </div>
        )}

        <div className="cache-panel-footer">
          <span className="cache-selected-count">
            {selectedEpisodes.size} episode{selectedEpisodes.size !== 1 ? 's' : ''} selected
          </span>
          <div className="cache-footer-actions">
            <button className="btn btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button
              className="btn btn-primary"
              onClick={startDownload}
              disabled={selectedEpisodes.size === 0 || isDownloading}
            >
              {isDownloading ? (
                <>
                  <FiLoader className="spin" size={14} /> Downloading...
                </>
              ) : (
                <>
                  <FiDownload size={14} /> Download
                </>
              )}
            </button>
          </div>
        </div>

        {toast && (
          <div className={`cache-toast cache-toast-${toast.type}`}>
            {toast.type === 'success' && <FiCheck size={14} />}
            {toast.type === 'error' && <FiX size={14} />}
            {toast.type === 'info' && <FiInfo size={14} />}
            <span>{toast.message}</span>
          </div>
        )}
      </div>
    </div>
  );
};
