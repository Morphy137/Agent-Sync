import * as AppBindings from "../../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import type {
  autostart,
  main,
  config,
  service,
  orchestrator,
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
  getStatus: (): Promise<orchestrator.SyncPlan> => AppBindings.GetStatus(),
  checkRemote: (): Promise<orchestrator.SyncPlan> => AppBindings.CheckRemote(),
  planSync: (): Promise<orchestrator.SyncPlan> => AppBindings.PlanSync(),
  executeSync: (
    fingerprint: string,
    resolutions: orchestrator.ConflictResolution[]
  ): Promise<orchestrator.SyncResult> => AppBindings.ExecuteSync(fingerprint, resolutions),
  openInExplorer: (path: string): Promise<void> => AppBindings.OpenInExplorer(path),
  initRepository: (): Promise<void> => AppBindings.InitRepository(),
  addRemote: (url: string): Promise<void> => AppBindings.AddRemote(url),
  setGitIdentity: (name: string, email: string): Promise<void> =>
    AppBindings.SetGitIdentity(name, email),
  getGitInfo: (): Promise<service.GitInfoPayload> => AppBindings.GetGitInfo(),
  getStartupStatus: (): Promise<autostart.Status> => AppBindings.GetStartupStatus(),
  selectFolder: (title: string, defaultDir = ""): Promise<string> =>
    AppBindings.SelectFolder(title, defaultDir),
  selectFiles: (title: string, defaultDir = ""): Promise<string[]> =>
    AppBindings.SelectFiles(title, defaultDir),
  selectFile: (title: string, defaultDir = ""): Promise<string> =>
    AppBindings.SelectFile(title, defaultDir),
  openPath: (path: string): Promise<void> => AppBindings.OpenPath(path),

  // Backups
  listBackups: (): Promise<service.BackupEntry[]> => AppBindings.ListBackups(),
  restoreBackup: (id: string, files: string[]): Promise<service.RestoreBackupResult> =>
    AppBindings.RestoreBackup(id, files),

  // Sync rules
  listFileRoots: () => AppBindings.ListFileRoots(),
  listFileDirectory: (rootId: string, path: string) => AppBindings.ListFileDirectory(rootId, path),
  readBrowserFile: (rootId: string, path: string) => AppBindings.ReadBrowserFile(rootId, path),
  saveBrowserFile: (
    rootId: string,
    path: string,
    content: string,
    hash: string,
    absolutePath: string
  ) => AppBindings.SaveBrowserFile(rootId, path, content, hash, absolutePath),
  listRuleSourceFiles: (id: string) => AppBindings.ListRuleSourceFiles(id),
  readRuleSourceFile: (id: string, path: string) => AppBindings.ReadRuleSourceFile(id, path),
  saveRuleSourceFile: (id: string, path: string, content: string, hash: string) =>
    AppBindings.SaveRuleSourceFile(id, path, content, hash),
  getRulesReport: (): Promise<rulesync.Report> => AppBindings.GetRulesReport(),
  saveRule: (rule: rules.Rule, scope: RuleScope): Promise<service.SaveRuleResult> =>
    AppBindings.SaveRule(rule, scope),
  deleteRule: (id: string): Promise<void> => AppBindings.DeleteRule(id),
  getRuleCategories: (): Promise<rules.Category[]> => AppBindings.GetRuleCategories(),
  saveRuleCategory: (category: rules.Category, scope: RuleScope): Promise<rules.Category> =>
    AppBindings.SaveRuleCategory(category, scope),
  deleteRuleCategory: (id: string, scope: RuleScope): Promise<void> =>
    AppBindings.DeleteRuleCategory(id, scope),
  migrateRuleCategories: (): Promise<void> => AppBindings.MigrateRuleCategories(),
  syncRules: (): Promise<rulesync.ApplyResult> => AppBindings.SyncRules(),
  resolveRuleFile: (resolution: rulesync.Resolution): Promise<void> =>
    AppBindings.ResolveRuleFile(resolution),
  getRuleDiff: (ruleId: string, destId: string, relPath: string): Promise<rulesync.Diff> =>
    AppBindings.GetRuleDiff(ruleId, destId, relPath),
  listRepoTree: (): Promise<service.SourceTreeEntry[]> => AppBindings.ListRepoTree(),
  getChangelog: (): Promise<string> => AppBindings.GetChangelog(),
  getAppInfo: (): Promise<main.AppInfo> => AppBindings.GetAppInfo(),
  checkForUpdate: (): Promise<main.UpdateStatus> => AppBindings.CheckForUpdate(),
  installUpdate: (): Promise<void> => AppBindings.InstallUpdate(),
  openURL: (url: string): void => BrowserOpenURL(url),
  setTutorialCompleted: (done: boolean): Promise<config.Config> =>
    AppBindings.SetTutorialCompleted(done),
  getGitignore: (): Promise<service.GitignorePayload> => AppBindings.GetGitignore(),
  saveGitignore: (content: string): Promise<service.GitignorePayload> =>
    AppBindings.SaveGitignore(content),
  addRecommendedGitignore: (): Promise<service.GitignorePayload> =>
    AppBindings.AddRecommendedGitignore(),
  excludeRepoPath: (path: string): Promise<service.GitignorePayload> =>
    AppBindings.ExcludeRepoPath(path),
  importIntoRepo: (sourcePath: string, repoRelDir: string): Promise<string> =>
    AppBindings.ImportIntoRepo(sourcePath, repoRelDir),
  detectSource: (source: rules.Source): Promise<recognize.SourceInfo> =>
    AppBindings.DetectSource(source),
  suggestRuleCategory: (source: rules.Source, kind: string): Promise<string> =>
    AppBindings.SuggestRuleCategory(source, kind),
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
  listSubscriptionSuggestions: (id: string): Promise<service.SuggestionInfo[]> =>
    AppBindings.ListSubscriptionSuggestions(id),
  applySubscriptionSuggestions: (
    id: string,
    suggestionIds: string[],
    locationMappings: Record<string, string>
  ): Promise<service.SuggestionsApplyOutcome> =>
    AppBindings.ApplySubscriptionSuggestions(id, suggestionIds, locationMappings),

  // Repository remote
  connectRemote: (url: string): Promise<gitops.ConnectRemoteResult> =>
    AppBindings.ConnectRemote(url),
  getRepoFileDiff: (relPath: string, mode: RepoDiffMode): Promise<rulesync.Diff> =>
    AppBindings.GetRepoFileDiff(relPath, mode),
  discardRepoFile: (relPath: string): Promise<void> => AppBindings.DiscardRepoFile(relPath),
};
