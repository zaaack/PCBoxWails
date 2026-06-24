import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useStore } from '../store';
import { api } from '../lib/api';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import 'videojs-hotkeys';
import Player from 'video.js/dist/types/player';
import { FiArrowLeft, FiList, FiMaximize, FiMonitor, FiMapPin } from 'react-icons/fi';
import { MdOutlinePlayDisabled } from 'react-icons/md';

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
  } = useStore();

  const videoContainerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<Player | null>(null);
  const [playError, setPlayError] = useState('');
  const [isSystemFullscreen, setIsSystemFullscreen] = useState(false);
  const [isOnTop, setIsOnTop] = useState(false);
  const [showEpisodePanel, setShowEpisodePanel] = useState(false);
  const [showOverlay, setShowOverlay] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const hideTimerRef = useRef<ReturnType<typeof setTimeout>>();
  const progressSaveRef = useRef<ReturnType<typeof setInterval>>();

  const resetHideTimer = useCallback(() => {
    setShowOverlay(true);
    if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    if (!isPaused) {
      hideTimerRef.current = setTimeout(() => setShowOverlay(false), 3000);
    }
  }, [isPaused]);

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
        pictureInPictureToggle: true,
        fullscreenToggle: false,
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
    });

    player.on('pause', () => {
      setIsPaused(true);
      setShowOverlay(true);
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    });

    player.on('error', () => {
      const error = player.error();
      if (error) {
        console.error('[PCBox] Video error:', error.message);
        setPlayError(`Playback error: ${error.message}`);
      }
    });

    return () => {
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
    resetHideTimer();
    return () => {
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    };
  }, [isPaused, resetHideTimer]);

  useEffect(() => {
    progressSaveRef.current = setInterval(() => {
        console.log('save history', playerRef.current, currentVideo , currentEpisode, )
      if (playerRef.current && currentVideo && currentEpisode) {
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
          progress: Math.floor((playerRef.current.currentTime() || 0) * 1000),
          duration: Math.floor((playerRef.current.duration() || 0) * 1000),
        });
      }
    }, 10000);

    return () => {
      if (progressSaveRef.current) {
        clearInterval(progressSaveRef.current);
      }
    };
  }, [currentVideo, currentEpisode, currentEpisodeIndex, currentPlayFlag]);

  const playNextEpisode = () => {
    if (!currentVideo || !currentPlayFlag) return;

    const playFlags = currentVideo.urlBean?.infoList || [];
    const currentFlag = playFlags.find((f) => f.flag === currentPlayFlag);
    if (!currentFlag) return;

    const nextIndex = currentEpisodeIndex + 1;
    if (nextIndex < currentFlag.beanList.length) {
      const nextEpisode = currentFlag.beanList[nextIndex];
      handlePlayEpisode(nextEpisode, nextIndex, currentPlayFlag);
    }
  };

  const handlePlayEpisode = async (episode: any, episodeIndex: number, playFlag: string) => {
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
        finalUrl = 'file://' + cachedFile.replace(/\\/g, '/');
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

  const toggleWindowFullscreen = async () => {
    if (!isSystemFullscreen) {
      const result = await api.toggleFullscreen(true);
      setIsSystemFullscreen(result);
    } else {
      await api.toggleFullscreen(false);
      setIsSystemFullscreen(false);
    }
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
        } else if (!e.altKey) {
          toggleWindowFullscreen();
        }
      } else if (e.key === 'F11') {
        e.preventDefault();
        toggleWindowFullscreen();
      } else if (e.key === 'e' || e.key === 'E') {
        setShowEpisodePanel((p) => !p);
      } else if (e.key === 't' || e.key === 'T') {
        if (!e.ctrlKey && !e.altKey) {
          toggleAlwaysOnTop();
        }
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [showEpisodePanel, isSystemFullscreen]);

  const handleMouseMove = () => {
    resetHideTimer();
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
      className={`player-page ${isFs ? 'is-fullscreen' : ''} ${showOverlay ? '' : 'hide-cursor'}`}
      onMouseMove={handleMouseMove}
    >
      <div className="video-container">
        <div ref={videoContainerRef} className="video-js-wrapper" />
      </div>

      <div className={`player-overlay ${showOverlay || showEpisodePanel ? 'visible' : ''}`}>
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
            onClick={toggleWindowFullscreen}
            title="Window Fullscreen (F)"
          >
            <FiMaximize size={18} />
          </button>
          <button
            className="overlay-btn"
            onClick={toggleSystemFullscreen}
            title="System Fullscreen (Ctrl+F)"
          >
            <FiMonitor size={18} />
          </button>
        </div>
      </div>

      {showEpisodePanel && (
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
    </div>
  );
};
