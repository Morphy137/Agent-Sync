import * as AppBindings from "../../../wailsjs/go/main/App";
import type {
  config,
  service,
  orchestrator,
  backup,
  profiles,
  rules,
  rulesync,
  recognize,
  subscriptions,
  gitops,
} from "../../../wailsjs/go/models";

export type RuleScope = "repo" | "local";
export type RepoDiffMode = "working" | "incoming";

export interface ParsedAppError {
  code: string;
  params?: Record<string, string>;
  message: string;
}

export function parseError(err: unknown): ParsedAppError {
  if (typeof err === "string") {
    const match = err.match(/^\[([a-z0-9_]+)\](?:\s*(.*))?$/s);
    if (match) {
      const code = match[1];
      const details = match[2]?.trim();
      let params: Record<string, string> | undefined;
      if (details && details.startsWith("{") && details.endsWith("}")) {
        try {
          params = JSON.parse(details);
        } catch {
          // not json
        }
      }
      return { code, params, message: details || code };
    }
    return { code: "unknown", message: err };
  }
  if (err instanceof Error) {
    return parseError(err.message);
  }
  return { code: "unknown", message: String(err) };
}

export const api = {
  getConfig: (): Promise<config.Config> => AppBindings.GetConfig(),
  updateSettings: (payload: service.SettingsPayload): Promise<config.Config> =>
    AppBindings.UpdateSettings(payload),
  selectRootFolder: (title: string): Promise<string> => AppBindings.SelectRootFolder(title),
  setRootPath: (rootPath: string): Promise<config.Config> => AppBindings.SetRootPath(rootPath),
  listProfiles: (): Promise<profiles.Profile[]> => AppBindings.ListProfiles(),
  selectTargetFolder: (title: string): Promise<string> => AppBindings.SelectTargetFolder(title),
  connectProfile: (profile: string, targetPath: string): Promise<service.InitialLinkResult> =>
    AppBindings.ConnectProfile(profile, targetPath),
  changeTarget: (connectionId: string, targetPath: string): Promise<service.InitialLinkResult> =>
    AppBindings.ChangeTarget(connectionId, targetPath),
  resolveInitialLink: (pendingId: string, choice: string): Promise<void> =>
    AppBindings.ResolveInitialLink(pendingId, choice),
  disconnectProfile: (connectionId: string): Promise<config.Config> =>
    AppBindings.DisconnectProfile(connectionId),
  getStatus: (): Promise<orchestrator.SyncPlan> => AppBindings.GetStatus(),
  checkRemote: (): Promise<orchestrator.SyncPlan> => AppBindings.CheckRemote(),
  applyConnection: (connectionId: string): Promise<void> =>
    AppBindings.ApplyConnection(connectionId),
  planSync: (): Promise<orchestrator.SyncPlan> => AppBindings.PlanSync(),
  executeSync: (
    fingerprint: string,
    resolutions: orchestrator.ConflictResolution[]
  ): Promise<orchestrator.SyncResult> => AppBindings.ExecuteSync(fingerprint, resolutions),
  listBackups: (connectionId: string): Promise<backup.BackupInfo[]> =>
    AppBindings.ListBackups(connectionId),
  restoreBackup: (connectionId: string, backupId: string): Promise<void> =>
    AppBindings.RestoreBackup(connectionId, backupId),
  openInExplorer: (path: string): Promise<void> => AppBindings.OpenInExplorer(path),
  initRepository: (): Promise<void> => AppBindings.InitRepository(),
  addRemote: (url: string): Promise<void> => AppBindings.AddRemote(url),
  setGitIdentity: (name: string, email: string): Promise<void> =>
    AppBindings.SetGitIdentity(name, email),
  getGitInfo: (): Promise<service.GitInfoPayload> => AppBindings.GetGitInfo(),
  selectFolder: (title: string, defaultDir = ""): Promise<string> =>
    AppBindings.SelectFolder(title, defaultDir),
  selectFile: (title: string, defaultDir = ""): Promise<string> =>
    AppBindings.SelectFile(title, defaultDir),
  openPath: (path: string): Promise<void> => AppBindings.OpenPath(path),

  // Sync rules
  getRulesReport: (): Promise<rulesync.Report> => AppBindings.GetRulesReport(),
  saveRule: (rule: rules.Rule, scope: RuleScope): Promise<service.SaveRuleResult> =>
    AppBindings.SaveRule(rule, scope),
  deleteRule: (id: string): Promise<void> => AppBindings.DeleteRule(id),
  syncRules: (): Promise<rulesync.ApplyResult> => AppBindings.SyncRules(),
  resolveRuleFile: (resolution: rulesync.Resolution): Promise<void> =>
    AppBindings.ResolveRuleFile(resolution),
  getRuleDiff: (ruleId: string, destId: string, relPath: string): Promise<rulesync.Diff> =>
    AppBindings.GetRuleDiff(ruleId, destId, relPath),
  listRepoTree: (): Promise<service.SourceTreeEntry[]> => AppBindings.ListRepoTree(),
  importIntoRepo: (sourcePath: string, repoRelDir: string): Promise<string> =>
    AppBindings.ImportIntoRepo(sourcePath, repoRelDir),
  detectSource: (source: rules.Source): Promise<recognize.SourceInfo> =>
    AppBindings.DetectSource(source),
  suggestPlacement: (source: rules.Source, alias: string): Promise<recognize.Suggestion> =>
    AppBindings.SuggestPlacement(source, alias),
  getRecognizersPath: (): Promise<string> => AppBindings.GetRecognizersPath(),

  // Locations
  listLocations: (): Promise<service.LocationInfo[]> => AppBindings.ListLocations(),
  saveLocation: (alias: string, path: string): Promise<config.Config> =>
    AppBindings.SaveLocation(alias, path),
  removeLocation: (alias: string): Promise<config.Config> => AppBindings.RemoveLocation(alias),

  // Subscriptions
  listSubscriptions: (): Promise<service.SubscriptionInfo[]> => AppBindings.ListSubscriptions(),
  addSubscription: (url: string, name: string): Promise<config.Subscription> =>
    AppBindings.AddSubscription(url, name),
  removeSubscription: (id: string): Promise<void> => AppBindings.RemoveSubscription(id),
  checkSubscription: (id: string): Promise<subscriptions.UpdateInfo> =>
    AppBindings.CheckSubscription(id),
  updateSubscription: (id: string): Promise<service.SubscriptionUpdateResult> =>
    AppBindings.UpdateSubscription(id),
  setSubscriptionAutoUpdate: (id: string, enabled: boolean): Promise<config.Config> =>
    AppBindings.SetSubscriptionAutoUpdate(id, enabled),
  getSubscriptionTree: (id: string): Promise<service.SourceTreeEntry[]> =>
    AppBindings.GetSubscriptionTree(id),
  getSubscriptionIncomingDiff: (id: string, relPath: string): Promise<rulesync.Diff> =>
    AppBindings.GetSubscriptionIncomingDiff(id, relPath),

  // Repository remote
  connectRemote: (url: string): Promise<gitops.ConnectRemoteResult> =>
    AppBindings.ConnectRemote(url),
  getRepoFileDiff: (relPath: string, mode: RepoDiffMode): Promise<rulesync.Diff> =>
    AppBindings.GetRepoFileDiff(relPath, mode),
  discardRepoFile: (relPath: string): Promise<void> => AppBindings.DiscardRepoFile(relPath),
};
