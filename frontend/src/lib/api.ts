// Wails API wrapper — drop-in replacement for window.electronAPI
import * as runtime from '../../wailsjs/runtime/runtime';
;(window as any).go.main.App ??= (window as any).go.main.WindowApp

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

export const api = {
  startWsServer: (port: number) => window.go.main.App.StartWsServer(port),
  stopWsServer: () => window.go.main.App.StopWsServer(),
  getWsServerStatus: () => window.go.main.App.GetWsServerStatus(),
  getLocalIp: () => window.go.main.App.GetLocalIp(),
  getClients: () => window.go.main.App.GetClients(),
  sendMessage: (clientId: string, code: number, data: any) =>
    window.go.main.App.SendMessage(clientId, code, data),

  createProxySession: (url: string, headers: Record<string, string>) =>
    window.go.main.App.CreateProxySession(url, headers),

  getProxyPort: () => window.go.main.App.GetProxyPort(),

  setCacheDir: (dir: string) => window.go.main.App.SetCacheDir(dir),
  getCacheDir: () => window.go.main.App.GetCacheDir(),
  selectCacheDir: () => window.go.main.App.SelectCacheDir(),
  downloadVideo: (url: string, headers: Record<string, string>, videoName: string) =>
    window.go.main.App.DownloadVideo(url, headers, videoName),
  getCachedFile: (url: string) => window.go.main.App.GetCachedFile(url),
  getDownloadProgress: (id: string) => window.go.main.App.GetDownloadProgress(id),
  listCachedFiles: () => window.go.main.App.ListCachedFiles(),
  deleteCachedFile: (url: string) => window.go.main.App.DeleteCachedFile(url),
  getDownloadQueue: () => window.go.main.App.GetDownloadQueue(),
  cancelDownload: (id: string) => window.go.main.App.CancelDownload(id),
  retryDownload: (id: string) => window.go.main.App.RetryDownload(id),
  listCachedFilesPaged: (page: number, pageSize: number, keyword: string, status: string) =>
    window.go.main.App.ListCachedFilesPaged(page, pageSize, keyword, status),
  deleteCacheById: (id: number) => window.go.main.App.DeleteCacheByID(id),
  deleteCacheBatch: (ids: number[]) => window.go.main.App.DeleteCacheBatch(ids),
  getCacheStats: () => window.go.main.App.GetCacheStats(),
  setKeepScreenOn: (active: boolean) => window.go.main.App.SetKeepScreenOn(active),

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
    try {
      if (fullscreen === undefined || fullscreen) {
        runtime.WindowFullscreen();
      } else {
        runtime.WindowUnfullscreen();
      }
      return fullscreen !== undefined ? fullscreen : true;
    } catch (e) {
      console.warn('[Wails] toggleFullscreen error:', e);
      return false;
    }
  },

  isFullscreen: () => Promise.resolve(false),

  setAlwaysOnTop: (flag: boolean) => {
    try {
      runtime.WindowSetAlwaysOnTop(flag);
    } catch (e) {
      console.warn('[Wails] setAlwaysOnTop error:', e);
    }
    return Promise.resolve(true);
  },

  setFrame: (_frame: boolean) => Promise.resolve(true),

  isFrameless: () => Promise.resolve(false),

  setMenuBarVisibility: (_visible: boolean) => Promise.resolve(true),

  isMenuBarVisible: () => Promise.resolve(true),

  removeAllListeners: (_channel: string) => {},

  minimizeWindow: () => {
    try {
      runtime.WindowMinimise();
    } catch (e) {
      console.warn('[Wails] minimizeWindow error:', e);
    }
  },

  openDevTools: () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'F12', code: 'F12' }));
  },
};

export { runtime };
