package service

import (
	"agentsync/internal/apperr"
	"agentsync/internal/rules"
)

// UpdateRuleInScope patches an existing rule and optionally moves it between
// scopes under one service lock. Empty scope delegates to UpdateRule, preserving
// its current scope. Never call SaveRule from an UpdateRule callback: both lock mu.
// Like SaveRule, migration writes the target before removing the old copy; file
// write failures are reported and this is not a multi-file filesystem transaction.
func (s *Service) UpdateRuleInScope(id, scope string, mutate func(*rules.Rule) error) (*SaveRuleResult, error) {
	if scope == "" {
		return s.UpdateRule(id, mutate)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, local, err := s.loadRuleFilesLocked()
	if err != nil {
		return nil, err
	}
	var rule rules.Rule
	if i := findRuleIndex(repo, id); i >= 0 {
		rule = repo.Rules[i]
	} else if i := findRuleIndex(local, id); i >= 0 {
		rule = local.Rules[i]
	} else {
		return nil, apperr.New(apperr.RuleNotFound).WithParam("rule", id)
	}
	if err := mutate(&rule); err != nil {
		return nil, err
	}
	rule.ID = id
	return s.saveRuleLocked(rule, scope)
}
