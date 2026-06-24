import React, { useState, useEffect } from 'react';
import { useStore, CacheTask } from '../store';
import { FiX, FiDownload, FiCheck, FiLoader, FiInfo } from 'react-icons/fi';
import { EpisodeInfo } from '../types';

interface CacheModalProps {
  videoName: string;
  playFlags: { flag: string; beanList: EpisodeInfo[] }[];
  onClose: () => void;
}

export const CacheModal: React.FC<CacheModalProps> = ({ videoName, playFlags, onClose }) => {
  const {
    downloadVideo,
    loadPlayerContent,
    cachedVideos,
    loadCachedFiles,
    currentSource,
    cacheTasks,
    setCacheTasks,
    updateCacheTask,
    clearCacheTasks,
    isCacheDownloading,
    setIsCacheDownloading,
    cacheToast,
    setCacheToast,
  } = useStore();

  const [selectedEpisodes, setSelectedEpisodes] = useState<Set<string>>(new Set());

  useEffect(() => {
    loadCachedFiles();
  }, []);

  const isEpisodeCached = (episodeUrl: string): boolean => {
    return cachedVideos.some((v) => v.url === episodeUrl);
  };

  const isEpisodeBusy = (epKey: string): boolean => {
    const task = cacheTasks.find((t) => t.epKey === epKey);
    if (task && (task.status === 'pending' || task.status === 'resolving' || task.status === 'downloading')) {
      return true;
    }
    return false;
  };

  const toggleEpisode = (epKey: string) => {
    const url = epKey.split(/:(.*)/)[1];
    if (isEpisodeCached(url) || isEpisodeBusy(epKey)) return;
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
      f.beanList
        .filter((ep) => !isEpisodeCached(ep.url) && !isEpisodeBusy(`${f.flag}:${ep.url}`))
        .map((ep) => `${f.flag}:${ep.url}`)
    );
    const allSelected = allKeys.length > 0 && allKeys.every((k) => selectedEpisodes.has(k));
    setSelectedEpisodes(allSelected ? new Set() : new Set(allKeys));
  };

  const startDownload = async () => {
    if (selectedEpisodes.size === 0 || isCacheDownloading) return;

    setIsCacheDownloading(true);
    const selected = Array.from(selectedEpisodes);

    const initialTasks: CacheTask[] = selected.map((epKey) => {
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

    setCacheTasks(initialTasks);
    setCacheToast({ message: `Starting download of ${selected.length} episodes...`, type: 'info' });

    const source = currentSource;
    if (!source) {
      setCacheToast({ message: 'No source available', type: 'error' });
      setIsCacheDownloading(false);
      return;
    }

    for (let i = 0; i < initialTasks.length; i++) {
      const task = initialTasks[i];

      updateCacheTask(task.epKey, { status: 'resolving' });

      try {
        const result = await loadPlayerContent(source.key, task.playFlag, task.episode.url);
        if (!result || !result.url) {
          updateCacheTask(task.epKey, { status: 'failed', error: 'Failed to resolve URL' });
          continue;
        }

        const downloadId = await downloadVideo(result.url, result.headers || {}, `${videoName} - ${task.episode.name}`);

        if (!downloadId) {
          updateCacheTask(task.epKey, { status: 'failed', error: 'Failed to start download' });
          continue;
        }

        updateCacheTask(task.epKey, { status: 'downloading', downloadId });

        await new Promise<void>((resolve) => {
          const checkProgress = () => {
            const progress = useStore.getState().downloadProgress.get(downloadId);
            if (progress) {
              updateCacheTask(task.epKey, { progress: progress.progress });

              if (progress.status === 'completed') {
                updateCacheTask(task.epKey, { status: 'completed', progress: 100 });
                resolve();
              } else if (progress.status === 'failed') {
                updateCacheTask(task.epKey, { status: 'failed', error: progress.error || 'Download failed' });
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
        updateCacheTask(task.epKey, { status: 'failed', error: String(e) });
      }
    }

    setIsCacheDownloading(false);
    loadCachedFiles();

    const currentTasks = useStore.getState().cacheTasks;
    const completedCount = currentTasks.filter((t) => t.status === 'completed').length;
    const failedCount = currentTasks.filter((t) => t.status === 'failed').length;
    if (failedCount === 0) {
      setCacheToast({ message: `All ${completedCount} episodes downloaded!`, type: 'success' });
    } else {
      setCacheToast({
        message: `Downloaded ${completedCount}, failed ${failedCount}`,
        type: failedCount > 0 ? 'error' : 'success',
      });
    }
  };

  const totalProgress = cacheTasks.length > 0
    ? cacheTasks.reduce((sum, t) => sum + t.progress, 0) / cacheTasks.length
    : 0;

  const completedCount = cacheTasks.filter((t) => t.status === 'completed').length;
  const activeTask = cacheTasks.find((t) => t.status === 'resolving' || t.status === 'downloading');

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
            {playFlags.flatMap((f) => f.beanList)
              .filter((ep) => !isEpisodeCached(ep.url) && !isEpisodeBusy(`${playFlags[0]?.flag}:${ep.url}`))
              .every((ep) =>
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
                  const task = cacheTasks.find((t) => t.epKey === epKey);
                  const cached = isEpisodeCached(episode.url);

                  return (
                    <button
                      key={epIndex}
                      className={`cache-ep-btn ${isSelected ? 'selected' : ''} ${task?.status === 'completed' ? 'completed' : ''} ${task?.status === 'failed' ? 'failed' : ''} ${cached && !task ? 'cached' : ''} ${isEpisodeBusy(epKey) ? 'busy' : ''}`}
                      onClick={() => !isCacheDownloading && toggleEpisode(epKey)}
                      disabled={isCacheDownloading || cached || isEpisodeBusy(epKey)}
                    >
                      <span className="cache-ep-name">{episode.name}</span>
                      {task?.status === 'completed' && (
                        <FiCheck size={10} className="cache-ep-icon" />
                      )}
                      {task?.status === 'failed' && (
                        <FiX size={10} className="cache-ep-icon" />
                      )}
                      {task?.status === 'downloading' && (
                        <FiLoader size={10} className="cache-ep-icon spin" />
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

        {(isCacheDownloading || cacheTasks.length > 0) && (
          <div className="cache-progress-section">
            <div className="cache-progress-bar-wrapper">
              <div
                className="cache-progress-bar"
                style={{ width: `${totalProgress}%` }}
              />
            </div>
            <div className="cache-progress-info">
              {isCacheDownloading && activeTask ? (
                <span>
                  {activeTask.status === 'resolving'
                    ? `Resolving: ${activeTask.episode.name}...`
                    : `Downloading: ${activeTask.episode.name} (${Math.round(activeTask.progress)}%)`}
                </span>
              ) : cacheTasks.length > 0 ? (
                <span>
                  {completedCount}/{cacheTasks.length} completed
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
            {cacheTasks.length > 0 && !isCacheDownloading && (
              <button className="btn btn-sm btn-secondary" onClick={clearCacheTasks}>
                Clear
              </button>
            )}
            <button className="btn btn-secondary" onClick={onClose}>
              {isCacheDownloading ? 'Hide' : 'Cancel'}
            </button>
            <button
              className="btn btn-primary"
              onClick={startDownload}
              disabled={selectedEpisodes.size === 0 || isCacheDownloading}
            >
              {isCacheDownloading ? (
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

        {cacheToast && (
          <div className={`cache-toast cache-toast-${cacheToast.type}`}>
            {cacheToast.type === 'success' && <FiCheck size={14} />}
            {cacheToast.type === 'error' && <FiX size={14} />}
            {cacheToast.type === 'info' && <FiInfo size={14} />}
            <span>{cacheToast.message}</span>
          </div>
        )}
      </div>
    </div>
  );
};
