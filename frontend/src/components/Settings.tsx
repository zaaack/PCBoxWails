import React, { useState } from 'react';
import { useStore } from '../store';
import { api } from '../lib/api';

interface SettingsProps {
  onStartServer: (port: number) => void;
  onStopServer: () => void;
}

export const Settings: React.FC<SettingsProps> = ({ onStartServer, onStopServer }) => {
  const { wsRunning, wsPort, localIp, connectedClient, theme, setTheme, menuBarVisible, setMenuBarVisible } = useStore();
  const [port, setPort] = useState(wsPort);

  return (
    <div className="settings">
      <h2>Settings</h2>

      <div className="settings-section">
        <h3>Appearance</h3>
        <div className="settings-row">
          <label>Theme:</label>
          <select
            className="settings-input"
            value={theme}
            onChange={(e) => setTheme(e.target.value as 'light' | 'dark' | 'system')}
          >
            <option value="dark">Dark</option>
            <option value="light">Light</option>
            <option value="system">System</option>
          </select>
        </div>
        <div className="settings-row">
          <label>Menu Bar:</label>
          <button
            className={`btn ${menuBarVisible ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => setMenuBarVisible(!menuBarVisible)}
          >
            {menuBarVisible ? 'Visible' : 'Hidden'}
          </button>
        </div>
      </div>

      <div className="settings-section">
        <h3>WebSocket Server</h3>
        <div className="settings-row">
          <label>Status:</label>
          <span className={wsRunning ? 'status-connected' : 'status-disconnected'}>
            {wsRunning ? 'Running' : 'Stopped'}
          </span>
        </div>
        <div className="settings-row">
          <label>Port:</label>
          <input
            type="number"
            value={port}
            onChange={(e) => setPort(parseInt(e.target.value) || 9898)}
            disabled={wsRunning}
            className="settings-input"
          />
        </div>
        <div className="settings-row">
          <label>Local IP:</label>
          <span>{localIp}</span>
        </div>
        <div className="settings-actions">
          {!wsRunning ? (
            <button className="btn btn-primary" onClick={() => onStartServer(port)}>
              Start Server
            </button>
          ) : (
            <button className="btn btn-danger" onClick={onStopServer}>
              Stop Server
            </button>
          )}
        </div>
      </div>

      {wsRunning && (
        <div className="settings-section">
          <h3>Pairing Info</h3>
          <div className="pairing-info">
            <p>IP Address: <strong>{localIp}</strong></p>
            <p>Port: <strong>{wsPort}</strong></p>
            <p className="hint">
              Open TV-K App → Settings → FreeBox Pairing<br />
              Enter the IP and Port above, then click Connect
            </p>
          </div>
        </div>
      )}

      {connectedClient && (
        <div className="settings-section">
          <h3>Connected Client</h3>
          <div className="client-info">
            <p>Name: <strong>{connectedClient.name}</strong></p>
            <p>ID: <strong>{connectedClient.id}</strong></p>
          </div>
        </div>
      )}

      <div className="settings-section">
        <h3>Developer</h3>
        <div className="settings-actions">
          <button className="btn btn-secondary" onClick={() => api.openDevTools()}>
            Open DevTools
          </button>
        </div>
      </div>
    </div>
  );
};
