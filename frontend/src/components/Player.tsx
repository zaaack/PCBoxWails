import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useStore } from '../store';
import { api } from '../lib/api';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import 'videojs-hotkeys';
import Player from 'video.js/dist/types/player';
import { FiArrowLeft, FiList, FiMaximize, FiMinus, FiMapPin, FiSkipBack, FiSkipForward, FiCamera, FiX } from 'react-icons/fi';
import { MdOutlinePlayDisabled } from 'react-icons/md';

const formatSystemTime = (date: Date) => {
  const h = date.getHours();
  const m = date.getMinutes();
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`;
};

export const PlayerView: React.FC = () => {
  const {
    playUrl,
    playHeaders,
    currentEpisode,
    currentVideo,
    currentEpisodeIndex,
    currentPlayFlag,
    previousView,
    setPlayState,
    setViewMode,
    saveHistory,
    historyHighlightEpisode,
    getCachedFile,
    showPlayerTime,
  } = useStore();

  const videoContainerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<Player | null>(null);
  const [playError, setPlayError] = useState('');
  const [isSystemFullscreen, setIsSystemFullscreen] = useState(false);
  const [isOnTop, setIsOnTop] = useState(false);
  const [showEpisodePanel, setShowEpisodePanel] = useState(false);
  const [showOverlay, setShowOverlay] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const [progressConflict, setProgressConflict] = useState<{ tvk: number; local: number } | null>(null);
  const [isScreenCaptureMode, setIsScreenCaptureMode] = useState(false);
  const [systemTime, setSystemTime] = useState(() => formatSystemTime(new Date()));
  const systemTimeRef = useRef<ReturnType<typeof setInterval>>();
  const hideTimerRef = useRef<ReturnType<typeof setTimeout>>();
  const progressSaveRef = useRef<ReturnType<typeof setInterval>>();
  const cacheProgressRef = useRef<ReturnType<typeof setInterval>>();
  const isPausedRef = useRef(isPaused);
  isPausedRef.current = isPaused;

  useEffect(() => {
    systemTimeRef.current = setInterval(() => {
      setSystemTime(formatSystemTime(new Date()));
    }, 60000);
    return () => {
      if (systemTimeRef.current) clearInterval(systemTimeRef.current);
    };
  }, []);

  const resetHideTimer = useCallback(() => {
    setShowOverlay(true);
    if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    if (!isPausedRef.current) {
      hideTimerRef.current = setTimeout(() => setShowOverlay(false), 3000);
    }
  }, []);

  useEffect(() => {
    if (!videoContainerRef.current || !playUrl) return;

    if (playerRef.current) {
      playerRef.current.dispose();
      playerRef.current = null;
    }

    const videoElement = document.createElement('video-js');
    videoElement.classList.add('vjs-big-play-centered');
    videoContainerRef.current.appendChild(videoElement);

    const isHls = playUrl.includes('.m3u8') || playUrl.includes('m3u8');

    const player = videojs(videoElement, {
      controls: true,
      autoplay: true,
      preload: 'auto',
      fluid: false,
      responsive: false,
      playbackRates: [0.5, 0.75, 1, 1.25, 1.5, 2],
      controlBar: {
        volumePanel: { inline: false },
        pictureInPictureToggle: false,
        fullscreenToggle: true,
      },
      sources: [{ src: playUrl, type: isHls ? 'application/x-mpegURL' : 'video/mp4' }],
      html5: {
        hls: {
          enableWorker: true,
          lowLatencyMode: true,
          xhrSetup: (xhr: XMLHttpRequest) => {
            for (const [key, value] of Object.entries(playHeaders)) {
              xhr.setRequestHeader(key, value);
            }
          },
        },
        vhs: {
          enableLowInitialPlaylist: true,
          // 限制前向缓冲的最大秒数，默认值可能比较小，你可以改大（例如 5 分钟 = 300 秒）
          maxBufferLength: 300*10, 
          // 只要低于这个秒数，就会一直疯狂下载切片
          goalBufferLength: 300,
        },
        nativeAudioTracks: true,
        nativeVideoTracks: true,
      },
    });

    playerRef.current = player;

    const savedVolume = localStorage.getItem('player-volume');
    if (savedVolume !== null) {
      player.volume(parseFloat(savedVolume));
    }

    player.on('volumechange', () => {
      localStorage.setItem('player-volume', String(player.volume()));
    });

    const fsButton = (player as any).controlBar.fullscreenToggle;
    if (fsButton) {
      fsButton.off('click');
      fsButton.on('click', () => {
        toggleSystemFullscreen();
      });
    }

    if (currentPlayFlag === 'cache' && currentEpisode) {
      const tvkProgress =
        historyHighlightEpisode?.progress &&
        historyHighlightEpisode.episodeUrl === currentEpisode.url
          ? historyHighlightEpisode.progress
          : 0;

      const filePath = currentEpisode.url;
      (async () => {
        let localProgress = 0;
        try {
          const saved = await api.getCacheProgress(filePath);
          if (saved) {
            localProgress = saved.progress || 0;
          }
        } catch {}

        const hasBoth = tvkProgress > 0 && localProgress > 0 && Math.abs(tvkProgress - localProgress) > 5000;

        if (hasBoth) {
          setProgressConflict({ tvk: tvkProgress, local: localProgress });
        } else {
          const seekTo = (tvkProgress || localProgress) / 1000;
          if (seekTo > 0) {
            player.on('loadedmetadata', () => {
              player.currentTime(seekTo);
              console.log('[PCBox] Restored cache progress:', seekTo, 's');
            });
          }
        }
      })();
    }

    (player as any).hotkeys({
      volumeStep: 0.1,
      seekStep: 5,
      enableVolumeScroll: true,
      enableModifiersForNumbers: false,
      alwaysCaptureHotkeys: true,
    });

    player.on('ended', () => {
      playNextEpisode();
    });

    player.on('play', () => {
      setIsPaused(false);
      resetHideTimer();
      api.setKeepScreenOn(true);
    });

    player.on('pause', () => {
      setIsPaused(true);
      setShowOverlay(true);
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
      api.setKeepScreenOn(false);
    });

    player.on('error', () => {
      const error = player.error();
      if (error) {
        console.error('[PCBox] Video error:', error.message);
        setPlayError(`Playback error: ${error.message}`);
      }
    });

    return () => {
      api.setKeepScreenOn(false);
      if (playerRef.current) {
        playerRef.current.dispose();
        playerRef.current = null;
      }
    };
  }, [playUrl]);

  useEffect(() => {
    if (playerRef.current && historyHighlightEpisode?.progress) {
      const seekTo = historyHighlightEpisode.progress / 1000;
      playerRef.current.ready(() => {
        playerRef.current?.currentTime(seekTo);
      });
    }
  }, [playUrl, historyHighlightEpisode]);

  useEffect(() => {
    if (isPaused) {
      setShowOverlay(true);
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    } else {
      resetHideTimer();
    }
    return () => {
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    };
  }, [isPaused]);

  useEffect(() => {
    progressSaveRef.current = setInterval(() => {
      if (!playerRef.current || !currentEpisode) return;

      const progress = Math.floor((playerRef.current.currentTime() || 0) * 1000);
      const duration = Math.floor((playerRef.current.duration() || 0) * 1000);

      // Save to Go backend (works for both cache and non-cache modes)
      api.savePlayHistory({
        sourceKey: currentVideo?.sourceKey,
        vodId: currentVideo?.id,
        vodName: currentVideo?.name || currentEpisode.name,
        vodPic: currentVideo?.pic,
        playFlag: currentPlayFlag,
        episodeFlag: currentEpisode.name,
        episodeUrl: currentEpisode.url,
        episodeIndex: currentEpisodeIndex,
        reverseSort: false,
        progress,
        duration,
        updatedAt: Date.now(),
      });

      // Save to mobile client (only when currentVideo is set)
      if (currentVideo) {
        saveHistory({
          id: currentVideo.id,
          sourceKey: currentVideo.sourceKey,
          name: currentVideo.name,
          pic: currentVideo.pic,
          playFlag: currentPlayFlag,
          episodeFlag: currentEpisode.name,
          episodeUrl: currentEpisode.url,
          episodeIndex: currentEpisodeIndex,
          reverseSort: false,
          progress,
          duration,
        });
      }
    }, 10000);

    return () => {
      if (progressSaveRef.current) {
        clearInterval(progressSaveRef.current);
      }
    };
  }, [currentVideo, currentEpisode, currentEpisodeIndex, currentPlayFlag]);

  useEffect(() => {
    if (cacheProgressRef.current) {
      clearInterval(cacheProgressRef.current);
    }

    if (currentPlayFlag !== 'cache' || !currentEpisode) return;

    const filePath = currentEpisode.url;

    cacheProgressRef.current = setInterval(() => {
      if (playerRef.current) {
        const currentTime = Math.floor((playerRef.current.currentTime() || 0) * 1000);
        const duration = Math.floor((playerRef.current.duration() || 0) * 1000);
        api.saveCacheProgress(filePath, currentTime, duration);
      }
    }, 5000);

    return () => {
      if (cacheProgressRef.current) {
        clearInterval(cacheProgressRef.current);
      }
    };
  }, [currentPlayFlag, currentEpisode]);

  const playNextEpisode = async () => {
    if (!currentPlayFlag) return;

    // Cache-only mode (from CacheManager, no currentVideo)
    if (currentPlayFlag === 'cache' && !currentVideo) {
      if (!currentEpisode) return;
      const record = await api.findDownloadRecordByFilePath(currentEpisode.url);
      if (!record || !record.sourceKey || !record.playFlag) return;

      const nextIndex = currentEpisodeIndex + 1;
      const nextRecord = await api.findNextCachedEpisode(record.sourceKey, record.playFlag, nextIndex);
      if (nextRecord && nextRecord.filePath) {
        const port = await api.getProxyPort();
        const fileUrl = `http://127.0.0.1:${port}/local?u=${encodeURIComponent(nextRecord.filePath)}`;
        handlePlayEpisode(
          { name: nextRecord.videoName, url: nextRecord.filePath },
          nextIndex,
          'cache',
          fileUrl
        );
        return;
      }
      return;
    }

    // Normal flow with currentVideo (detail page)
    if (!currentVideo) return;

    const playFlags = currentVideo.urlBean?.infoList || [];
    const currentFlag = playFlags.find((f) => f.flag === currentPlayFlag);
    if (!currentFlag) return;

    const nextIndex = currentEpisodeIndex + 1;
    if (nextIndex < currentFlag.beanList.length) {
      const nextEpisode = currentFlag.beanList[nextIndex];
      handlePlayEpisode(nextEpisode, nextIndex, currentPlayFlag);
    }
  };

  const playPreviousEpisode = async () => {
    if (!currentPlayFlag) return;

    // Cache-only mode
    if (currentPlayFlag === 'cache' && !currentVideo) {
      if (!currentEpisode) return;
      const record = await api.findDownloadRecordByFilePath(currentEpisode.url);
      if (!record || !record.sourceKey || !record.playFlag) return;

      const prevIndex = currentEpisodeIndex - 1;
      if (prevIndex < 0) return;
      const prevRecord = await api.findNextCachedEpisode(record.sourceKey, record.playFlag, prevIndex);
      if (prevRecord && prevRecord.filePath) {
        const port = await api.getProxyPort();
        const fileUrl = `http://127.0.0.1:${port}/local?u=${encodeURIComponent(prevRecord.filePath)}`;
        handlePlayEpisode(
          { name: prevRecord.videoName, url: prevRecord.filePath },
          prevIndex,
          'cache',
          fileUrl
        );
        return;
      }
      return;
    }

    // Normal flow
    if (!currentVideo) return;

    const playFlags = currentVideo.urlBean?.infoList || [];
    const currentFlag = playFlags.find((f) => f.flag === currentPlayFlag);
    if (!currentFlag) return;

    const prevIndex = currentEpisodeIndex - 1;
    if (prevIndex >= 0) {
      const prevEpisode = currentFlag.beanList[prevIndex];
      handlePlayEpisode(prevEpisode, prevIndex, currentPlayFlag);
    }
  };

  const handlePlayEpisode = async (episode: any, episodeIndex: number, playFlag: string, preResolvedUrl?: string) => {
    // If we already have a pre-resolved URL (e.g., from cache fallback), play directly
    if (preResolvedUrl) {
      setPlayState(episode, episodeIndex, playFlag, preResolvedUrl, {});
      setPlayError('');
      return;
    }

    if (!currentVideo || !currentVideo.sourceKey) return;

    setPlayState(episode, episodeIndex, playFlag, '');
    setPlayError('');

    const { loadPlayerContent } = useStore.getState();
    const result = await loadPlayerContent(currentVideo.sourceKey, playFlag, episode.url);
    if (result) {
      let finalUrl = result.url;
      let finalHeaders = result.headers;

      // Check if there's a cached version of this video
      const cachedFile = await getCachedFile(result.url);
      if (cachedFile) {
        console.log('[PCBox] Using cached file:', cachedFile);
        const port = await api.getProxyPort();
        finalUrl = `http://127.0.0.1:${port}/local?u=${encodeURIComponent(cachedFile)}`;
        finalHeaders = {};
      }

      setPlayState(episode, episodeIndex, playFlag, finalUrl, finalHeaders);
    } else {
      setPlayError('This source requires a parsing service that is not available. Please try a different source.');
    }
  };

  const handleBack = async () => {
    await exitAllFullscreen();
    if (isOnTop) {
      setIsOnTop(false);
      api.setAlwaysOnTop(false);
    }
    if (currentPlayFlag === 'cache') {
      useStore.getState().setShowCacheManager(true);
      setViewMode('home');
    } else if (currentVideo) {
      setViewMode('detail');
    } else {
      setViewMode('home');
    }
  };

  const toggleSystemFullscreen = async () => {
    if (!isSystemFullscreen) {
      await api.setFrame(false);
      await api.setAlwaysOnTop(true);
      const result = await api.toggleFullscreen(true);
      setIsSystemFullscreen(result);
    } else {
      await api.toggleFullscreen(false);
      await api.setAlwaysOnTop(false);
      await api.setFrame(true);
      setIsSystemFullscreen(false);
    }
  };

  const handleMinimize = async () => {
    await api.minimizeWindow();
  };

  const toggleAlwaysOnTop = async () => {
    const next = !isOnTop;
    setIsOnTop(next);
    api.setAlwaysOnTop(next);
  };

  const exitAllFullscreen = async () => {
    if (isSystemFullscreen) {
      await api.toggleFullscreen(false);
      await api.setAlwaysOnTop(false);
      await api.setFrame(true);
      setIsSystemFullscreen(false);
    }
  };



  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (showEpisodePanel) {
          setShowEpisodePanel(false);
        } else if (isSystemFullscreen) {
          toggleSystemFullscreen();
        }
      } else if (e.key === 'f' || e.key === 'F') {
        if (e.ctrlKey) {
          e.preventDefault();
          toggleSystemFullscreen();
        }
      } else if (e.key === 'F11') {
        e.preventDefault();
        toggleSystemFullscreen();
      } else if (e.key === 'e' || e.key === 'E') {
        setShowEpisodePanel((p) => !p);
      } else if (e.key === 't' || e.key === 'T') {
        if (!e.ctrlKey && !e.altKey) {
          toggleAlwaysOnTop();
        }
      } else if (e.key === 'c' || e.key === 'C') {
        if (!e.ctrlKey && !e.altKey) {
          setIsScreenCaptureMode((p) => !p);
        }
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [showEpisodePanel, isSystemFullscreen, isScreenCaptureMode]);

  useEffect(() => {
    const container = videoContainerRef.current;
    if (!container) return;

    const handleMouse = () => {
      resetHideTimer();
    };

    container.addEventListener('mousemove', handleMouse);
    return () => container.removeEventListener('mousemove', handleMouse);
  }, [playUrl]);

  const handleMouseMove = () => {
    resetHideTimer();
  };

  const formatMs = (ms: number) => {
    const s = Math.floor(ms / 1000);
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    return h > 0
      ? `${h}:${m.toString().padStart(2, '0')}:${sec.toString().padStart(2, '0')}`
      : `${m}:${sec.toString().padStart(2, '0')}`;
  };

  const handleConflictChoice = (source: 'tvk' | 'local') => {
    if (!progressConflict || !playerRef.current) return;
    const seekTo = (source === 'tvk' ? progressConflict.tvk : progressConflict.local) / 1000;
    playerRef.current.ready(() => {
      playerRef.current?.currentTime(seekTo);
    });
    setProgressConflict(null);
  };

  const isFs = isSystemFullscreen;

  if (!playUrl) {
    return (
      <div className="player-page">
        <div className="player-loading">
          {playError ? (
            <>
              <MdOutlinePlayDisabled size={48} color="var(--danger)" />
              <p style={{ color: 'var(--danger)' }}>{playError}</p>
              <button className="btn btn-secondary" onClick={handleBack}>
                Back
              </button>
            </>
          ) : (
            <>
              <div className="spinner"></div>
              <p>Loading video...</p>
              <button className="btn btn-secondary" onClick={handleBack}>
                Back
              </button>
            </>
          )}
        </div>
      </div>
    );
  }

  return (
    <div
      className={`player-page ${isFs ? 'is-fullscreen' : ''} ${showOverlay || isScreenCaptureMode ? '' : 'hide-cursor'} ${isScreenCaptureMode ? 'screen-capture-mode' : ''}`}
      onMouseMove={handleMouseMove}
    >
      <div className="video-container">
        <div ref={videoContainerRef} className="video-js-wrapper" />
      </div>

      {isScreenCaptureMode && (
        <div className="screen-capture-exit-bar">
          <button className="screen-capture-exit-btn" onClick={() => setIsScreenCaptureMode(false)}>
            <FiX size={16} />
            <span>Exit Screenshot Mode</span>
          </button>
        </div>
      )}

      {showPlayerTime && !isScreenCaptureMode && (
        <div className="player-system-time">
          {systemTime}
        </div>
      )}

      {!isScreenCaptureMode && (
      <div
        className={`player-overlay ${showOverlay || showEpisodePanel ? 'visible' : ''}`}
        onMouseMove={resetHideTimer}
        onClick={resetHideTimer}
      >
        <div className="overlay-top">
          <button className="overlay-btn" onClick={handleBack} title="Back">
            <FiArrowLeft size={18} />
          </button>
          <span className="overlay-title">
            {currentVideo?.name} - {currentEpisode?.name || 'Loading...'}
          </span>
        </div>

        <div className="overlay-top-right">
          <button
            className="overlay-btn"
            onClick={playPreviousEpisode}
            title="Previous Episode"
            disabled={currentEpisodeIndex <= 0 && (!currentVideo || !currentPlayFlag)}
          >
            <FiSkipBack size={18} />
          </button>
          <button
            className="overlay-btn"
            onClick={playNextEpisode}
            title="Next Episode"
            disabled={!currentPlayFlag}
          >
            <FiSkipForward size={18} />
          </button>
          <button
            className="overlay-btn"
            onClick={() => setIsScreenCaptureMode(true)}
            title="Screenshot Mode (C)"
          >
            <FiCamera size={18} />
          </button>
          <button
            className="overlay-btn"
            onClick={() => setShowEpisodePanel(!showEpisodePanel)}
            title="Episodes (E)"
          >
            <FiList size={18} />
          </button>
          <button
            className={`overlay-btn ${isOnTop ? 'active' : ''}`}
            onClick={toggleAlwaysOnTop}
            title="Always on Top (T)"
          >
            <FiMapPin size={18} />
          </button>
          <button
            className="overlay-btn"
            onClick={handleMinimize}
            title="Minimize"
          >
            <FiMinus size={18} />
          </button>
          <button
            className="overlay-btn"
            onClick={toggleSystemFullscreen}
            title="Fullscreen (F11 / Ctrl+F)"
          >
            <FiMaximize size={18} />
          </button>
        </div>
      </div>
      )}

      {!isScreenCaptureMode && showEpisodePanel && (
        <div className="episode-overlay" onClick={() => setShowEpisodePanel(false)}>
          <div className="episode-panel" onClick={(e) => e.stopPropagation()}>
            <div className="episode-panel-header">
              <h3>Episodes</h3>
              <button className="btn-close" onClick={() => setShowEpisodePanel(false)}>
                ×
              </button>
            </div>
            <div className="episode-panel-list">
              {currentVideo?.urlBean?.infoList
                .find((f) => f.flag === currentPlayFlag)
                ?.beanList.map((ep, index) => (
                  <button
                    key={index}
                    className={`episode-panel-item ${
                      index === currentEpisodeIndex ? 'active' : ''
                    }`}
                    onClick={() => {
                      handlePlayEpisode(ep, index, currentPlayFlag);
                      setShowEpisodePanel(false);
                    }}
                    title={ep.name}
                  >
                    <span className="episode-panel-name">{index + 1}</span>
                  </button>
                ))}
            </div>
          </div>
        </div>
      )}

      {progressConflict && (
        <div className="episode-overlay" onClick={() => {}}>
          <div className="episode-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 380 }}>
            <div className="episode-panel-header">
              <h3>Playback Progress</h3>
            </div>
            <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: 12 }}>
              <p style={{ margin: 0, fontSize: 14, opacity: 0.8 }}>This video has progress from two sources, choose one:</p>
              <button
                className="btn btn-secondary"
                style={{ textAlign: 'left', padding: '12px 16px' }}
                onClick={() => handleConflictChoice('tvk')}
              >
                TV-K History — {formatMs(progressConflict.tvk)}
              </button>
              <button
                className="btn btn-secondary"
                style={{ textAlign: 'left', padding: '12px 16px' }}
                onClick={() => handleConflictChoice('local')}
              >
                Local Cache — {formatMs(progressConflict.local)}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
