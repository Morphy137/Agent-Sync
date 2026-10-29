import * as AppBindings from "../../../wailsjs/go/main/App";
import type { config, main, orchestrator, backup, profiles } from "../../../wailsjs/go/models";

export interface ParsedAppError {
  code: string;
  params?: Record<string, string>;
  message: string;
}

export function parseError(err: unknown): ParsedAppError {
  if (typeof err === "string") {
    // Check if error format is "[code] ..." or "[code]"
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
  updateSettings: (payload: main.SettingsPayload): Promise<config.Config> =>
    AppBindings.UpdateSettings(payload),
  selectRootFolder: (): Promise<string> => AppBindings.SelectRootFolder(),
  setRootPath: (rootPath: string): Promise<config.Config> => AppBindings.SetRootPath(rootPath),
  listProfiles: (): Promise<profiles.Profile[]> => AppBindings.ListProfiles(),
  selectTargetFolder: (profile: string): Promise<string> => AppBindings.SelectTargetFolder(profile),
  connectProfile: (profile: string, targetPath: string): Promise<main.InitialLinkResult> =>
    AppBindings.ConnectProfile(profile, targetPath),
  resolveInitialLink: (conn: config.Connection, choice: string): Promise<void> =>
    AppBindings.ResolveInitialLink(conn, choice),
  disconnectProfile: (connectionId: string): Promise<config.Config> =>
    AppBindings.DisconnectProfile(connectionId),
  getStatus: (): Promise<orchestrator.SyncPlan> => AppBindings.GetStatus(),
  applyConnection: (connectionId: string): Promise<void> =>
    AppBindings.ApplyConnection(connectionId),
  planSync: (): Promise<orchestrator.SyncPlan> => AppBindings.PlanSync(),
  executeSync: (
    resolutions: orchestrator.ConflictResolution[]
  ): Promise<orchestrator.SyncResult> => AppBindings.ExecuteSync(resolutions),
  listBackups: (connectionId: string): Promise<backup.BackupInfo[]> =>
    AppBindings.ListBackups(connectionId),
  restoreBackup: (connectionId: string, backupId: string): Promise<void> =>
    AppBindings.RestoreBackup(connectionId, backupId),
  openInExplorer: (path: string): Promise<void> => AppBindings.OpenInExplorer(path),
  initRepository: (): Promise<void> => AppBindings.InitRepository(),
  addRemote: (url: string): Promise<void> => AppBindings.AddRemote(url),
  getGitInfo: (): Promise<main.GitInfoPayload> => AppBindings.GetGitInfo(),
};
