package ops

import (
	"agentsync/internal/config"
	"agentsync/internal/service"
)

func (a *API) GetSettings() (service.OperationalSettings, error) {
	return a.svc.GetOperationalSettings()
}
func (a *API) UpdateSettings(in service.OperationalSettingsPatch) (service.OperationalSettings, error) {
	return a.svc.UpdateOperationalSettings(in)
}
func (a *API) CreateRoot(parent, name string) (*config.Config, error) {
	return a.svc.CreateRoot(parent, name)
}
func (a *API) CloneRepository(url, parent, name string) (*config.Config, error) {
	return a.svc.CloneRepository(url, parent, name)
}
func (a *API) InitRepository() error                     { return a.svc.InitRepository() }
func (a *API) SetGitIdentity(name, email string) error   { return a.svc.SetGitIdentity(name, email) }
func (a *API) GitInfo() (*service.GitInfoPayload, error) { return a.svc.GetGitInfo() }

// AddRemote sets the remote and pushes the current branch; callers must obtain confirmation.
func (a *API) AddRemote(url string) error       { return a.svc.AddRemote(url) }
func (a *API) RecognizersPath() (string, error) { return a.svc.GetRecognizersPath() }
