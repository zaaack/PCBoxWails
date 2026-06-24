import React, { useEffect, useState } from 'react';
import { useStore } from '../store';
import { TvBoxVideo } from '../lib/converter';
import { FiDownload } from 'react-icons/fi';
import { CacheModal } from './CacheModal';

export const VideoDetail: React.FC = () => {
  const {
    currentVideo,
    setViewMode,
    setPlayState,
    loadPlayerContent,
    loadDetailContent,
    currentSource,
    loading,
    historyHighlightEpisode,
  } = useStore();

  const [detailVideo, setDetailVideo] = useState<TvBoxVideo | null>(null);
  const [showCacheModal, setShowCacheModal] = useState(false);

  useEffect(() => {
    if (currentVideo && currentSource) {
      loadDetailContent(currentVideo.id);
    }
  }, [currentVideo?.id]);

  useEffect(() => {
    if (currentVideo) {
      setDetailVideo(currentVideo);
    }
  }, [currentVideo]);

  const video = detailVideo || currentVideo;

  if (!video) {
    return (
      <div className="detail-page">
        <div className="empty-state">
          <p>No video selected</p>
          <button className="btn btn-secondary" onClick={() => setViewMode('home')}>
            Back to Home
          </button>
        </div>
      </div>
    );
  }

  const handlePlayEpisode = async (episode: any, episodeIndex: number, playFlag: string) => {
    const source = currentSource || { key: video.sourceKey };
    if (!source) return;

    setPlayState(episode, episodeIndex, playFlag, '');
    setViewMode('player');

    const result = await loadPlayerContent(source.key, playFlag, episode.url);
    if (result) {
      setPlayState(episode, episodeIndex, playFlag, result.url, result.headers);
    }
  };

  const playFlags = video.urlBean?.infoList || [];

  return (
    <div className="detail-page">
      <div className="detail-header">
        <button className="btn btn-secondary" onClick={() => setViewMode('home')}>
          ← Back
        </button>
        {playFlags.length > 0 && (
          <button className="btn btn-primary" onClick={() => setShowCacheModal(true)}>
            <FiDownload size={14} /> Cache
          </button>
        )}
      </div>

      {loading && (
        <div className="loading-state">
          <div className="spinner"></div>
          <p>Loading...</p>
        </div>
      )}

      {!loading && (
        <>
          <div className="detail-content">
            <div className="detail-poster">
              <img src={video.pic} alt={video.name} />
            </div>

            <div className="detail-info">
              <h1 className="detail-title">{video.name}</h1>
              <div className="detail-meta">
                {video.type && <span>Type: {video.type}</span>}
                {video.area && <span> · {video.area}</span>}
                {video.year > 0 && <span> · {video.year}</span>}
              </div>
              {video.note && <p className="detail-note">{video.note}</p>}
              {video.director && <p className="detail-director">Director: {video.director}</p>}
              {video.actor && <p className="detail-actor">Cast: {video.actor}</p>}
              {video.des && (
                <div className="detail-desc">
                  <h3>Description</h3>
                  <p>{video.des.replace(/<[^>]*>/g, '')}</p>
                </div>
              )}
            </div>
          </div>

          {playFlags.length > 0 && (
            <div className="episode-section">
              <h3>Episodes</h3>
              {playFlags.map((flagInfo, flagIndex) => (
                <div key={flagIndex} className="episode-group">
                  <h4 className="flag-name">{flagInfo.flag}</h4>
                  <div className="episode-list">
                    {flagInfo.beanList.map((episode, epIndex) => {
                      const isHighlighted =
                        historyHighlightEpisode &&
                        historyHighlightEpisode.playFlag === flagInfo.flag &&
                        historyHighlightEpisode.episodeUrl === episode.url;
                      return (
                        <button
                          key={epIndex}
                          className={`episode-btn ${isHighlighted ? 'active' : ''}`}
                          onClick={() => handlePlayEpisode(episode, epIndex, flagInfo.flag)}
                        >
                          {episode.name}
                        </button>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}

          {playFlags.length === 0 && !loading && (
            <div className="empty-state">
              <p>No episodes available</p>
            </div>
          )}
        </>
      )}

      {showCacheModal && (
        <CacheModal
          videoName={video.name}
          playFlags={playFlags}
          onClose={() => setShowCacheModal(false)}
        />
      )}
    </div>
  );
};
