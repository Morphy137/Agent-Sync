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
	
	export class Subscription {
	    id: string;
	    name: string;
	    url: string;
	    branch: string;
	    autoUpdate: boolean;
	    addedAt: string;
	    lastCheckedAt?: string;
	    lastUpdatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new Subscription(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.branch = source["branch"];
	        this.autoUpdate = source["autoUpdate"];
	        this.addedAt = source["addedAt"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.lastUpdatedAt = source["lastUpdatedAt"];
	    }
	}
	export class Location {
	    alias: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Location(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.path = source["path"];
	    }
	}
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
	    tutorialCompleted: boolean;
	    confirmBeforeSync: boolean;
	    connections: Connection[];
	    locations: Location[];
	    subscriptions: Subscription[];
	
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
	        this.tutorialCompleted = source["tutorialCompleted"];
	        this.confirmBeforeSync = source["confirmBeforeSync"];
	        this.connections = this.convertValues(source["connections"], Connection);
	        this.locations = this.convertValues(source["locations"], Location);
	        this.subscriptions = this.convertValues(source["subscriptions"], Subscription);
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

export namespace gitops {
	
	export class ChangedPath {
	    status: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new ChangedPath(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.path = source["path"];
	    }
	}
	export class ConnectRemoteResult {
	    branch: string;
	    remoteHadContent: boolean;
	    backupBranch?: string;
	    localChanges: ChangedPath[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectRemoteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.remoteHadContent = source["remoteHadContent"];
	        this.backupBranch = source["backupBranch"];
	        this.localChanges = this.convertValues(source["localChanges"], ChangedPath);
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
	    rulesPending: number;
	    rulesConflicts: number;
	
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
	        this.rulesPending = source["rulesPending"];
	        this.rulesConflicts = source["rulesConflicts"];
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
	    rulesWritten: number;
	    rulesCaptured: number;
	    rulesSkipped: number;
	
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
	        this.rulesWritten = source["rulesWritten"];
	        this.rulesCaptured = source["rulesCaptured"];
	        this.rulesSkipped = source["rulesSkipped"];
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

export namespace recognize {
	
	export class SourceInfo {
	    kind: string;
	    kindId: string;
	    name: string;
	    isDir: boolean;
	    baseName: string;
	
	    static createFrom(source: any = {}) {
	        return new SourceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.kindId = source["kindId"];
	        this.name = source["name"];
	        this.isDir = source["isDir"];
	        this.baseName = source["baseName"];
	    }
	}
	export class Suggestion {
	    profileId: string;
	    label: string;
	    location: string;
	    path: string;
	    mode: string;
	    transform?: rules.Transform;
	    detected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Suggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.label = source["label"];
	        this.location = source["location"];
	        this.path = source["path"];
	        this.mode = source["mode"];
	        this.transform = this.convertValues(source["transform"], rules.Transform);
	        this.detected = source["detected"];
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

export namespace rules {
	
	export class DestRef {
	    ruleId?: string;
	    destId?: string;
	    connectionId?: string;
	
	    static createFrom(source: any = {}) {
	        return new DestRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.destId = source["destId"];
	        this.connectionId = source["connectionId"];
	    }
	}
	export class Collision {
	    type: string;
	    path: string;
	    refs: DestRef[];
	
	    static createFrom(source: any = {}) {
	        return new Collision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.path = source["path"];
	        this.refs = this.convertValues(source["refs"], DestRef);
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
	
	export class Transform {
	    type: string;
	    key: string;
	
	    static createFrom(source: any = {}) {
	        return new Transform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.key = source["key"];
	    }
	}
	export class Destination {
	    id: string;
	    location: string;
	    path: string;
	    mode: string;
	    transform?: Transform;
	    enabled: boolean;
	    ignore?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Destination(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.location = source["location"];
	        this.path = source["path"];
	        this.mode = source["mode"];
	        this.transform = this.convertValues(source["transform"], Transform);
	        this.enabled = source["enabled"];
	        this.ignore = source["ignore"];
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
	export class Source {
	    origin: string;
	    subscription?: string;
	    path: string;
	    include?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.origin = source["origin"];
	        this.subscription = source["subscription"];
	        this.path = source["path"];
	        this.include = source["include"];
	    }
	}
	export class Rule {
	    id: string;
	    name: string;
	    enabled: boolean;
	    source: Source;
	    kind: string;
	    exclude?: string[];
	    destinations: Destination[];
	    scope?: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.source = this.convertValues(source["source"], Source);
	        this.kind = source["kind"];
	        this.exclude = source["exclude"];
	        this.destinations = this.convertValues(source["destinations"], Destination);
	        this.scope = source["scope"];
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

export namespace rulesync {
	
	export class ApplyResult {
	    written: number;
	    captured: number;
	    deleted: number;
	    backups: string[];
	    skipped: number;
	    capturedRepoPaths: string[];
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = source["written"];
	        this.captured = source["captured"];
	        this.deleted = source["deleted"];
	        this.backups = source["backups"];
	        this.skipped = source["skipped"];
	        this.capturedRepoPaths = source["capturedRepoPaths"];
	    }
	}
	export class FileStatus {
	    relPath: string;
	    sourceAbs: string;
	    targetAbs: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new FileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.sourceAbs = source["sourceAbs"];
	        this.targetAbs = source["targetAbs"];
	        this.status = source["status"];
	    }
	}
	export class DestinationStatus {
	    ruleId: string;
	    destId: string;
	    state: string;
	    targetRoot: string;
	    files: FileStatus[];
	
	    static createFrom(source: any = {}) {
	        return new DestinationStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.destId = source["destId"];
	        this.state = source["state"];
	        this.targetRoot = source["targetRoot"];
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
	export class Diff {
	    leftLabel: string;
	    rightLabel: string;
	    left: string;
	    right: string;
	    binary: boolean;
	    transform: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Diff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.leftLabel = source["leftLabel"];
	        this.rightLabel = source["rightLabel"];
	        this.left = source["left"];
	        this.right = source["right"];
	        this.binary = source["binary"];
	        this.transform = source["transform"];
	    }
	}
	
	export class RuleStatus {
	    rule: rules.Rule;
	    scope: string;
	    detectedKind: string;
	    destinations: DestinationStatus[];
	
	    static createFrom(source: any = {}) {
	        return new RuleStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rule = this.convertValues(source["rule"], rules.Rule);
	        this.scope = source["scope"];
	        this.detectedKind = source["detectedKind"];
	        this.destinations = this.convertValues(source["destinations"], DestinationStatus);
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
	export class Report {
	    rules: RuleStatus[];
	    collisions: rules.Collision[];
	    unmapped: string[];
	    pending: number;
	    conflicts: number;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rules = this.convertValues(source["rules"], RuleStatus);
	        this.collisions = this.convertValues(source["collisions"], rules.Collision);
	        this.unmapped = source["unmapped"];
	        this.pending = source["pending"];
	        this.conflicts = source["conflicts"];
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
	export class Resolution {
	    ruleId: string;
	    destId: string;
	    relPath: string;
	    choice: string;
	    content?: string;
	
	    static createFrom(source: any = {}) {
	        return new Resolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.destId = source["destId"];
	        this.relPath = source["relPath"];
	        this.choice = source["choice"];
	        this.content = source["content"];
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
	export class GitignorePayload {
	    path: string;
	    exists: boolean;
	    content: string;
	    missingRecommended: string[];
	
	    static createFrom(source: any = {}) {
	        return new GitignorePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.content = source["content"];
	        this.missingRecommended = source["missingRecommended"];
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
	export class LocationInfo {
	    alias: string;
	    path: string;
	    builtin: boolean;
	    exists: boolean;
	    profileId: string;
	    profileLabel: string;
	    ruleCount: number;
	
	    static createFrom(source: any = {}) {
	        return new LocationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.path = source["path"];
	        this.builtin = source["builtin"];
	        this.exists = source["exists"];
	        this.profileId = source["profileId"];
	        this.profileLabel = source["profileLabel"];
	        this.ruleCount = source["ruleCount"];
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
	export class SaveRuleResult {
	    rule: rules.Rule;
	    collisions: rules.Collision[];
	
	    static createFrom(source: any = {}) {
	        return new SaveRuleResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rule = this.convertValues(source["rule"], rules.Rule);
	        this.collisions = this.convertValues(source["collisions"], rules.Collision);
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
	export class SourceTreeEntry {
	    path: string;
	    isDir: boolean;
	    size: number;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new SourceTreeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.kind = source["kind"];
	    }
	}
	export class SubscriptionInfo {
	    id: string;
	    name: string;
	    url: string;
	    branch: string;
	    autoUpdate: boolean;
	    addedAt: string;
	    lastCheckedAt?: string;
	    lastUpdatedAt?: string;
	    update?: subscriptions.UpdateInfo;
	    ruleCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.branch = source["branch"];
	        this.autoUpdate = source["autoUpdate"];
	        this.addedAt = source["addedAt"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.lastUpdatedAt = source["lastUpdatedAt"];
	        this.update = this.convertValues(source["update"], subscriptions.UpdateInfo);
	        this.ruleCount = source["ruleCount"];
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
	export class SubscriptionUpdateResult {
	    changedFiles: string[];
	    applied: rulesync.ApplyResult;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changedFiles = source["changedFiles"];
	        this.applied = this.convertValues(source["applied"], rulesync.ApplyResult);
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

export namespace subscriptions {
	
	export class UpdateInfo {
	    subscriptionId: string;
	    behind: number;
	    incomingFiles: string[];
	    headCommit: string;
	    remoteCommit: string;
	    offline: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subscriptionId = source["subscriptionId"];
	        this.behind = source["behind"];
	        this.incomingFiles = source["incomingFiles"];
	        this.headCommit = source["headCommit"];
	        this.remoteCommit = source["remoteCommit"];
	        this.offline = source["offline"];
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

