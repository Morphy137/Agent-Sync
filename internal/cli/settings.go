package cli

import (
	"agentsync/internal/service"
)

func init() {
	for lang, values := range map[string]map[string]string{
		"en": {"cmd.settings_show": "Show operational settings (desktop-only preferences are excluded).", "cmd.settings_update": "Update only the settings present in the JSON input.", "cmd.root_create": "Create a repository root folder and select it.", "cmd.root_clone": "Clone a repository and select it as root.", "cmd.repo_init": "Initialize Git in the root folder.", "cmd.repo_identity": "Set repository-local Git identity.", "cmd.repo_info": "Show repository and Git identity information.", "cmd.repo_add_remote": "Add origin and push the current branch (needs --yes).", "cmd.recognizers_path": "Initialize and show the custom recognizers file path."},
		"es": {"cmd.settings_show": "Mostrar preferencias operativas (sin opciones exclusivas del escritorio).", "cmd.settings_update": "Actualizar solo las preferencias presentes en el JSON.", "cmd.root_create": "Crear una carpeta raíz y seleccionarla.", "cmd.root_clone": "Clonar un repositorio y seleccionarlo como raíz.", "cmd.repo_init": "Inicializar Git en la carpeta raíz.", "cmd.repo_identity": "Configurar la identidad Git local del repositorio.", "cmd.repo_info": "Mostrar información del repositorio y su identidad Git.", "cmd.repo_add_remote": "Añadir origin y subir la rama actual (requiere --yes).", "cmd.recognizers_path": "Inicializar y mostrar la ruta de los reconocedores personalizados."},
	} {
		for key, value := range values {
			messages[lang][key] = value
		}
	}
}

func settingsCommands() []command {
	return []command{
		{path: "settings show", desc: "cmd.settings_show", run: cmdSettingsShow},
		{path: "settings update", usage: "--input FILE|-", desc: "cmd.settings_update", run: cmdSettingsUpdate},
		{path: "root create", usage: "<parent> <name>", desc: "cmd.root_create", run: cmdRootCreate},
		{path: "root clone", usage: "<url> <parent> <name>", desc: "cmd.root_clone", run: cmdRootClone},
		{path: "repo init", desc: "cmd.repo_init", run: cmdRepoInit},
		{path: "repo identity", usage: "--name N --email E", desc: "cmd.repo_identity", run: cmdRepoIdentity},
		{path: "repo info", desc: "cmd.repo_info", run: cmdRepoInfo},
		{path: "repo add-remote", usage: "<url> --yes", desc: "cmd.repo_add_remote", run: cmdRepoAddRemote},
		{path: "recognizers path", desc: "cmd.recognizers_path", run: cmdRecognizersPath},
	}
}

func cmdSettingsShow(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("settings show"), args); err != nil {
		return nil, err
	}
	return e.API().GetSettings()
}
func cmdSettingsUpdate(e *env, args []string) (any, error) {
	fs := newFlags("settings update")
	input := fs.String("input", "", "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}
	if *input == "" {
		return nil, usagef("--input")
	}
	var in service.OperationalSettingsPatch
	if err := readInput(*input, stdin, &in); err != nil {
		return nil, err
	}
	return e.API().UpdateSettings(in)
}
func cmdRootCreate(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("root create"), args, "<parent>", "<name>")
	if err != nil {
		return nil, err
	}
	return e.API().CreateRoot(pos[0], pos[1])
}
func cmdRootClone(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("root clone"), args, "<url>", "<parent>", "<name>")
	if err != nil {
		return nil, err
	}
	return e.API().CloneRepository(pos[0], pos[1], pos[2])
}
func cmdRepoInit(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("repo init"), args); err != nil {
		return nil, err
	}
	if err := e.API().InitRepository(); err != nil {
		return nil, err
	}
	return e.API().GitInfo()
}
func cmdRepoIdentity(e *env, args []string) (any, error) {
	fs := newFlags("repo identity")
	name, email := fs.String("name", "", ""), fs.String("email", "", "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}
	if *name == "" || *email == "" {
		return nil, usagef("--name, --email")
	}
	if err := e.API().SetGitIdentity(*name, *email); err != nil {
		return nil, err
	}
	return e.API().GitInfo()
}
func cmdRepoInfo(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("repo info"), args); err != nil {
		return nil, err
	}
	return e.API().GitInfo()
}
func cmdRepoAddRemote(e *env, args []string) (any, error) {
	fs := newFlags("repo add-remote")
	yes := fs.Bool("yes", false, "")
	pos, err := expectArgs(fs, args, "<url>")
	if err != nil {
		return nil, err
	}
	if !*yes {
		return nil, usagef("--yes")
	}
	if err := e.API().AddRemote(pos[0]); err != nil {
		return nil, err
	}
	return e.API().GitInfo()
}
func cmdRecognizersPath(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("recognizers path"), args); err != nil {
		return nil, err
	}
	path, err := e.API().RecognizersPath()
	return map[string]string{"path": path}, err
}
