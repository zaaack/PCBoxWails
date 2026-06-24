import React, { useEffect, useState } from 'react';
import { useStore } from '../store';
import { FiPlay } from 'react-icons/fi';

export const Home: React.FC = () => {
  const {
    videoList,
    categories,
    currentSource,
    setCurrentVideo,
    setViewMode,
    loadHomeContent,
    loadCategoryContent,
    loading,
    setPlayState,
    loadPlayerContent,
    connectedClient,
  } = useStore();

  const [activeCategory, setActiveCategory] = useState<string | null>(null);
  const [searchInput, setSearchInput] = useState('');

  useEffect(() => {
    if (currentSource && connectedClient && videoList.length === 0) {
      loadHomeContent();
    }
  }, [currentSource, connectedClient]);

  const handleCategoryClick = (tid: string) => {
    setActiveCategory(tid);
    loadCategoryContent(tid, '1');
  };

  const handleVideoClick = (video: any) => {
    setCurrentVideo(video);
    setViewMode('detail');
  };

  const handleQuickPlay = async (video: any, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!video.urlBean?.infoList?.[0]) return;

    const firstFlag = video.urlBean.infoList[0];
    const firstEpisode = firstFlag.beanList?.[0];
    if (!firstEpisode) return;

    setCurrentVideo(video);
    setPlayState(firstEpisode, 0, firstFlag.flag, '');
    setViewMode('player');

    const result = await loadPlayerContent(video.sourceKey, firstFlag.flag, firstEpisode.url);
    if (result) {
      setPlayState(firstEpisode, 0, firstFlag.flag, result.url, result.headers);
    }
  };

  const handleSearch = () => {
    if (searchInput.trim()) {
      useStore.getState().setSearchKeyword(searchInput.trim());
      setViewMode('search');
    }
  };

  const classList = categories?.sortList || [];

  return (
    <div className="home-page">
      <div className="home-header">
        <div className="search-bar">
          <input
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            placeholder="搜索影片..."
          />
          <button className="btn btn-primary" onClick={handleSearch}>
            搜索
          </button>
        </div>
      </div>

      {!currentSource && (
        <div className="empty-state">
          <h2>Welcome to PCBox</h2>
          <p>Please start the WebSocket server and connect TV-K app</p>
          <p>Then select a source from the sidebar</p>
        </div>
      )}

      {currentSource && (
        <>
          {classList.length > 0 && (
            <div className="category-tabs">
              <button
                className={`category-tab ${activeCategory === null ? 'active' : ''}`}
                onClick={() => {
                  setActiveCategory(null);
                  loadHomeContent();
                }}
              >
                All
              </button>
              {classList.map((cls) => (
                <button
                  key={cls.id}
                  className={`category-tab ${activeCategory === cls.id ? 'active' : ''}`}
                  onClick={() => handleCategoryClick(cls.id)}
                >
                  {cls.name}
                </button>
              ))}
            </div>
          )}

          {loading && (
            <div className="loading-state">
              <div className="spinner"></div>
              <p>Loading...</p>
            </div>
          )}

          {!loading && videoList.length > 0 && (
            <div className="video-grid">
              {videoList.map((video, index) => (
                <div
                  key={`${video.id}-${index}`}
                  className="video-card"
                  onClick={() => handleVideoClick(video)}
                >
                  <div className="video-poster">
                    <img src={video.pic} alt={video.name} loading="lazy" />
                    {video.note && <span className="video-badge">{video.note}</span>}
                    <button
                      className="quick-play-btn"
                      onClick={(e) => handleQuickPlay(video, e)}
                    >
                      <FiPlay size={14} />
                    </button>
                  </div>
                  <div className="video-info">
                    <h4 className="video-title">{video.name}</h4>
                    <p className="video-meta">
                      {video.type && <span>{video.type}</span>}
                      {video.area && <span> · {video.area}</span>}
                      {video.year > 0 && <span> · {video.year}</span>}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          )}

          {!loading && videoList.length === 0 && (
            <div className="empty-state">
              <p>No content available</p>
            </div>
          )}
        </>
      )}
    </div>
  );
};
