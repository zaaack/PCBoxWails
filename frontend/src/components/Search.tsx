import React, { useState, useMemo } from 'react';
import { useStore } from '../store';

export const Search: React.FC = () => {
  const {
    search,
    searchResults,
    loading,
    setCurrentVideo,
    setViewMode,
    searchKeyword,
    setSearchKeyword,
    sources,
    searchSelectedSources,
    toggleSearchSource,
    selectAllSearchSources,
    searchSourcesShowCount,
    setSearchSourcesShowCount,
  } = useStore();

  const [inputValue, setInputValue] = useState(searchKeyword);

  const searchableSources = useMemo(
    () => sources.filter((s) => s.searchable === 1),
    [sources]
  );

  const visibleSources = useMemo(
    () => searchableSources.slice(0, searchSourcesShowCount),
    [searchableSources, searchSourcesShowCount]
  );

  const hasMoreSources = searchableSources.length > searchSourcesShowCount;

  const handleSearch = () => {
    if (inputValue.trim()) {
      search(inputValue.trim());
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  const getSourceName = (sourceKey: string) => {
    const source = sources.find((s) => s.key === sourceKey);
    return source?.name || sourceKey;
  };

  const handleVideoClick = (video: any) => {
    const matchingSource = sources.find((s) => s.key === video.sourceKey);
    if (matchingSource) {
      useStore.getState().setCurrentSource(matchingSource);
    }
    setCurrentVideo(video);
    setViewMode('detail');
  };

  const handleSelectAll = () => {
    selectAllSearchSources();
  };

  return (
    <div className="search-page">
      <div className="search-layout">
        <div className="search-sidebar">
          <div className="search-sidebar-header">
            <h3>Sources ({searchSelectedSources.size}/{searchableSources.length})</h3>
            <button
              className="btn-text"
              onClick={handleSelectAll}
            >
              {searchSelectedSources.size === searchableSources.length ? 'Deselect All' : 'Select All'}
            </button>
          </div>
          <div className="search-source-list">
            {visibleSources.map((source) => (
              <label
                key={source.key}
                className={`search-source-item ${searchSelectedSources.has(source.key) ? 'selected' : ''}`}
              >
                <input
                  type="checkbox"
                  checked={searchSelectedSources.has(source.key)}
                  onChange={() => toggleSearchSource(source.key)}
                />
                <span className="source-name">{source.name}</span>
              </label>
            ))}
          </div>
          {hasMoreSources && (
            <button
              className="btn-load-more"
              onClick={() => setSearchSourcesShowCount(searchSourcesShowCount + 15)}
            >
              Show More ({searchableSources.length - searchSourcesShowCount} remaining)
            </button>
          )}
          {searchSourcesShowCount > 15 && (
            <button
              className="btn-load-more btn-collapse"
              onClick={() => setSearchSourcesShowCount(15)}
            >
              Collapse
            </button>
          )}
        </div>

        <div className="search-main">
          <div className="search-box">
            <input
              type="text"
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Search movies/TV shows..."
              className="search-input"
            />
            <button className="btn btn-primary" onClick={handleSearch} disabled={loading}>
              {loading ? 'Searching...' : 'Search'}
            </button>
          </div>

          {loading && (
            <div className="loading-state">
              <div className="spinner"></div>
              <p>Searching...</p>
            </div>
          )}

          {!loading && searchResults.length > 0 && (
            <div className="search-results">
              <h3>Search Results ({searchResults.length})</h3>
              <div className="video-grid">
                {searchResults.map((video, index) => (
                  <div
                    key={`${video.id}-${index}`}
                    className="video-card"
                    onClick={() => handleVideoClick(video)}
                  >
                    <div className="video-poster">
                      <img src={video.pic} alt={video.name} loading="lazy" />
                      {video.note && <span className="video-badge">{video.note}</span>}
                    </div>
                    <div className="video-info">
                      <h4 className="video-title">{video.name}</h4>
                      <p className="video-meta">
                        {video.sourceKey && <span className="source-tag">{getSourceName(video.sourceKey)}</span>}
                        {video.type && <span>{video.type}</span>}
                        {video.area && <span> · {video.area}</span>}
                        {video.year > 0 && <span> · {video.year}</span>}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {!loading && searchResults.length === 0 && searchKeyword && (
            <div className="empty-state">
              <p>No results found for "{searchKeyword}"</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
