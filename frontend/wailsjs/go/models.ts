export namespace main {
	
	export class PlayHistoryEntry {
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
	
	    static createFrom(source: any = {}) {
	        return new PlayHistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceKey = source["sourceKey"];
	        this.vodId = source["vodId"];
	        this.vodName = source["vodName"];
	        this.vodPic = source["vodPic"];
	        this.playFlag = source["playFlag"];
	        this.episodeFlag = source["episodeFlag"];
	        this.episodeUrl = source["episodeUrl"];
	        this.episodeIndex = source["episodeIndex"];
	        this.reverseSort = source["reverseSort"];
	        this.progress = source["progress"];
	        this.duration = source["duration"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

