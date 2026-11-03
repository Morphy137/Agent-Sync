package main

import "agentsync/internal/service"

// ListBackups returns every backup (rule files, rule sources, discarded repository edits), newest first.
func (a *App) ListBackups() ([]service.BackupEntry, error) { return a.svc.ListBackups() }

// RestoreBackup puts the files of a backup (all when files is empty) back, backing up the current ones first.
func (a *App) RestoreBackup(id string, files []string) (*service.RestoreBackupResult, error) {
	return a.svc.RestoreBackup(id, files)
}
