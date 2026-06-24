/**
 * CatVod -> TVBox format converter
 */

export interface CatVodResult {
  class?: { type_id: string; type_name: string; type_flag?: string }[];
  list?: CatVodVod[];
  filters?: Record<string, any[]>;
  page?: number;
  pagecount?: number;
  limit?: number;
  total?: number;
  url?: any;
  header?: string;
  format?: string;
  msg?: string;
}

export interface CatVodVod {
  type_name?: string;
  vod_id: string;
  vod_name: string;
  vod_pic: string;
  vod_remarks?: string;
  vod_year?: string;
  vod_area?: string;
  vod_actor?: string;
  vod_director?: string;
  vod_content?: string;
  vod_play_from?: string;
  vod_play_url?: string;
  vod_tag?: string;
  style?: any;
}

export interface CatVodHistory {
  key: string;
  vodPic: string;
  vodName: string;
  vodFlag: string;
  vodRemarks: string;
  episodeUrl: string;
  revSort: boolean;
  position: number;
  duration: number;
  createTime: number;
}

export interface TvBoxVideo {
  id: string;
  name: string;
  pic: string;
  type: string;
  area: string;
  year: number;
  actor: string;
  director: string;
  des: string;
  note: string;
  tag: string;
  sourceKey: string;
  urlBean: {
    infoList: {
      flag: string;
      urls: string;
      beanList: { name: string; url: string }[];
    }[];
  };
}

export interface TvBoxMovie {
  page: number;
  pagecount: number;
  pagesize: number;
  recordcount: number;
  videoList: TvBoxVideo[];
}

export interface TvBoxMovieSort {
  sortList: {
    id: string;
    name: string;
    flag: string;
    filters: any[];
  }[];
}

export interface TvBoxAbsSortXml {
  classes: TvBoxMovieSort;
  list: TvBoxMovie | null;
}

export interface TvBoxAbsXml {
  movie: TvBoxMovie;
}

function vodToVideo(vod: CatVodVod, sourceKey: string): TvBoxVideo {
  const playFroms = (vod.vod_play_from || '').split('$$$').filter(Boolean);
  const playUrls = (vod.vod_play_url || '').split('$$$');

  const urlInfos: TvBoxVideo['urlBean']['infoList'] = [];

  for (let i = 0; i < playFroms.length; i++) {
    const playUrl = playUrls[i];
    if (!playUrl) continue;

    const eps = playUrl.split('#').filter(Boolean);
    const beanList: { name: string; url: string }[] = [];

    for (const ep of eps) {
      const dollarIdx = ep.indexOf('$');
      if (dollarIdx > 0) {
        beanList.push({ name: ep.substring(0, dollarIdx), url: ep.substring(dollarIdx + 1) });
      }
    }

    urlInfos.push({
      flag: playFroms[i],
      urls: playUrl,
      beanList,
    });
  }

  return {
    id: vod.vod_id,
    name: vod.vod_name,
    pic: vod.vod_pic,
    type: vod.type_name || '',
    area: vod.vod_area || '',
    year: parseInt(vod.vod_year || '0') || 0,
    actor: vod.vod_actor || '',
    director: vod.vod_director || '',
    des: vod.vod_content || '',
    note: vod.vod_remarks || '',
    tag: vod.vod_tag || '',
    sourceKey,
    urlBean: { infoList: urlInfos },
  };
}

export function resultToAbsSortXml(result: CatVodResult, sourceKey: string): TvBoxAbsSortXml {
  const classes: TvBoxMovieSort['sortList'] = (result.class || []).map((c) => ({
    id: c.type_id,
    name: c.type_name,
    flag: c.type_flag || '',
    filters: [],
  }));

  const videoList = (result.list || []).map((v) => vodToVideo(v, sourceKey));

  return {
    classes: { sortList: classes },
    list: {
      page: result.page || 1,
      pagecount: result.pagecount || 1,
      pagesize: result.limit || videoList.length,
      recordcount: result.total || videoList.length,
      videoList,
    },
  };
}

export function resultToAbsXml(result: CatVodResult, sourceKey: string): TvBoxAbsXml {
  const videoList = (result.list || []).map((v) => vodToVideo(v, sourceKey));

  return {
    movie: {
      page: result.page || 1,
      pagecount: result.pagecount || 1,
      pagesize: result.limit || videoList.length,
      recordcount: result.total || videoList.length,
      videoList,
    },
  };
}

export function catVodPlayContentToTvBoxPlayContent(data: any): any {
  if (!data) return null;

  if (data.url && typeof data.url === 'object' && data.url.values && data.url.position !== undefined) {
    const values = data.url.values;
    const position = data.url.position;
    if (Array.isArray(values) && typeof position === 'number' && position >= 0 && position < values.length) {
      const entry = values[position];
      if (entry && entry.v) {
        return { nameValuePairs: { url: entry.v } };
      }
    }
    return null;
  }

  if (typeof data.url === 'string') {
    return { nameValuePairs: { url: data.url } };
  }

  return data;
}

export function historyToVodInfo(history: CatVodHistory): any {
  const keyParts = (history.key || '').split('@@@');
  const sourceKey = keyParts[0] || '';
  const vodId = keyParts[1] || '';

  return {
    id: vodId,
    sourceKey,
    name: history.vodName,
    pic: history.vodPic,
    playFlag: history.vodFlag || '',
    episodeFlag: history.vodRemarks || '',
    episodeUrl: history.episodeUrl || '',
    episodeIndex: 0,
    reverseSort: history.revSort,
    progress: history.position || 0,
    duration: history.duration || 0,
    timestamp: history.createTime || 0,
  };
}
