export interface SourceBean {
  key: string;
  name: string;
  api: string;
  type: number;
  searchable: number;
  quickSearch: number;
  filterable: number;
  playerUrl: string;
  ext: string;
  jar: string;
  categories: string[];
  playerType: number;
  clickSelector: string;
}

export interface EpisodeInfo {
  name: string;
  url: string;
}

export interface UrlInfo {
  flag: string;
  urls: string;
  beanList: EpisodeInfo[];
}

export interface Video {
  last?: string;
  id: string;
  tid?: number;
  name: string;
  type: string;
  pic: string;
  lang?: string;
  area: string;
  year: number;
  state?: string;
  note: string;
  actor: string;
  director: string;
  des: string;
  tag: string;
  sourceKey: string;
  urlBean: {
    infoList: UrlInfo[];
  };
}

export interface VodInfo {
  id: string;
  sourceKey: string;
  name: string;
  pic: string;
  playFlag: string;
  episodeFlag: string;
  episodeUrl: string;
  episodeIndex: number;
  reverseSort: boolean;
  progress: number;
  duration: number;
  timestamp: number;
}

export interface ClientInfo {
  id: string;
  name: string;
}

export type ViewMode = 'home' | 'search' | 'detail' | 'player' | 'history';
