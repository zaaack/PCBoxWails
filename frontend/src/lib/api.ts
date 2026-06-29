import * as runtime from '../../wailsjs/runtime/runtime';

const w=(window as any);
if (w.go) {
    w.go.main.App ??= (window as any).go.main.WindowApp
}
export const isWeb = !(window as any).go?.main?.App;

export interface CachedVideo {
  id: string;
  url: string;
  videoName: string;
  filePath: string;
  isHLS: boolean;
  size: number;
}

export interface DownloadProgress {
  id: string;
  progress: number;
  status: string;
  error?: string;
}

export interface DownloadRecord {
  id: number;
  urlHash: string;
  url: string;
  headers: string;
  videoName: string;
  filePath: string;
  isHLS: boolean;
  size: number;
  status: string;
  progress: number;
  error?: string;
  createdAt: string;
  updatedAt: string;
}

export interface PagedResult {
  records: DownloadRecord[];
  total: number;
  page: number;
  pageSize: number;
}

export interface CacheStats {
  total: number;
  totalSize: number;
  pending: number;
}

export interface PlayHistoryEntry {
  sourceKey?: string;
  vodId?: string;
  vodName: string;
  vodPic?: string;
  playFlag: string;
  episodeFlag: string;
  episodeUrl: string;
  episodeIndex: number;
  reverseSort: boolean;
  progress: number;
  duration: number;
  updatedAt: number;
}

declare global {
  interface Window {
    go: {
      main: {
        App: {
          StartWsServer(port: number): Promise<boolean>;
          StopWsServer(): Promise<boolean>;
          GetWsServerStatus(): Promise<{ running: boolean; port: number }>;
          GetLocalIp(): Promise<string>;
          GetClients(): Promise<Array<{ id: string; name: string; connectedAt: number }>>;
          SendMessage(clientId: string, code: number, data: any): Promise<boolean>;
          CreateProxySession(url: string, headers: Record<string, string>): Promise<string>;
          GetProxyPort(): Promise<number>;
          SetCacheDir(dir: string): Promise<boolean>;
          GetCacheDir(): Promise<string>;
          SelectCacheDir(): Promise<string>;
          DownloadVideo(url: string, headers: Record<string, string>, videoName: string): Promise<string>;
          GetCachedFile(url: string): Promise<string>;
          GetDownloadProgress(id: string): Promise<DownloadProgress | null>;
          ListCachedFiles(): Promise<CachedVideo[]>;
          DeleteCachedFile(url: string): Promise<boolean>;
          GetDownloadQueue(): Promise<DownloadRecord[]>;
          CancelDownload(id: string): Promise<boolean>;
          RetryDownload(id: string): Promise<boolean>;
          ListCachedFilesPaged(page: number, pageSize: number, keyword: string, status: string): Promise<PagedResult>;
          DeleteCacheByID(id: number): Promise<boolean>;
          DeleteCacheBatch(ids: number[]): Promise<number>;
          GetCacheStats(): Promise<CacheStats>;
          SetKeepScreenOn(active: boolean): Promise<void>;
          SaveCacheProgress(filePath: string, progress: number, duration: number): Promise<boolean>;
          GetCacheProgress(filePath: string): Promise<{ progress: number; duration: number }>;
          DownloadVideoWithMeta(url: string, headers: Record<string, string>, videoName: string, sourceKey: string, playFlag: string, episodeIndex: number, vodId: string, vodPic: string): Promise<string>;
          SavePlayHistory(entry: PlayHistoryEntry): Promise<boolean>;
          GetPlayHistory(): Promise<PlayHistoryEntry[]>;
          FindNextCachedEpisode(sourceKey: string, playFlag: string, episodeIndex: number): Promise<DownloadRecord | null>;
          FindDownloadRecordByFilePath(filePath: string): Promise<DownloadRecord | null>;
          GetLocalIps(): Promise<string[]>;
          GetSelectedLanIp(): Promise<string>;
          SetSelectedLanIp(ip: string): Promise<boolean>;
        };
      };
    };
  }
}

type ClientConnectedCallback = (client: { id: string; name: string }) => void;
type ClientDisconnectedCallback = () => void;
type WsResponseCallback = (response: { topicId: string; code: number; data: any }) => void;
type DownloadProgressCallback = (progress: DownloadProgress) => void;

const clientConnectedListeners: ClientConnectedCallback[] = [];
const clientDisconnectedListeners: ClientDisconnectedCallback[] = [];
const wsResponseListeners: WsResponseCallback[] = [];
const downloadProgressListeners: DownloadProgressCallback[] = [];

if (!isWeb) {
  try {
    runtime.EventsOn('client-connected', (...data: any[]) => {
      const client = data[0];
      clientConnectedListeners.forEach((cb) => cb(client));
    });

    runtime.EventsOn('client-disconnected', () => {
      clientDisconnectedListeners.forEach((cb) => cb());
    });

    runtime.EventsOn('ws-response', (...data: any[]) => {
      const response = data[0];
      wsResponseListeners.forEach((cb) => cb(response));
    });

    runtime.EventsOn('download-progress', (...data: any[]) => {
      const progress = data[0] as DownloadProgress;
      downloadProgressListeners.forEach((cb) => cb(progress));
    });
  } catch (e) {
    console.warn('[PCBox] Wails runtime events unavailable:', e);
  }
} else {
  const es = new EventSource('/api/events');
  es.addEventListener('client-connected', (e) => {
    try {
      const client = JSON.parse(e.data);
      clientConnectedListeners.forEach((cb) => cb(client));
    } catch { }
  });
  es.addEventListener('client-disconnected', () => {
    clientDisconnectedListeners.forEach((cb) => cb());
  });
  es.addEventListener('ws-response', (e) => {
    try {
      const response = JSON.parse(e.data);
      wsResponseListeners.forEach((cb) => cb(response));
    } catch { }
  });
  es.addEventListener('download-progress', (e) => {
    try {
      const progress = JSON.parse(e.data) as DownloadProgress;
      downloadProgressListeners.forEach((cb) => cb(progress));
    } catch { }
  });
}

async function httpPost(method: string, body?: any): Promise<any> {
  const res = await fetch(`/api/${method}`, {
    method: body !== undefined ? 'POST' : 'GET',
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${method}`);
  const text = await res.text();
  if (!text) return null;
  return JSON.parse(text);
}

const httpAPI = {
  startWsServer: (port: number) => httpPost('StartWsServer', [port]),
  stopWsServer: () => httpPost('StopWsServer'),
  getWsServerStatus: () => httpPost('GetWsServerStatus'),
  getLocalIp: () => httpPost('GetLocalIp'),
  getClients: () => httpPost('GetClients'),
  sendMessage: (clientId: string, code: number, data: any) =>
    httpPost('SendMessage', { clientId, code, data }),
  createProxySession: (url: string, headers: Record<string, string>) =>
    httpPost('CreateProxySession', { url, headers }),
  getProxyPort: () => httpPost('GetProxyPort'),
  setCacheDir: (dir: string) => httpPost('SetCacheDir', dir),
  getCacheDir: () => httpPost('GetCacheDir'),
  selectCacheDir: () => httpPost('SelectCacheDir'),
  downloadVideo: (url: string, headers: Record<string, string>, videoName: string) =>
    httpPost('DownloadVideo', { url, headers, videoName }),
  getCachedFile: (url: string) => httpPost('GetCachedFile', url),
  getDownloadProgress: (id: string) => httpPost('GetDownloadProgress', id),
  listCachedFiles: () => httpPost('ListCachedFiles'),
  deleteCachedFile: (url: string) => httpPost('DeleteCachedFile', url),
  getDownloadQueue: () => httpPost('GetDownloadQueue'),
  cancelDownload: (id: string) => httpPost('CancelDownload', id),
  retryDownload: (id: string) => httpPost('RetryDownload', id),
  listCachedFilesPaged: (page: number, pageSize: number, keyword: string, status: string) =>
    httpPost('ListCachedFilesPaged', { page, pageSize, keyword, status }),
  deleteCacheById: (id: number) => httpPost('DeleteCacheByID', id),
  deleteCacheBatch: (ids: number[]) => httpPost('DeleteCacheBatch', ids),
  getCacheStats: () => httpPost('GetCacheStats'),
  sendTopicMessage: (clientId: string, code: number, data: any, topicId: string) =>
    httpPost('SendTopicMessage', { clientId, code, data, topicId }),
  saveCacheProgress: (filePath: string, progress: number, duration: number) =>
    httpPost('SaveCacheProgress', { filePath, progress, duration }),
  getCacheProgress: (filePath: string) =>
    httpPost('GetCacheProgress', filePath),
  savePlayHistory: (entry: PlayHistoryEntry) =>
    httpPost('SavePlayHistory', entry),
  getPlayHistory: () =>
    httpPost('GetPlayHistory'),
  findNextCachedEpisode: (sourceKey: string, playFlag: string, episodeIndex: number) =>
    httpPost('FindNextCachedEpisode', { sourceKey, playFlag, episodeIndex }),
  findDownloadRecordByFilePath: (filePath: string) =>
    httpPost('FindDownloadRecordByFilePath', filePath),
  getLocalIps: () =>
    httpPost('GetLocalIps'),
  getSelectedLanIp: () =>
    httpPost('GetSelectedLanIp'),
  setSelectedLanIp: (ip: string) =>
    httpPost('SetSelectedLanIp', ip),
};

export const api = {
  startWsServer: (port: number) =>
    isWeb ? httpAPI.startWsServer(port) : window.go.main.App.StartWsServer(port),
  stopWsServer: () =>
    isWeb ? httpAPI.stopWsServer() : window.go.main.App.StopWsServer(),
  getWsServerStatus: () =>
    isWeb ? httpAPI.getWsServerStatus() : window.go.main.App.GetWsServerStatus(),
  getLocalIp: () =>
    isWeb ? httpAPI.getLocalIp() : window.go.main.App.GetLocalIp(),
  getClients: () =>
    isWeb ? httpAPI.getClients() : window.go.main.App.GetClients(),
  sendMessage: (clientId: string, code: number, data: any) =>
    isWeb ? httpAPI.sendMessage(clientId, code, data) : window.go.main.App.SendMessage(clientId, code, data),
  createProxySession: (url: string, headers: Record<string, string>) =>
    isWeb ? httpAPI.createProxySession(url, headers) : window.go.main.App.CreateProxySession(url, headers),
  getProxyPort: () =>
    isWeb ? httpAPI.getProxyPort() : window.go.main.App.GetProxyPort(),
  setCacheDir: (dir: string) =>
    isWeb ? httpAPI.setCacheDir(dir) : window.go.main.App.SetCacheDir(dir),
  getCacheDir: () =>
    isWeb ? httpAPI.getCacheDir() : window.go.main.App.GetCacheDir(),
  selectCacheDir: () =>
    isWeb ? httpAPI.selectCacheDir() : window.go.main.App.SelectCacheDir(),
  downloadVideo: (url: string, headers: Record<string, string>, videoName: string) =>
    isWeb ? httpAPI.downloadVideo(url, headers, videoName) : window.go.main.App.DownloadVideo(url, headers, videoName),
  downloadVideoWithMeta: (url: string, headers: Record<string, string>, videoName: string, sourceKey: string, playFlag: string, episodeIndex: number, vodId: string, vodPic: string) =>
    isWeb ? httpAPI.downloadVideo(url, headers, videoName) : window.go.main.App.DownloadVideoWithMeta(url, headers, videoName, sourceKey, playFlag, episodeIndex, vodId, vodPic),
  getCachedFile: (url: string) =>
    isWeb ? httpAPI.getCachedFile(url) : window.go.main.App.GetCachedFile(url),
  getDownloadProgress: (id: string) =>
    isWeb ? httpAPI.getDownloadProgress(id) : window.go.main.App.GetDownloadProgress(id),
  listCachedFiles: () =>
    isWeb ? httpAPI.listCachedFiles() : window.go.main.App.ListCachedFiles(),
  deleteCachedFile: (url: string) =>
    isWeb ? httpAPI.deleteCachedFile(url) : window.go.main.App.DeleteCachedFile(url),
  getDownloadQueue: () =>
    isWeb ? httpAPI.getDownloadQueue() : window.go.main.App.GetDownloadQueue(),
  cancelDownload: (id: string) =>
    isWeb ? httpAPI.cancelDownload(id) : window.go.main.App.CancelDownload(id),
  retryDownload: (id: string) =>
    isWeb ? httpAPI.retryDownload(id) : window.go.main.App.RetryDownload(id),
  listCachedFilesPaged: (page: number, pageSize: number, keyword: string, status: string) =>
    isWeb ? httpAPI.listCachedFilesPaged(page, pageSize, keyword, status) : window.go.main.App.ListCachedFilesPaged(page, pageSize, keyword, status),
  deleteCacheById: (id: number) =>
    isWeb ? httpAPI.deleteCacheById(id) : window.go.main.App.DeleteCacheByID(id),
  deleteCacheBatch: (ids: number[]) =>
    isWeb ? httpAPI.deleteCacheBatch(ids) : window.go.main.App.DeleteCacheBatch(ids),
  getCacheStats: () =>
    isWeb ? httpAPI.getCacheStats() : window.go.main.App.GetCacheStats(),

  saveCacheProgress: (filePath: string, progress: number, duration: number) =>
    isWeb ? httpAPI.saveCacheProgress(filePath, progress, duration) : window.go.main.App.SaveCacheProgress(filePath, progress, duration),

  getCacheProgress: (filePath: string) =>
    isWeb ? httpAPI.getCacheProgress(filePath) : window.go.main.App.GetCacheProgress(filePath),

  savePlayHistory: (entry: PlayHistoryEntry) =>
    isWeb ? httpAPI.savePlayHistory(entry) : window.go.main.App.SavePlayHistory(entry),

  getPlayHistory: () =>
    isWeb ? httpAPI.getPlayHistory() : window.go.main.App.GetPlayHistory(),

  findNextCachedEpisode: (sourceKey: string, playFlag: string, episodeIndex: number) =>
    isWeb ? httpAPI.findNextCachedEpisode(sourceKey, playFlag, episodeIndex) : window.go.main.App.FindNextCachedEpisode(sourceKey, playFlag, episodeIndex),

  findDownloadRecordByFilePath: (filePath: string) =>
    isWeb ? httpAPI.findDownloadRecordByFilePath(filePath) : window.go.main.App.FindDownloadRecordByFilePath(filePath),

  getLocalIps: () =>
    isWeb ? httpAPI.getLocalIps() : window.go.main.App.GetLocalIps(),

  getSelectedLanIp: () =>
    isWeb ? httpAPI.getSelectedLanIp() : window.go.main.App.GetSelectedLanIp(),

  setSelectedLanIp: (ip: string) =>
    isWeb ? httpAPI.setSelectedLanIp(ip) : window.go.main.App.SetSelectedLanIp(ip),

  sendTopicMessage: async (clientId: string, code: number, data: any, topicId: string, callback: (data: any) => void) => {
    if (isWeb) {
      try {
        const result = await httpAPI.sendTopicMessage(clientId, code, data, topicId);
        callback(result);
      } catch (e) {
        console.warn('[PCBox] sendTopicMessage http failed:', e);
      }
    } else {
      const { useStore } = await import('../store');
      useStore.getState().addTopicCallback(topicId, callback);
      window.go.main.App.SendMessage(clientId, code, data);
    }
  },

  setKeepScreenOn: (_active: boolean) => {
    if (!isWeb) {
      window.go.main.App.SetKeepScreenOn(_active);
    }
  },

  onClientConnected: (callback: ClientConnectedCallback) => {
    clientConnectedListeners.push(callback);
    return () => {
      const idx = clientConnectedListeners.indexOf(callback);
      if (idx >= 0) clientConnectedListeners.splice(idx, 1);
    };
  },

  onClientDisconnected: (callback: ClientDisconnectedCallback) => {
    clientDisconnectedListeners.push(callback);
    return () => {
      const idx = clientDisconnectedListeners.indexOf(callback);
      if (idx >= 0) clientDisconnectedListeners.splice(idx, 1);
    };
  },

  onWsResponse: (callback: WsResponseCallback) => {
    wsResponseListeners.push(callback);
    return () => {
      const idx = wsResponseListeners.indexOf(callback);
      if (idx >= 0) wsResponseListeners.splice(idx, 1);
    };
  },

  onDownloadProgress: (callback: DownloadProgressCallback) => {
    downloadProgressListeners.push(callback);
    return () => {
      const idx = downloadProgressListeners.indexOf(callback);
      if (idx >= 0) downloadProgressListeners.splice(idx, 1);
    };
  },

  toggleFullscreen: async (fullscreen?: boolean) => {
    if (!isWeb) {
      try {
        if (fullscreen === undefined || fullscreen) {
          runtime.WindowFullscreen();
        } else {
          runtime.WindowUnfullscreen();
        }
        return fullscreen !== undefined ? fullscreen : true;
      } catch (e) {
        console.warn('[PCBox] toggleFullscreen error:', e);
        return false;
      }
    }
    try {
      if (fullscreen === undefined || fullscreen) {
        await document.documentElement.requestFullscreen();
      } else {
        await document.exitFullscreen();
      }
      return fullscreen !== undefined ? fullscreen : true;
    } catch (e) {
      console.warn('[PCBox] toggleFullscreen error:', e);
      return false;
    }
  },

  isFullscreen: () => {
    if (!isWeb) return Promise.resolve(false);
    return Promise.resolve(!!document.fullscreenElement);
  },

  setAlwaysOnTop: (_flag: boolean) => {
    if (!isWeb) {
      try {
        runtime.WindowSetAlwaysOnTop(_flag);
      } catch (e) {
        console.warn('[PCBox] setAlwaysOnTop error:', e);
      }
    }
    return Promise.resolve(true);
  },

  setFrame: (_frame: boolean) => Promise.resolve(true),

  isFrameless: () => Promise.resolve(false),

  setMenuBarVisibility: (_visible: boolean) => Promise.resolve(true),

  isMenuBarVisible: () => Promise.resolve(true),

  removeAllListeners: (_channel: string) => {},

  minimizeWindow: () => {
    if (!isWeb) {
      try {
        runtime.WindowMinimise();
      } catch (e) {
        console.warn('[PCBox] minimizeWindow error:', e);
      }
    }
  },

  openDevTools: () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'F12', code: 'F12' }));
  },
};

export { runtime };