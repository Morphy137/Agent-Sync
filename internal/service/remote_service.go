package service

import (
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/subscriptions"
)

const (
	subscriptionTreeMaxDepth = 6
	binarySniffBytes         = 8000
	diffModeWorking          = "working"
	diffModeIncoming         = "incoming"
	headRev                  = "HEAD"
	upstreamRev              = "@{u}"
	diffLabelRepo            = "repository"
	diffLabelLocal           = "local"
	diffLabelRemote          = "remote"
)

// SubscriptionInfo is a subscription plus its latest known update status.
type SubscriptionInfo struct {
	config.Subscription
	Update    *subscriptions.UpdateInfo `json:"update,omitempty"`
	RuleCount int                       `json:"ruleCount"`
}

// SubscriptionUpdateResult summarizes an update: files changed in the clone and what the rules applied.
type SubscriptionUpdateResult struct {
	ChangedFiles []string             `json:"changedFiles"`
	Applied      rulesync.ApplyResult `json:"applied"`
}

func subscriptionsRoot(id string) string {
	return subscriptions.Root(id)
}

func (s *Service) hasSubscriptionLocked(id string) bool {
	return s.subscriptionIndexLocked(id) >= 0
}

func (s *Service) subscriptionIndexLocked(id string) int {
	for i := range s.cfg.Subscriptions {
		if s.cfg.Subscriptions[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *Service) subscriptionRuleCountsLocked() map[string]int {
	counts := make(map[string]int)
	all, err := s.allRulesLocked()
	if err != nil {
		return counts
	}
	for _, r := range all {
		if r.Source.Origin == rules.OriginSubscription {
			counts[r.Source.Subscription]++
		}
	}
	return counts
}

// ListSubscriptions returns subscriptions with the last known update check.
func (s *Service) ListSubscriptions() ([]SubscriptionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := s.subscriptionRuleCountsLocked()
	result := make([]SubscriptionInfo, 0, len(s.cfg.Subscriptions))
	for _, sub := range s.cfg.Subscriptions {
		info := SubscriptionInfo{Subscription: sub, RuleCount: counts[sub.ID]}
		if upd, ok := s.subUpdates[sub.ID]; ok {
			copied := upd
			info.Update = &copied
		}
		result = append(result, info)
	}
	return result, nil
}

// AddSubscription clones a remote repository (read-only) so its files can feed rules.
func (s *Service) AddSubscription(url, name string) (*config.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, err := subscriptions.Add(strings.TrimSpace(url), strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	s.cfg.Subscriptions = append(s.cfg.Subscriptions, sub)
	if err := config.Save(s.cfg); err != nil {
		_ = subscriptions.Remove(sub.ID)
		return nil, err
	}
	s.emitRulesChangedLocked()
	return &sub, nil
}

// RemoveSubscription deletes the clone and every local rule fed by it. Files already copied stay in place.
func (s *Service) RemoveSubscription(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.subscriptionIndexLocked(id)
	if idx < 0 {
		return apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", id)
	}

	localFile, err := rules.LoadLocalRules()
	if err != nil {
		return err
	}
	kept := make([]rules.Rule, 0, len(localFile.Rules))
	for _, r := range localFile.Rules {
		if r.Source.Origin == rules.OriginSubscription && r.Source.Subscription == id {
			for _, d := range r.Destinations {
				s.st.RemoveConnection(rulesync.BaselineKey(r.ID, d.ID))
				s.st.RemoveConnection(rulesync.ManagedKey(r.ID, d.ID))
			}
			continue
		}
		kept = append(kept, r)
	}
	localFile.Rules = kept
	if err := rules.SaveLocalRules(localFile); err != nil {
		return err
	}
	if err := s.st.Save(); err != nil {
		return err
	}

	s.cfg.Subscriptions = append(s.cfg.Subscriptions[:idx], s.cfg.Subscriptions[idx+1:]...)
	if err := config.Save(s.cfg); err != nil {
		return err
	}
	delete(s.subUpdates, id)
	if err := subscriptions.Remove(id); err != nil {
		return err
	}
	s.emitRulesChangedLocked()
	return nil
}

func (s *Service) checkSubscriptionLocked(idx int) (subscriptions.UpdateInfo, error) {
	sub := s.cfg.Subscriptions[idx]
	info, err := subscriptions.Check(sub)
	if err != nil {
		return info, err
	}
	s.subUpdates[sub.ID] = info
	s.cfg.Subscriptions[idx].LastCheckedAt = config.FormatRFC3339(time.Now())
	_ = config.Save(s.cfg)
	return info, nil
}

// CheckSubscription fetches the remote and reports incoming changes without touching any file.
func (s *Service) CheckSubscription(id string) (*subscriptions.UpdateInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.subscriptionIndexLocked(id)
	if idx < 0 {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", id)
	}
	info, err := s.checkSubscriptionLocked(idx)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// updateSubscriptionLocked pulls the subscription and applies its rules (conflicts are left for the user).
func (s *Service) updateSubscriptionLocked(idx int) (*SubscriptionUpdateResult, error) {
	sub := s.cfg.Subscriptions[idx]
	changed, err := subscriptions.Update(sub)
	if err != nil {
		return nil, err
	}
	s.cfg.Subscriptions[idx].LastUpdatedAt = config.FormatRFC3339(time.Now())
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}
	delete(s.subUpdates, sub.ID)

	all, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	subRules := make([]rules.Rule, 0)
	for _, r := range all {
		if r.Source.Origin == rules.OriginSubscription && r.Source.Subscription == sub.ID {
			subRules = append(subRules, r)
		}
	}
	applied := rulesync.ApplyResult{}
	if len(subRules) > 0 {
		applied, err = s.engineLocked().Sync(subRules)
		if err != nil {
			return nil, err
		}
	}
	if changed == nil {
		changed = []string{}
	}
	return &SubscriptionUpdateResult{ChangedFiles: changed, Applied: applied}, nil
}

// UpdateSubscription downloads the latest remote content and applies the subscription's rules.
func (s *Service) UpdateSubscription(id string) (*SubscriptionUpdateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.subscriptionIndexLocked(id)
	if idx < 0 {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", id)
	}
	res, err := s.updateSubscriptionLocked(idx)
	if err != nil {
		return nil, err
	}
	s.emitRulesChangedLocked()
	return res, nil
}

// SetSubscriptionAutoUpdate toggles automatic update + apply during periodic checks.
func (s *Service) SetSubscriptionAutoUpdate(id string, enabled bool) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.subscriptionIndexLocked(id)
	if idx < 0 {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", id)
	}
	s.cfg.Subscriptions[idx].AutoUpdate = enabled
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}
	return cloneConfig(s.cfg), nil
}

// GetSubscriptionTree lists the subscription's files/folders with detected kinds for selection.
func (s *Service) GetSubscriptionTree(id string) ([]SourceTreeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasSubscriptionLocked(id) {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", id)
	}
	return listSourceTree(subscriptionsRoot(id), subscriptionTreeMaxDepth, s.recognizersLocked())
}

// GetSubscriptionIncomingDiff compares a file between the local clone and the fetched remote.
func (s *Service) GetSubscriptionIncomingDiff(id, relPath string) (*rulesync.Diff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasSubscriptionLocked(id) {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", id)
	}
	before, after, err := subscriptions.IncomingDiff(id, relPath)
	if err != nil {
		return nil, err
	}
	return buildDiff(diffLabelLocal, diffLabelRemote, before, after), nil
}

// checkSubscriptionsLocked runs during periodic checks: fetches each subscription and auto-updates opted-in ones.
func (s *Service) checkSubscriptionsLocked() {
	for idx := range s.cfg.Subscriptions {
		info, err := s.checkSubscriptionLocked(idx)
		if err != nil || info.Offline || info.Behind == 0 {
			continue
		}
		if s.cfg.Subscriptions[idx].AutoUpdate {
			_, _ = s.updateSubscriptionLocked(idx)
		}
	}
}

// ---- Repository remote ----

// ConnectRemote attaches the root folder to a remote repository without losing local files.
// Local differences are left uncommitted so the user can review them before syncing.
func (s *Service) ConnectRemote(url string) (*gitops.ConnectRemoteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}
	res, err := gitops.ConnectRemote(s.cfg.RootPath, strings.TrimSpace(url))
	if err != nil {
		return nil, err
	}
	s.emitStatusLocked()
	s.emitRulesChangedLocked()
	return res, nil
}

// GetRepoFileDiff compares a repo file: "working" = HEAD vs working tree, "incoming" = HEAD vs upstream.
func (s *Service) GetRepoFileDiff(relPath, mode string) (*rulesync.Diff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}
	if !gitops.IsRepo(s.cfg.RootPath) {
		return nil, apperr.New(apperr.RootNotRepo)
	}
	abs, err := safeJoin(s.cfg.RootPath, filepath.FromSlash(relPath))
	if err != nil {
		return nil, err
	}
	runner := gitops.NewRunner(s.cfg.RootPath)
	head, _, err := runner.ShowFile(headRev, relPath)
	if err != nil {
		return nil, err
	}

	switch mode {
	case diffModeWorking:
		working, readErr := os.ReadFile(abs)
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, apperr.Wrap(apperr.Unknown, readErr)
		}
		return buildDiff(diffLabelRepo, diffLabelLocal, head, working), nil
	case diffModeIncoming:
		upstream, _, err := runner.ShowFile(upstreamRev, relPath)
		if err != nil {
			return nil, err
		}
		return buildDiff(diffLabelLocal, diffLabelRemote, head, upstream), nil
	}
	return nil, apperr.New(apperr.Unknown).WithParam("mode", mode)
}

// DiscardRepoFile reverts a local repo change to the committed version (backup first).
// Files that do not exist in HEAD are removed after the backup.
func (s *Service) DiscardRepoFile(relPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return apperr.New(apperr.RootNotFound)
	}
	abs, err := safeJoin(s.cfg.RootPath, filepath.FromSlash(relPath))
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(abs); statErr == nil {
		if _, err := backup.CreateRepoBackup(s.cfg.BackupPath, s.cfg.RootPath, []string{relPath}, backup.ReasonDiscardRepoEdit, s.cfg.BackupRetention); err != nil {
			return err
		}
	}
	runner := gitops.NewRunner(s.cfg.RootPath)
	_, inHead, err := runner.ShowFile(headRev, relPath)
	if err != nil {
		return err
	}
	if inHead {
		if _, err := runner.DiscardPath(relPath); err != nil {
			return err
		}
	} else if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	s.emitStatusLocked()
	return nil
}

// buildDiff packages two file versions for the UI, flagging binary or oversized content.
func buildDiff(leftLabel, rightLabel string, left, right []byte) *rulesync.Diff {
	diff := &rulesync.Diff{LeftLabel: leftLabel, RightLabel: rightLabel}
	if isBinaryContent(left) || isBinaryContent(right) || len(left) > rulesync.MaxDiffBytes || len(right) > rulesync.MaxDiffBytes {
		diff.Binary = true
		return diff
	}
	diff.Left = string(left)
	diff.Right = string(right)
	return diff
}

func isBinaryContent(data []byte) bool {
	sniff := data
	if len(sniff) > binarySniffBytes {
		sniff = sniff[:binarySniffBytes]
	}
	for _, b := range sniff {
		if b == 0 {
			return true
		}
	}
	return !utf8.Valid(data)
}

// syncRulesAfterGitLocked applies rules after the git sync and commits/pushes captured repo files.
func (s *Service) syncRulesAfterGitLocked(res *orchestrator.SyncResult) error {
	all, err := s.allRulesLocked()
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return nil
	}
	applied, err := s.engineLocked().Sync(all)
	if err != nil {
		return err
	}
	res.RulesWritten = applied.Written + applied.Deleted
	res.RulesCaptured = applied.Captured
	res.RulesSkipped = applied.Skipped
	res.BackupSessions = append(res.BackupSessions, applied.Backups...)

	if len(applied.CapturedRepoPaths) == 0 || !gitops.IsRepo(s.cfg.RootPath) {
		return nil
	}
	runner := gitops.NewRunner(s.cfg.RootPath)
	runner.OnBeforePush = s.orch.OnBeforePush
	if err := runner.EnsureIdentity(); err != nil {
		return err
	}
	hostname, hostErr := os.Hostname()
	if hostErr != nil || hostname == "" {
		hostname = "local"
	}
	if err := runner.CommitPaths(applied.CapturedRepoPaths, "chore(sync): update rule sources from "+hostname); err != nil {
		return err
	}
	if !runner.HasUpstream() {
		return nil
	}
	if err := runner.Push(); err != nil {
		return err
	}
	res.Pushed++
	return nil
}
