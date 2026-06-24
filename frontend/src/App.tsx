import React, { useEffect, useState } from 'react';
import { useStore } from './store';
import { api } from './lib/api';
import { Sidebar } from './components/Sidebar';
import { Home } from './components/Home';
import { Search } from './components/Search';
import { VideoDetail } from './components/VideoDetail';
import { PlayerView } from './components/Player';
import { History } from './components/History';
import { Settings } from './components/Settings';
import { CacheManager } from './components/CacheManager';
import { GlobalToast } from './components/Toast';

const App: React.FC = () => {
  const {
    viewMode,
    setWsStatus,
    setLocalIp,
    setConnectedClient,
    setSources,
    setCurrentSource,
    topicCallbacks,
    removeTopicCallback,
    theme,
    setTheme,
    setDownloadProgress,
    loadCacheDir,
    addToast,
    downloadQueue,
    loadDownloadQueue,
    showCacheManager,
    setShowCacheManager,
  } = useStore();

  const [showSettings, setShowSettings] = useState(false);

  useEffect(() => {
    initApp();
    setupListeners();
    applyTheme();
    loadCacheDir();
    checkPendingDownloads();
  }, []);

  const applyTheme = () => {
    const savedTheme = useStore.getState().theme;
    const resolved = savedTheme === 'system'
      ? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : savedTheme;
    document.documentElement.setAttribute('data-theme', resolved);
  };

  const initApp = async () => {
    const ip = await api.getLocalIp();
    if (ip) setLocalIp(ip);

    try {
      console.log('[PCBox] Querying server status...');
      const status = await api.getWsServerStatus();
      console.log('[PCBox] Server status:', status);
      if (status && status.running) {
        setWsStatus(true, status.port);
        try {
          const clients = await api.getClients();
          if (clients && clients.length > 0) {
            setConnectedClient(clients[0]);
            setTimeout(() => loadSources(), 500);
          }
        } catch (e) {
          console.warn('[PCBox] getClients failed:', e);
        }
        return;
      }
    } catch (e) {
      console.warn('[PCBox] getWsServerStatus failed:', e);
    }

    const wsPort = 9898;
    try {
      console.log('[PCBox] Starting WS server on port', wsPort);
      const success = await api.startWsServer(wsPort);
      console.log('[PCBox] StartWsServer result:', success);
      if (success) setWsStatus(true, wsPort);
    } catch (e) {
      console.warn('[PCBox] startWsServer failed:', e);
    }
  };

  const checkPendingDownloads = async () => {
    try {
      const queue = await api.getDownloadQueue();
      if (queue.length > 0) {
        loadDownloadQueue();
        addToast({
          message: `Resuming ${queue.length} download(s)...`,
          type: 'info',
          action: {
            label: 'Cancel All',
            onClick: async () => {
              for (const item of queue) {
                await api.cancelDownload(item.urlHash);
              }
              loadDownloadQueue();
              addToast({ message: 'All downloads cancelled', type: 'success' });
            },
          },
          duration: 8000,
        });
      }
    } catch (e) {
      console.warn('[PCBox] checkPendingDownloads failed:', e);
    }
  };

  const setupListeners = () => {
    api.onClientConnected((client: any) => {
      setConnectedClient(client);
      setTimeout(() => loadSources(), 500);
    });

    api.onClientDisconnected(() => {
      setConnectedClient(null);
      setSources([]);
    });

    api.onWsResponse((response: any) => {
      const state = useStore.getState();
      const callback = state.topicCallbacks.get(response.topicId);
      if (callback) {
        callback(response.data);
        state.removeTopicCallback(response.topicId);
      }
    });

    api.onDownloadProgress((progress: any) => {
      setDownloadProgress(progress.id, progress);
    });
  };

  const loadSources = async () => {
    const topicId = Math.random().toString(36).substring(2) + Date.now().toString(36);

    useStore.getState().addTopicCallback(topicId, (data: any) => {
      if (Array.isArray(data)) {
        setSources(data);
        if (data.length > 0) {
          setCurrentSource(data[0]);
          setTimeout(() => {
            useStore.getState().loadHomeContent();
          }, 300);
        }
      }
    });

    const client = useStore.getState().connectedClient;
    if (client) {
      api.sendMessage(client.id, 201, { topicId });
    }
  };

  const handleStartServer = async (port: number) => {
    try {
      console.log('[PCBox] handleStartServer: checking status...');
      const status = await api.getWsServerStatus();
      console.log('[PCBox] handleStartServer: current status:', status);
      if (status && status.running) {
        setWsStatus(true, status.port);
        return;
      }
    } catch (e) {
      console.warn('[PCBox] handleStartServer: status check failed:', e);
    }

    console.log('[PCBox] handleStartServer: calling startWsServer...');
    const success = await api.startWsServer(port);
    console.log('[PCBox] handleStartServer: result:', success);
    if (success) setWsStatus(true, port);
  };

  const handleStopServer = async () => {
    await api.stopWsServer();
    setWsStatus(false, 0);
    setConnectedClient(null);
  };

  const renderContent = () => {
    if (showCacheManager) {
      return <CacheManager />;
    }
    if (showSettings) {
      return (
        <Settings
          onStartServer={handleStartServer}
          onStopServer={handleStopServer}
        />
      );
    }

    switch (viewMode) {
      case 'search':
        return <Search />;
      case 'detail':
        return <VideoDetail />;
      case 'player':
        return <PlayerView />;
      case 'history':
        return <History />;
      case 'home':
      default:
        return <Home />;
    }
  };

  const isPlayer = viewMode === 'player';

  return (
    <div className={`app ${isPlayer ? 'app-fullscreen' : ''}`}>
      {!isPlayer && (
        <Sidebar
          onOpenSettings={() => { setShowSettings(!showSettings); setShowCacheManager(false); }}
          showSettings={showSettings}
          onNavigate={() => { setShowSettings(false); setShowCacheManager(false); }}
          onOpenCacheManager={() => { setShowCacheManager(!showCacheManager); setShowSettings(false); }}
          showCacheManager={showCacheManager}
        />
      )}
      <main className="main-content">{renderContent()}</main>
      <GlobalToast />
    </div>
  );
};

export default App;
