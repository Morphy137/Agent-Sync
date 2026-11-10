package cli

import (
	"io"
	"os"
	"strings"
)

func init() {
	for lang, values := range map[string]map[string]string{
		LangEN: {
			"cmd.repo_import":        "Copy a local file or folder into the repository without overwriting existing files.",
			"cmd.subscriptions_diff": "Compare a subscription file with the last fetched remote version.",
			"cmd.rules_files":        "List the source files selected by a rule.",
			"cmd.rules_read_file":    "Read a selected rule source file and its hash.",
			"cmd.rules_save_file":    "Save a selected rule source file if its hash still matches.",
			"cmd.files_roots":        "List configured file browser roots.",
			"cmd.files_list":         "List a directory in a configured root.",
			"cmd.files_read":         "Read a text file with its hash and resolved path.",
			"cmd.files_save":         "Save a text file only if its hash and resolved path still match.",
			"err.files_required":     "required flag: --%s",
		},
		LangES: {
			"cmd.repo_import":        "Copiar un archivo o carpeta local al repositorio sin sobrescribir archivos existentes.",
			"cmd.subscriptions_diff": "Comparar un archivo de suscripción con la última versión remota obtenida.",
			"cmd.rules_files":        "Listar los archivos de origen seleccionados por una regla.",
			"cmd.rules_read_file":    "Leer un archivo de origen seleccionado y su hash.",
			"cmd.rules_save_file":    "Guardar un archivo de origen seleccionado si su hash coincide.",
			"cmd.files_roots":        "Listar las raíces configuradas del explorador de archivos.",
			"cmd.files_list":         "Listar un directorio de una raíz configurada.",
			"cmd.files_read":         "Leer un archivo de texto con su hash y ruta resuelta.",
			"cmd.files_save":         "Guardar un archivo de texto solo si su hash y ruta resuelta coinciden.",
			"err.files_required":     "opción obligatoria: --%s",
		},
	} {
		for key, value := range values {
			messages[lang][key] = value
		}
	}
}

// fileCommands is wired into the main command table by the caller.
func fileCommands() []command {
	return []command{
		{path: "repo import", usage: "<source> [--dir DIR]", desc: "cmd.repo_import", run: cmdImportFile},
		{path: "subscriptions diff", usage: "<id> <path>", desc: "cmd.subscriptions_diff", run: cmdSubscriptionFileDiff},
		{path: "rules files", usage: "<ruleId>", desc: "cmd.rules_files", run: cmdRuleSourceFile("files")},
		{path: "rules read-file", usage: "<ruleId> <path>", desc: "cmd.rules_read_file", run: cmdRuleSourceFile("read-file")},
		{path: "rules save-file", usage: "<ruleId> <path> --content-file FILE|- --expected-hash H", desc: "cmd.rules_save_file", run: cmdRuleSourceFile("save-file")},
		{path: "files roots", desc: "cmd.files_roots", run: cmdFileRoots},
		{path: "files list", usage: "--root ID [--path P]", desc: "cmd.files_list", run: cmdBrowserFile("list")},
		{path: "files read", usage: "--root ID --path P", desc: "cmd.files_read", run: cmdBrowserFile("read")},
		{path: "files save", usage: "--root ID --path P --content-file FILE|- --expected-hash H --expected-path P", desc: "cmd.files_save", run: cmdBrowserFile("save")},
	}
}

func cmdImportFile(e *env, args []string) (any, error) {
	fs := newFlags("repo import")
	dir := fs.String("dir", "", "")
	pos, err := expectArgs(fs, args, "source")
	if err != nil {
		return nil, err
	}
	return e.API().ImportIntoRepo(pos[0], *dir)
}

func cmdSubscriptionFileDiff(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("subscriptions diff"), args, "id", "path")
	if err != nil {
		return nil, err
	}
	return e.API().SubscriptionFileDiff(pos[0], pos[1])
}

func cmdRuleSourceFile(action string) func(*env, []string) (any, error) {
	return func(e *env, args []string) (any, error) {
		fs := newFlags("rules " + action)
		var contentFile, hash string
		if action == "save-file" {
			fs.StringVar(&contentFile, "content-file", "", "")
			fs.StringVar(&hash, "expected-hash", "", "")
		}
		names := []string{"ruleId"}
		if action != "files" {
			names = append(names, "path")
		}
		pos, err := expectArgs(fs, args, names...)
		if err != nil {
			return nil, err
		}
		switch action {
		case "files":
			return e.API().ListRuleSourceFiles(pos[0])
		case "read-file":
			return e.API().ReadRuleSourceFile(pos[0], pos[1])
		default:
			if err := requireFileFlag(e, "content-file", contentFile); err != nil {
				return nil, err
			}
			if err := requireFileFlag(e, "expected-hash", hash); err != nil {
				return nil, err
			}
			content, err := readFileContent(contentFile)
			if err != nil {
				return nil, err
			}
			return e.API().SaveRuleSourceFile(pos[0], pos[1], content, hash)
		}
	}
}

func cmdFileRoots(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("files roots"), args); err != nil {
		return nil, err
	}
	return e.API().ListFileRoots()
}

func requireFileFlag(e *env, name, value string) error {
	if strings.TrimSpace(value) == "" {
		return usagef(tr(e.lang, "err.files_required"), name)
	}
	return nil
}

func readFileContent(path string) (string, error) {
	var data []byte
	var err error
	if path == stdinPath {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	return string(data), err
}

func cmdBrowserFile(action string) func(*env, []string) (any, error) {
	return func(e *env, args []string) (any, error) {
		fs := newFlags("files " + action)
		root := fs.String("root", "", "")
		path := fs.String("path", "", "")
		var contentFile, hash, expectedPath string
		if action == "save" {
			fs.StringVar(&contentFile, "content-file", "", "")
			fs.StringVar(&hash, "expected-hash", "", "")
			fs.StringVar(&expectedPath, "expected-path", "", "")
		}
		if _, err := expectArgs(fs, args); err != nil {
			return nil, err
		}
		required := []struct{ name, value string }{{"root", *root}}
		if action != "list" {
			required = append(required, struct{ name, value string }{"path", *path})
		}
		if action == "save" {
			required = append(required, []struct{ name, value string }{{"content-file", contentFile}, {"expected-hash", hash}, {"expected-path", expectedPath}}...)
		}
		for _, flag := range required {
			if err := requireFileFlag(e, flag.name, flag.value); err != nil {
				return nil, err
			}
		}
		switch action {
		case "list":
			return e.API().ListFileDirectory(*root, *path)
		case "read":
			return e.API().ReadBrowserFile(*root, *path)
		default:
			content, err := readFileContent(contentFile)
			if err != nil {
				return nil, err
			}
			return e.API().SaveBrowserFile(*root, *path, content, hash, expectedPath)
		}
	}
}
