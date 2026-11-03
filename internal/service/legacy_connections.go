package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
)

// legacyConnection is a folder connection stored by versions that predate sync rules: a top-level
// folder of the repository (the profile) mirrored to one folder on this PC.
type legacyConnection struct {
	ID         string `json:"id"`
	Profile    string `json:"profile"`
	TargetPath string `json:"targetPath"`
}

// legacyConfigFile reads only the field the migration needs from config.json.
type legacyConfigFile struct {
	Connections []legacyConnection `json:"connections"`
}

const (
	// legacyConnectionMode is the mode given to migrated rules. The old mirror compared the repo
	// file, the PC file and the last synced hash, copied repo changes to the PC and PC edits back to
	// the repo, and asked on conflicts: exactly what a two-way rule does.
	legacyConnectionMode = rules.ModeTwoWay
	// legacyDestinationPath is the location root itself, because the old target folder was the mirror.
	legacyDestinationPath = "."
	// fallbackAliasName is used when a profile name has no characters allowed in an alias.
	fallbackAliasName = "location"
)

var aliasForbiddenChars = regexp.MustCompile(`[^a-z0-9_-]+`)

// readLegacyConnections returns the connections still stored in config.json. A missing or
// unreadable file has none.
func readLegacyConnections() []legacyConnection {
	data, err := os.ReadFile(filepath.Join(config.GetConfigDir(), config.ConfigFileName))
	if err != nil {
		return nil
	}
	var file legacyConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil
	}
	return file.Connections
}

// migrateLegacyConnectionsLocked turns the folder connections found in config.json into local rules
// (repo folder -> location folder, two-way), carries their sync baselines over, and drops the old
// data from config and state. It is idempotent: once config.json is saved without connections there
// is nothing left to migrate. Profile folders missing from the repository are skipped.
func (s *Service) migrateLegacyConnectionsLocked() error {
	legacy := readLegacyConnections()
	if len(legacy) == 0 {
		return s.saveStateIf(s.st.PruneBaselines(isRuleBaselineKey))
	}

	localFile, err := rules.LoadLocalRules()
	if err != nil {
		return err
	}
	rulesChanged := false
	for _, conn := range legacy {
		conn.Profile = strings.TrimSpace(conn.Profile)
		target, ok := legacyTarget(s.cfg.RootPath, conn)
		if !ok {
			continue
		}
		alias := s.locationAliasFor(target, conn.Profile)

		if existing := findMigratedRule(localFile, conn.Profile, alias); existing != nil {
			s.carryBaselines(conn.ID, *existing)
			continue
		}
		rule := newMigratedRule(conn.Profile, alias)
		localFile.Rules = append(localFile.Rules, rule)
		s.carryBaselines(conn.ID, rule)
		rulesChanged = true
	}

	if rulesChanged {
		if err := rules.SaveLocalRules(localFile); err != nil {
			return err
		}
	}
	s.st.PruneBaselines(isRuleBaselineKey)
	if err := s.st.Save(); err != nil {
		return err
	}
	// Saving the config is the commit point: it no longer holds the connections.
	return config.Save(s.cfg)
}

func (s *Service) saveStateIf(changed bool) error {
	if !changed {
		return nil
	}
	return s.st.Save()
}

func isRuleBaselineKey(key string) bool {
	return strings.HasPrefix(key, rulesync.BaselinePrefix)
}

// legacyTarget validates a connection against the repository and returns its cleaned target folder.
func legacyTarget(rootPath string, conn legacyConnection) (string, bool) {
	profile := strings.TrimSpace(conn.Profile)
	if rootPath == "" || profile == "" || profile != filepath.Base(profile) || profile == "." || profile == ".." {
		return "", false
	}
	if info, err := os.Stat(filepath.Join(rootPath, profile)); err != nil || !info.IsDir() {
		return "", false
	}
	target := filepath.Clean(conn.TargetPath)
	if conn.TargetPath == "" || !filepath.IsAbs(target) {
		return "", false
	}
	return target, true
}

// locationAliasFor returns the alias that maps to target, creating one named after the profile when
// no location has that path yet.
func (s *Service) locationAliasFor(target, profile string) string {
	if !s.cfg.HomeLocationDisabled && samePath(rules.DefaultHomeDir(), target) {
		return rules.BuiltinHomeAlias
	}
	taken := make(map[string]bool, len(s.cfg.Locations)+1)
	for _, loc := range s.cfg.Locations {
		if samePath(loc.Path, target) {
			return loc.Alias
		}
		taken[loc.Alias] = true
	}
	if !s.cfg.HomeLocationDisabled {
		taken[rules.BuiltinHomeAlias] = true
	}

	alias := uniqueAlias(aliasBase(profile), taken)
	s.cfg.Locations = append(s.cfg.Locations, config.Location{Alias: alias, Path: target})
	return alias
}

func samePath(a, b string) bool {
	return a != "" && b != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// aliasBase lower-cases a profile name and keeps only characters an alias accepts.
func aliasBase(profile string) string {
	base := aliasForbiddenChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(profile)), "-")
	base = strings.TrimLeft(base, "-_")
	if len(base) > maxAliasLength {
		base = base[:maxAliasLength]
	}
	if base == "" {
		return fallbackAliasName
	}
	return base
}

// uniqueAlias appends -2, -3, ... until the alias is not taken (and still fits the length limit).
func uniqueAlias(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		suffix := "-" + strconv.Itoa(n)
		stem := base
		if len(stem)+len(suffix) > maxAliasLength {
			stem = stem[:maxAliasLength-len(suffix)]
		}
		if candidate := stem + suffix; !taken[candidate] {
			return candidate
		}
	}
}

func newMigratedRule(profile, alias string) rules.Rule {
	return rules.Rule{
		ID:      rules.NewID(rules.RuleIDPrefix),
		Name:    profile,
		Enabled: true,
		Source:  rules.Source{Origin: rules.OriginRepo, Path: profile},
		Kind:    rules.KindFolder,
		Destinations: []rules.Destination{{
			ID:       rules.NewID(rules.DestinationIDPrefix),
			Location: alias,
			Path:     legacyDestinationPath,
			Mode:     legacyConnectionMode,
			Enabled:  true,
		}},
	}
}

// findMigratedRule finds a local rule that already mirrors the profile into the alias, so a retried
// migration does not create it twice.
func findMigratedRule(file *rules.File, profile, alias string) *rules.Rule {
	for i := range file.Rules {
		rule := &file.Rules[i]
		if rule.Source.Origin != rules.OriginRepo || rule.Source.Path != profile {
			continue
		}
		for _, dest := range rule.Destinations {
			if dest.Location == alias && dest.Path == legacyDestinationPath {
				return rule
			}
		}
	}
	return nil
}

// carryBaselines copies the last synced hashes of a connection to the migrated rule destination so
// the first rule sync still tells which side changed instead of reporting every differing file.
func (s *Service) carryBaselines(connectionID string, rule rules.Rule) {
	if connectionID == "" || len(rule.Destinations) == 0 {
		return
	}
	key := rulesync.BaselineKey(rule.ID, rule.Destinations[0].ID)
	for rel, hash := range s.st.GetBaselines(connectionID) {
		if s.st.GetBaseline(key, rel) == "" {
			s.st.SetBaseline(key, rel, hash)
		}
	}
}
