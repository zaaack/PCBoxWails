export namespace main {
	
	export class ClientInfo {
	    id: string;
	    name: string;
	    connectedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ClientInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.connectedAt = source["connectedAt"];
	    }
	}

}

