export namespace backup {
	
	export class BackupInfo {
	    id: string;
	    connectionId: string;
	    profile: string;
	    targetPath: string;
	    createdAt: string;
	    reason: string;
	    files: string[];
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.connectionId = source["connectionId"];
	        this.profile = source["profile"];
	        this.targetPath = source["targetPath"];
	        this.createdAt = source["createdAt"];
	        this.reason = source["reason"];
	        this.files = source["files"];
	        this.path = source["path"];
	    }
	}

}

export namespace config {
	
	export class Connection {
	    id: string;
	    profile: string;
	    targetPath: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profile = source["profile"];
	        this.targetPath = source["targetPath"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Config {
	    version: number;
	    rootPath: string;
	    backupPath: string;
	    backupRetention: number;
	    checkIntervalMinutes: number;
	    language: string;
	    theme: string;
	    confirmBeforeSync: boolean;
	    connections: Connection[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.rootPath = source["rootPath"];
	        this.backupPath = source["backupPath"];
	        this.backupRetention = source["backupRetention"];
	        this.checkIntervalMinutes = source["checkIntervalMinutes"];
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.confirmBeforeSync = source["confirmBeforeSync"];
	        this.connections = this.convertValues(source["connections"], Connection);
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

export namespace orchestrator {
	
	export class ConflictItem {
	    profile: string;
	    relPath: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.relPath = source["relPath"];
	        this.reason = source["reason"];
	    }
	}
	export class ConflictResolution {
	    profile: string;
	    relPath: string;
	    resolution: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictResolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.relPath = source["relPath"];
	        this.resolution = source["resolution"];
	    }
	}
	export class SyncItem {
	    profile: string;
	    relPath: string;
	    fullPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.relPath = source["relPath"];
	        this.fullPath = source["fullPath"];
	    }
	}
	export class SyncPlan {
	    fingerprint: string;
	    direction: string;
	    downloads: SyncItem[];
	    uploads: SyncItem[];
	    conflicts: ConflictItem[];
	    commitPreview: string[];
	    ahead: number;
	    behind: number;
	    hasRemote: boolean;
	    isOffline: boolean;
	    dirtyOutside: string[];
	    dirtyOverlapExists: boolean;
	    connections: syncer.ConnectionStatus[];
	    lastCheckAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fingerprint = source["fingerprint"];
	        this.direction = source["direction"];
	        this.downloads = this.convertValues(source["downloads"], SyncItem);
	        this.uploads = this.convertValues(source["uploads"], SyncItem);
	        this.conflicts = this.convertValues(source["conflicts"], ConflictItem);
	        this.commitPreview = source["commitPreview"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.hasRemote = source["hasRemote"];
	        this.isOffline = source["isOffline"];
	        this.dirtyOutside = source["dirtyOutside"];
	        this.dirtyOverlapExists = source["dirtyOverlapExists"];
	        this.connections = this.convertValues(source["connections"], syncer.ConnectionStatus);
	        this.lastCheckAt = source["lastCheckAt"];
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
	export class SyncResult {
	    applied: number;
	    captured: number;
	    pulled: number;
	    pushed: number;
	    backupSessions: string[];
	
	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applied = source["applied"];
	        this.captured = source["captured"];
	        this.pulled = source["pulled"];
	        this.pushed = source["pushed"];
	        this.backupSessions = source["backupSessions"];
	    }
	}

}

export namespace profiles {
	
	export class Profile {
	    name: string;
	    path: string;
	    fileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.fileCount = source["fileCount"];
	    }
	}

}

export namespace service {
	
	export class AgentFolderSuggestion {
	    path: string;
	    label: string;
	    suggestedProfile: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentFolderSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.suggestedProfile = source["suggestedProfile"];
	    }
	}
	export class DraftDifference {
	    relPath: string;
	    repoHash: string;
	    targetHash: string;
	
	    static createFrom(source: any = {}) {
	        return new DraftDifference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.repoHash = source["repoHash"];
	        this.targetHash = source["targetHash"];
	    }
	}
	export class ConnectionDraftPreview {
	    toImport: string[];
	    toCreateInTarget: string[];
	    toOverwrite: string[];
	    differences: DraftDifference[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectionDraftPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toImport = source["toImport"];
	        this.toCreateInTarget = source["toCreateInTarget"];
	        this.toOverwrite = source["toOverwrite"];
	        this.differences = this.convertValues(source["differences"], DraftDifference);
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
	export class ConnectionDraft {
	    id: string;
	    profile: string;
	    targetPath: string;
	    createProfile: boolean;
	    importPaths: string[];
	    replacesConnectionId?: string;
	    // Go type: time
	    createdAt: any;
	    preview: ConnectionDraftPreview;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profile = source["profile"];
	        this.targetPath = source["targetPath"];
	        this.createProfile = source["createProfile"];
	        this.importPaths = source["importPaths"];
	        this.replacesConnectionId = source["replacesConnectionId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.preview = this.convertValues(source["preview"], ConnectionDraftPreview);
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
	
	export class ConnectionDraftRequest {
	    profile: string;
	    targetPath: string;
	    createProfile: boolean;
	    importPaths: string[];
	    replacesConnectionId?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionDraftRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.targetPath = source["targetPath"];
	        this.createProfile = source["createProfile"];
	        this.importPaths = source["importPaths"];
	        this.replacesConnectionId = source["replacesConnectionId"];
	    }
	}
	
	export class GitInfoPayload {
	    isInstalled: boolean;
	    isRepo: boolean;
	    branch: string;
	    remoteUrl: string;
	    hasUpstream: boolean;
	    ahead: number;
	    behind: number;
	    userName: string;
	    userEmail: string;
	
	    static createFrom(source: any = {}) {
	        return new GitInfoPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isInstalled = source["isInstalled"];
	        this.isRepo = source["isRepo"];
	        this.branch = source["branch"];
	        this.remoteUrl = source["remoteUrl"];
	        this.hasUpstream = source["hasUpstream"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.userName = source["userName"];
	        this.userEmail = source["userEmail"];
	    }
	}
	export class InitialLinkResult {
	    pendingId: string;
	    connection: config.Connection;
	    needsDecision: boolean;
	    untrackedFiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new InitialLinkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pendingId = source["pendingId"];
	        this.connection = this.convertValues(source["connection"], config.Connection);
	        this.needsDecision = source["needsDecision"];
	        this.untrackedFiles = source["untrackedFiles"];
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
	export class PathInfo {
	    normalized: string;
	    exists: boolean;
	    isDir: boolean;
	    writable: boolean;
	    isInsideRoot: boolean;
	    alreadyConnected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PathInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.normalized = source["normalized"];
	        this.exists = source["exists"];
	        this.isDir = source["isDir"];
	        this.writable = source["writable"];
	        this.isInsideRoot = source["isInsideRoot"];
	        this.alreadyConnected = source["alreadyConnected"];
	    }
	}
	export class SettingsPayload {
	    language: string;
	    theme: string;
	    backupPath: string;
	    backupRetention: number;
	    checkIntervalMinutes: number;
	    confirmBeforeSync: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.backupPath = source["backupPath"];
	        this.backupRetention = source["backupRetention"];
	        this.checkIntervalMinutes = source["checkIntervalMinutes"];
	        this.confirmBeforeSync = source["confirmBeforeSync"];
	    }
	}
	export class TargetEntry {
	    relPath: string;
	    isDir: boolean;
	    size: number;
	    fileCount: number;
	    tooLarge: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new TargetEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.fileCount = source["fileCount"];
	        this.tooLarge = source["tooLarge"];
	        this.reason = source["reason"];
	    }
	}

}

export namespace syncer {
	
	export class FileStatus {
	    relPath: string;
	    status: string;
	    sourceHash: string;
	    targetHash: string;
	    baselineHash: string;
	
	    static createFrom(source: any = {}) {
	        return new FileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.status = source["status"];
	        this.sourceHash = source["sourceHash"];
	        this.targetHash = source["targetHash"];
	        this.baselineHash = source["baselineHash"];
	    }
	}
	export class ConnectionStatus {
	    connectionId: string;
	    profile: string;
	    targetPath: string;
	    status: string;
	    files: FileStatus[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.profile = source["profile"];
	        this.targetPath = source["targetPath"];
	        this.status = source["status"];
	        this.files = this.convertValues(source["files"], FileStatus);
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

