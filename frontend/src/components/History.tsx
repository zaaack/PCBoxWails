import React, { useEffect } from 'react';
import { useStore } from '../store';

export const History: React.FC = () => {
  const { history, setViewMode, setCurrentVideo, setCurrentSource, sources, loadHistory, loadDetailContent, connectedClient, setHistoryHighlightEpisode } = useStore();

  const getSourceName = (sourceKey: string) => {
    const source = sources.find((s) => s.key === sourceKey);
    return source?.name || sourceKey;
  };

  useEffect(() => {
    if (connectedClient) {
      loadHistory();
    }
  }, [connectedClient]);

  const handleHistoryClick = (item: any) => {
    const sourceKey = item.sourceKey;
    const vodId = item.id;

    if (sourceKey) {
      const matchingSource = sources.find((s) => s.key === sourceKey);
      if (matchingSource) {
        setCurrentSource(matchingSource);
      }
    }

    setHistoryHighlightEpisode({
      playFlag: item.playFlag || '',
      episodeUrl: item.episodeUrl || '',
      progress: item.progress || 0,
    });

    const video: any = {
      id: vodId,
      name: item.name,
      pic: item.pic,
      sourceKey,
      playFlag: item.playFlag,
      urlBean: { infoList: [] },
    };
    setCurrentVideo(video);
    setViewMode('detail');

    setTimeout(() => {
      if (vodId) {
        loadDetailContent(vodId);
      }
    }, 300);
  };

  const formatTime = (ms: number) => {
    if (!ms) return '0:00';
    const seconds = Math.floor(ms / 1000);
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) {
      return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    }
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  return (
    <div className="history-page">
      <div className="history-header">
        <button className="btn btn-secondary" onClick={() => setViewMode('home')}>
          ← Back
        </button>
        <h2>Watch History</h2>
      </div>

      {!connectedClient && (
        <div className="empty-state">
          <p>Please connect TV-K first</p>
        </div>
      )}

      {connectedClient && history.length === 0 && (
        <div className="empty-state">
          <p>No watch history yet</p>
        </div>
      )}

      {connectedClient && history.length > 0 && (
        <div className="history-list">
          {history.map((item, index) => (
            <div
              key={`${item.id || index}-${index}`}
              className="history-item"
              onClick={() => handleHistoryClick(item)}
            >
              <div className="history-poster">
                <img src={item.pic} alt={item.name} />
              </div>
              <div className="history-info">
                <h4>{item.name}</h4>
                <p className="history-episode">
                  {item.sourceKey && <span className="source-tag">{getSourceName(item.sourceKey)}</span>}
                  {item.playFlag} - {item.episodeFlag}
                </p>
                <div className="history-progress">
                  <div
                    className="progress-bar"
                    style={{
                      width: item.duration
                        ? `${Math.min((item.progress / item.duration) * 100, 100)}%`
                        : '0%',
                    }}
                  />
                </div>
                <p className="history-time">
                  {formatTime(item.progress)} / {formatTime(item.duration)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
