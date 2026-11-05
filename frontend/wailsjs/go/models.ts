export namespace autostart {
	
	export class Status {
	    supported: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.enabled = source["enabled"];
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
	export class Config {
	    version: number;
	    rootPath: string;
	    backupPath: string;
	    backupRetention: number;
	    checkIntervalMinutes: number;
	    language: string;
	    theme: string;
	    tutorialCompleted: boolean;
	    disableUpdateCheck: boolean;
	    autoApplyUpdates: boolean;
	    closeToTray: boolean;
	    startWithWindows: boolean;
	    confirmBeforeSync: boolean;
	    locations: Location[];
	    homeLocationDisabled?: boolean;
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
	        this.disableUpdateCheck = source["disableUpdateCheck"];
	        this.autoApplyUpdates = source["autoApplyUpdates"];
	        this.closeToTray = source["closeToTray"];
	        this.startWithWindows = source["startWithWindows"];
	        this.confirmBeforeSync = source["confirmBeforeSync"];
	        this.locations = this.convertValues(source["locations"], Location);
	        this.homeLocationDisabled = source["homeLocationDisabled"];
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

export namespace main {
	
	export class AppInfo {
	    version: string;
	    installed: boolean;
	    portable: boolean;
	    dataDir: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.installed = source["installed"];
	        this.portable = source["portable"];
	        this.dataDir = source["dataDir"];
	    }
	}
	export class UpdateStatus {
	    current: string;
	    available: boolean;
	    release?: updater.Release;
	
	    static createFrom(source: any = {}) {
	        return new UpdateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.available = source["available"];
	        this.release = this.convertValues(source["release"], updater.Release);
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
	    path: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reason = source["reason"];
	    }
	}
	export class ConflictResolution {
	    path: string;
	    resolution: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictResolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.resolution = source["resolution"];
	    }
	}
	export class SyncPlan {
	    fingerprint: string;
	    direction: string;
	    conflicts: ConflictItem[];
	    commitPreview: string[];
	    ahead: number;
	    behind: number;
	    hasRemote: boolean;
	    isOffline: boolean;
	    dirtyOutside: string[];
	    dirtyOverlapExists: boolean;
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
	        this.conflicts = this.convertValues(source["conflicts"], ConflictItem);
	        this.commitPreview = source["commitPreview"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.hasRemote = source["hasRemote"];
	        this.isOffline = source["isOffline"];
	        this.dirtyOutside = source["dirtyOutside"];
	        this.dirtyOverlapExists = source["dirtyOverlapExists"];
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
	        this.pulled = source["pulled"];
	        this.pushed = source["pushed"];
	        this.backupSessions = source["backupSessions"];
	        this.rulesWritten = source["rulesWritten"];
	        this.rulesCaptured = source["rulesCaptured"];
	        this.rulesSkipped = source["rulesSkipped"];
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
	
	export class Category {
	    id: string;
	    name: string;
	    color: string;
	    icon?: string;
	    scope?: string;
	    deleted?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.scope = source["scope"];
	        this.deleted = source["deleted"];
	    }
	}
	export class DestRef {
	    ruleId: string;
	    destId: string;
	
	    static createFrom(source: any = {}) {
	        return new DestRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.destId = source["destId"];
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
	    categoryId?: string;
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
	        this.categoryId = source["categoryId"];
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
	
	export class BackupEntry {
	    id: string;
	    kind: string;
	    owner: string;
	    destination: string;
	    ruleId: string;
	    destId: string;
	    targetPath: string;
	    createdAt: string;
	    reason: string;
	    fileCount: number;
	    totalSize: number;
	    files: string[];
	    path: string;
	    restorable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BackupEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.owner = source["owner"];
	        this.destination = source["destination"];
	        this.ruleId = source["ruleId"];
	        this.destId = source["destId"];
	        this.targetPath = source["targetPath"];
	        this.createdAt = source["createdAt"];
	        this.reason = source["reason"];
	        this.fileCount = source["fileCount"];
	        this.totalSize = source["totalSize"];
	        this.files = source["files"];
	        this.path = source["path"];
	        this.restorable = source["restorable"];
	    }
	}
	export class FileDirectoryEntry {
	    path: string;
	    name: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileDirectoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.isDir = source["isDir"];
	    }
	}
	export class FileRoot {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new FileRoot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
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
	    }
	}
	export class RestoreBackupResult {
	    restored: string[];
	    target: string;
	    undoBackupId: string;
	
	    static createFrom(source: any = {}) {
	        return new RestoreBackupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.restored = source["restored"];
	        this.target = source["target"];
	        this.undoBackupId = source["undoBackupId"];
	    }
	}
	export class RuleSourceContent {
	    content: string;
	    hash: string;
	    readOnly: boolean;
	    absolutePath: string;
	
	    static createFrom(source: any = {}) {
	        return new RuleSourceContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.hash = source["hash"];
	        this.readOnly = source["readOnly"];
	        this.absolutePath = source["absolutePath"];
	    }
	}
	export class RuleSourceFile {
	    path: string;
	    name: string;
	    absolutePath: string;
	
	    static createFrom(source: any = {}) {
	        return new RuleSourceFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.absolutePath = source["absolutePath"];
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
	    disableUpdateCheck: boolean;
	    autoApplyUpdates: boolean;
	    closeToTray: boolean;
	    startWithWindows: boolean;
	
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
	        this.disableUpdateCheck = source["disableUpdateCheck"];
	        this.autoApplyUpdates = source["autoApplyUpdates"];
	        this.closeToTray = source["closeToTray"];
	        this.startWithWindows = source["startWithWindows"];
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
	    suggestionsPending: number;
	
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
	        this.suggestionsPending = source["suggestionsPending"];
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
	export class SuggestionApplyResult {
	    id: string;
	    status: string;
	    ruleId: string;
	    errorCode: string;
	    errorReason: string;
	    errorAlias: string;
	    collisions: number;
	
	    static createFrom(source: any = {}) {
	        return new SuggestionApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.ruleId = source["ruleId"];
	        this.errorCode = source["errorCode"];
	        this.errorReason = source["errorReason"];
	        this.errorAlias = source["errorAlias"];
	        this.collisions = source["collisions"];
	    }
	}
	export class SuggestionDestinationInfo {
	    location: string;
	    path: string;
	    mode: string;
	    mapped: boolean;
	    targetPath: string;
	    targetExists: boolean;
	    detectedPath: string;
	    applied: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SuggestionDestinationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.location = source["location"];
	        this.path = source["path"];
	        this.mode = source["mode"];
	        this.mapped = source["mapped"];
	        this.targetPath = source["targetPath"];
	        this.targetExists = source["targetExists"];
	        this.detectedPath = source["detectedPath"];
	        this.applied = source["applied"];
	    }
	}
	export class SuggestionInfo {
	    id: string;
	    name: string;
	    description: string;
	    source: string;
	    include: string[];
	    kind: string;
	    sourceExists: boolean;
	    destinations: SuggestionDestinationInfo[];
	    applied: boolean;
	    valid: boolean;
	    invalidReason: string;
	
	    static createFrom(source: any = {}) {
	        return new SuggestionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.source = source["source"];
	        this.include = source["include"];
	        this.kind = source["kind"];
	        this.sourceExists = source["sourceExists"];
	        this.destinations = this.convertValues(source["destinations"], SuggestionDestinationInfo);
	        this.applied = source["applied"];
	        this.valid = source["valid"];
	        this.invalidReason = source["invalidReason"];
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
	export class SuggestionsApplyOutcome {
	    results: SuggestionApplyResult[];
	    applied: number;
	
	    static createFrom(source: any = {}) {
	        return new SuggestionsApplyOutcome(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], SuggestionApplyResult);
	        this.applied = source["applied"];
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

export namespace updater {
	
	export class Release {
	    version: string;
	    tag: string;
	    name: string;
	    notes: string;
	    pageUrl: string;
	    publishedAt: string;
	    prerelease: boolean;
	    installerUrl: string;
	    checksumsUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.tag = source["tag"];
	        this.name = source["name"];
	        this.notes = source["notes"];
	        this.pageUrl = source["pageUrl"];
	        this.publishedAt = source["publishedAt"];
	        this.prerelease = source["prerelease"];
	        this.installerUrl = source["installerUrl"];
	        this.checksumsUrl = source["checksumsUrl"];
	    }
	}

}

