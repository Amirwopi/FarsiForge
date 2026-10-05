export namespace core {
	
	export class GameInfo {
	    engine: string;
	    backend?: string;
	    version?: string;
	    game_name: string;
	    game_exe?: string;
	    game_root: string;
	    data_path?: string;
	    platform?: string;
	    arch?: string;
	    confidence: number;
	    evidence?: string[];
	    metadata?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new GameInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.backend = source["backend"];
	        this.version = source["version"];
	        this.game_name = source["game_name"];
	        this.game_exe = source["game_exe"];
	        this.game_root = source["game_root"];
	        this.data_path = source["data_path"];
	        this.platform = source["platform"];
	        this.arch = source["arch"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.metadata = source["metadata"];
	    }
	}

}

