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
	export class PersianOptions {
	    reshape: boolean;
	    bidi_reorder: boolean;
	    fix_yeh: boolean;
	    persian_digits: boolean;
	    convert_punct: boolean;
	    drop_diacritics: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PersianOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reshape = source["reshape"];
	        this.bidi_reorder = source["bidi_reorder"];
	        this.fix_yeh = source["fix_yeh"];
	        this.persian_digits = source["persian_digits"];
	        this.convert_punct = source["convert_punct"];
	        this.drop_diacritics = source["drop_diacritics"];
	    }
	}
	export class StringEntry {
	    id: string;
	    source: string;
	    translation?: string;
	    file: string;
	    path: string;
	    context?: string;
	    speaker?: string;
	    line?: number;
	    status: string;
	    notes?: string;
	    max_length?: number;
	
	    static createFrom(source: any = {}) {
	        return new StringEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.translation = source["translation"];
	        this.file = source["file"];
	        this.path = source["path"];
	        this.context = source["context"];
	        this.speaker = source["speaker"];
	        this.line = source["line"];
	        this.status = source["status"];
	        this.notes = source["notes"];
	        this.max_length = source["max_length"];
	    }
	}
	export class Project {
	    name: string;
	    game_name: string;
	    game_root: string;
	    engine: string;
	    backend?: string;
	    version?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    entries: StringEntry[];
	    working_dir?: string;
	    project_dir?: string;
	    persian_opts: PersianOptions;
	    font_path?: string;
	    extracted_files?: string[];
	    modified_files?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.game_name = source["game_name"];
	        this.game_root = source["game_root"];
	        this.engine = source["engine"];
	        this.backend = source["backend"];
	        this.version = source["version"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.entries = this.convertValues(source["entries"], StringEntry);
	        this.working_dir = source["working_dir"];
	        this.project_dir = source["project_dir"];
	        this.persian_opts = this.convertValues(source["persian_opts"], PersianOptions);
	        this.font_path = source["font_path"];
	        this.extracted_files = source["extracted_files"];
	        this.modified_files = source["modified_files"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class BuildPatcherResult {
	    output_dir: string;
	    patch_file: string;
	    patcher_exe: string;
	    target_count: number;
	
	    static createFrom(source: any = {}) {
	        return new BuildPatcherResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output_dir = source["output_dir"];
	        this.patch_file = source["patch_file"];
	        this.patcher_exe = source["patcher_exe"];
	        this.target_count = source["target_count"];
	    }
	}
	export class Settings {
	    tools_dir: string;
	    output_dir: string;
	    game_search_paths: string[];
	    default_persian_opts: core.PersianOptions;
	    default_author: string;
	    default_language: string;
	    log_level: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tools_dir = source["tools_dir"];
	        this.output_dir = source["output_dir"];
	        this.game_search_paths = source["game_search_paths"];
	        this.default_persian_opts = this.convertValues(source["default_persian_opts"], core.PersianOptions);
	        this.default_author = source["default_author"];
	        this.default_language = source["default_language"];
	        this.log_level = source["log_level"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolStatus {
	    tools_dir: string;
	    python: string;
	    python_version: string;
	    python_ok: boolean;
	    unity_py: boolean;
	    ff_tools: string;
	    ff_tools_ok: boolean;
	    patcher_exe: string;
	    patcher_ok: boolean;
	    available: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new ToolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tools_dir = source["tools_dir"];
	        this.python = source["python"];
	        this.python_version = source["python_version"];
	        this.python_ok = source["python_ok"];
	        this.unity_py = source["unity_py"];
	        this.ff_tools = source["ff_tools"];
	        this.ff_tools_ok = source["ff_tools_ok"];
	        this.patcher_exe = source["patcher_exe"];
	        this.patcher_ok = source["patcher_ok"];
	        this.available = source["available"];
	    }
	}

}

