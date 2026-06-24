// Wails API wrapper — drop-in replacement for window.electronAPI
import * as runtime from '../../wailsjs/runtime/runtime';

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
        };
      };
    };
  }
}

type ClientConnectedCallback = (client: { id: string; name: string }) => void;
type ClientDisconnectedCallback = () => void;
type WsResponseCallback = (response: { topicId: string; code: number; data: any }) => void;

const clientConnectedListeners: ClientConnectedCallback[] = [];
const clientDisconnectedListeners: ClientDisconnectedCallback[] = [];
const wsResponseListeners: WsResponseCallback[] = [];

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
};

export { runtime };
