import * as AppBindings from "../../../wailsjs/go/main/App";
import type { config, service, orchestrator, backup, profiles } from "../../../wailsjs/go/models";

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
};
