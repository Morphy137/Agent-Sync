package recognize

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/rules"
)

// Size limits for content probes; larger files never match content-based conditions.
const (
	maxJSONProbeBytes        = 8 << 20
	maxFrontmatterProbeBytes = 1 << 20
)

// Folder detection scoring.
const (
	folderNameScore = 2
	conditionScore  = 1
)

const (
	frontmatterDelimiter    = "---"
	frontmatterEndAlternate = "..."
	utf8BOM                 = "\xef\xbb\xbf"
)

// SourceInfo describes what a source path is.
type SourceInfo struct {
	Kind   string `json:"kind"`   // rules.Kind* (never "auto")
	KindID string `json:"kindId"` // matching sourceKinds id, "" for plain file/folder
	Name   string `json:"name"`   // value used for {name}
	IsDir  bool   `json:"isDir"`
	// BaseName is the base name of the source path including any extension. Used by the
	// generic fallback suggestion (Name drops the extension of files).
	BaseName string `json:"baseName"`
}

// FolderProfile is the best profile match for a folder.
type FolderProfile struct {
	ProfileID string `json:"profileId"`
	Label     string `json:"label"`
	Score     int    `json:"score"`
}

// DetectSource classifies absPath using the enabled source kinds in order. The first match wins;
// otherwise the source is reported as a plain file or folder.
func (s *Set) DetectSource(absPath string) (SourceInfo, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return SourceInfo{}, apperr.Wrap(apperr.SourceNotFound, err).WithParam(paramPath, absPath)
	}

	baseName := filepath.Base(absPath)
	result := SourceInfo{IsDir: info.IsDir(), BaseName: baseName, Name: defaultName(baseName, info.IsDir())}

	for _, kind := range s.SourceKinds {
		if kind.Disabled {
			continue
		}
		frontmatter, ok := matchSource(kind.Match, absPath, info)
		if !ok {
			continue
		}
		result.Kind = kind.Kind
		result.KindID = kind.ID
		if name := frontmatter["name"]; isPlainName(name) {
			result.Name = name
		}
		return result, nil
	}

	result.Kind = rules.KindFile
	if result.IsDir {
		result.Kind = rules.KindFolder
	}
	return result, nil
}

// DetectFolder returns the highest scoring enabled profile for absFolder. Ties keep the
// profile listed first. ok is false when nothing matched or the folder does not exist.
func (s *Set) DetectFolder(absFolder string) (FolderProfile, bool) {
	info, err := os.Stat(absFolder)
	if err != nil || !info.IsDir() {
		return FolderProfile{}, false
	}

	baseName := filepath.Base(absFolder)
	var best FolderProfile
	for _, profile := range s.Profiles {
		if profile.Disabled {
			continue
		}
		score := scoreProfile(profile.Detect, absFolder, baseName)
		if score > best.Score {
			best = FolderProfile{ProfileID: profile.ID, Label: profile.Label, Score: score}
		}
	}
	return best, best.Score > 0
}

func scoreProfile(detect Detect, absFolder, baseName string) int {
	score := 0
	if containsFold(detect.FolderNames, baseName) {
		score += folderNameScore
	}
	for _, cond := range detect.AnyOf {
		if conditionHolds(cond, absFolder) {
			score += conditionScore
		}
	}
	return score
}

func conditionHolds(cond DetectCondition, absFolder string) bool {
	if cond.File != "" {
		return isRegularFile(filepath.Join(absFolder, cond.File))
	}
	info, err := os.Stat(filepath.Join(absFolder, cond.Dir))
	return err == nil && info.IsDir()
}

// matchSource reports whether every present condition of match holds for absPath. It also
// returns the parsed frontmatter of ContainsFile when it was read.
func matchSource(match Match, absPath string, info fs.FileInfo) (map[string]string, bool) {
	wantDir := match.Type == MatchTypeDir
	if info.IsDir() != wantDir {
		return nil, false
	}

	baseName := filepath.Base(absPath)
	if len(match.FileNames) > 0 && !containsFold(match.FileNames, baseName) {
		return nil, false
	}
	if len(match.Extensions) > 0 && !containsFold(match.Extensions, filepath.Ext(baseName)) {
		return nil, false
	}
	if match.JSONKey != "" && !jsonHasTopLevelKey(absPath, match.JSONKey) {
		return nil, false
	}
	if match.ContainsFile == "" {
		return nil, true
	}

	innerPath := filepath.Join(absPath, match.ContainsFile)
	if !isRegularFile(innerPath) {
		return nil, false
	}
	if len(match.FrontmatterKeys) == 0 {
		return nil, true
	}
	frontmatter, err := readFrontmatter(innerPath)
	if err != nil {
		return nil, false
	}
	for _, key := range match.FrontmatterKeys {
		if _, present := frontmatter[key]; !present {
			return nil, false
		}
	}
	return frontmatter, true
}

func jsonHasTopLevelKey(absPath, key string) bool {
	data, err := readLimited(absPath, maxJSONProbeBytes)
	if err != nil {
		return false
	}
	data = bytes.TrimPrefix(data, []byte(utf8BOM))
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return false
	}
	_, ok := top[key]
	return ok
}

// readFrontmatter parses the simple YAML frontmatter at the top of a Markdown file: lines
// between a leading "---" and the next "---" (or "..."). Only top-level scalar keys are
// returned; quoted values are unquoted and indented continuation lines (plain multi-line or
// block scalars) are folded into the value.
func readFrontmatter(absPath string) (map[string]string, error) {
	data, err := readLimited(absPath, maxFrontmatterProbeBytes)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte(utf8BOM))

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxFrontmatterProbeBytes)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != frontmatterDelimiter {
		return nil, errNoFrontmatter
	}

	parser := frontmatterParser{values: map[string]string{}}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == frontmatterDelimiter || trimmed == frontmatterEndAlternate {
			parser.flush()
			return parser.values, nil
		}
		parser.consume(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errNoFrontmatter
}

var errNoFrontmatter = errors.New("no frontmatter block")

// frontmatterParser accumulates top-level "key: value" pairs line by line.
type frontmatterParser struct {
	values  map[string]string
	key     string
	parts   []string
	literal bool // "|" block scalar: keep line breaks
}

func (p *frontmatterParser) consume(line string) {
	trimmed := strings.TrimSpace(line)
	isIndented := line != "" && (line[0] == ' ' || line[0] == '\t')

	if isIndented {
		if p.key != "" && trimmed != "" {
			p.parts = append(p.parts, trimmed)
		}
		return
	}
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return
	}

	p.flush()
	key, value, found := strings.Cut(trimmed, ":")
	key = strings.TrimSpace(key)
	if !found || key == "" {
		return
	}
	p.key = key
	value = strings.TrimSpace(value)
	if isBlockIndicator(value) {
		p.literal = strings.HasPrefix(value, "|")
		return
	}
	if value != "" {
		p.parts = append(p.parts, value)
	}
}

func (p *frontmatterParser) flush() {
	if p.key == "" {
		return
	}
	separator := " "
	if p.literal {
		separator = "\n"
	}
	p.values[p.key] = unquote(strings.Join(p.parts, separator))
	p.key = ""
	p.parts = nil
	p.literal = false
}

func isBlockIndicator(value string) bool {
	if value == "" {
		return false
	}
	if value[0] != '|' && value[0] != '>' {
		return false
	}
	return strings.Trim(value[1:], "+-0123456789") == ""
}

func unquote(value string) string {
	const minQuotedLen = 2
	if len(value) >= minQuotedLen {
		first, last := value[0], value[len(value)-1]
		if (first == '"' || first == '\'') && first == last {
			return value[1 : len(value)-1]
		}
	}
	if idx := strings.Index(value, " #"); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func readLimited(absPath string, limit int64) ([]byte, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

var errTooLarge = errors.New("file too large to probe")

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

// defaultName is the {name} value before any frontmatter override: the folder name for dirs,
// the base name without extension for files (falling back to the full name for dotfiles).
func defaultName(baseName string, isDir bool) string {
	if isDir {
		return baseName
	}
	name := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	if name == "" {
		return baseName
	}
	return name
}
