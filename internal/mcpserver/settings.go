package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/service"
)

type CreateRootInput struct {
	Parent string `json:"parent" jsonschema:"existing parent directory"`
	Name   string `json:"name" jsonschema:"new folder name"`
}
type CloneRepositoryInput struct {
	URL    string `json:"url" jsonschema:"git repository URL"`
	Parent string `json:"parent" jsonschema:"existing parent directory"`
	Name   string `json:"name" jsonschema:"new clone folder name"`
}
type GitIdentityInput struct {
	Name  string `json:"name" jsonschema:"repository-local Git user name"`
	Email string `json:"email" jsonschema:"repository-local Git user email"`
}
type AddRemoteInput struct {
	URL     string `json:"url" jsonschema:"origin URL; this operation also pushes the current branch"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true after asking the user to approve uploading the repository"`
}
type RecognizersPathOutput struct {
	Path string `json:"path"`
}

func (s *Server) registerSettingsTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "get_settings", Description: "Read operational preferences. Desktop-only tray, startup and theme controls are excluded.", Annotations: readOnly}, handler(func(NoInput) (service.OperationalSettings, error) { return a.GetSettings() }))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "update_settings", Description: "Update only the supplied operational preferences. Omitted values are preserved. Periodic checks and automatic application require the desktop app to be running; this server does not start a scheduler.", Annotations: idempotent}, handler(func(in service.OperationalSettingsPatch) (service.OperationalSettings, error) {
		return a.UpdateSettings(in)
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "create_root", Description: "Create a directory and select it as the repository root. Does not initialize Git or upload files.", Annotations: additive}, handler(func(in CreateRootInput) (config.Config, error) { return deref(a.CreateRoot(in.Parent, in.Name)) }))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "clone_repository", Description: "Clone a remote repository into a new folder and select it as root.", Annotations: network}, handler(func(in CloneRepositoryInput) (config.Config, error) {
		return deref(a.CloneRepository(in.URL, in.Parent, in.Name))
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "init_repository", Description: "Initialize Git in the selected root. Does not push files.", Annotations: idempotent}, handler(func(NoInput) (Done, error) { err := a.InitRepository(); return Done{OK: err == nil}, err }))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "get_git_info", Description: "Read root repository, remote, branch and repository-local Git identity.", Annotations: readOnly}, handler(func(NoInput) (service.GitInfoPayload, error) { return deref(a.GitInfo()) }))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "set_git_identity", Description: "Set Git user name and email for the selected repository only.", Annotations: idempotent}, handler(func(in GitIdentityInput) (Done, error) {
		err := a.SetGitIdentity(in.Name, in.Email)
		return Done{OK: err == nil}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "add_remote", Description: "Add origin and push the current branch, setting its upstream. Ask the user before uploading; requires confirm=true. Unlike connect_remote, this operation pushes.", Annotations: network}, handler(func(in AddRemoteInput) (Done, error) {
		if !in.Confirm {
			return Done{}, apperr.New(apperr.ConfirmationRequired)
		}
		err := a.AddRemote(in.URL)
		return Done{OK: err == nil}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "get_recognizers_path", Description: "Initialize the custom recognizers file if missing and return its path. Does not open an editor.", Annotations: idempotent}, handler(func(NoInput) (RecognizersPathOutput, error) {
		path, err := a.RecognizersPath()
		return RecognizersPathOutput{Path: path}, err
	}))
}
