package service

import "agentsync/internal/rules"

// rulesAutoAppliedEvent tells the UI that a periodic check wrote subscription files on its own.
const rulesAutoAppliedEvent = "rules:auto_applied"

// AutoAppliedPayload is the data of rulesAutoAppliedEvent.
type AutoAppliedPayload struct {
	// Count is the number of files written or removed.
	Count int `json:"count"`
	// Sources lists the names of the subscriptions the files came from.
	Sources []string `json:"sources"`
}

// autoApplied accumulates what a periodic check wrote without the user pressing anything.
type autoApplied struct {
	count   int
	sources []string
}

func (a *autoApplied) add(source string, files int) {
	if files <= 0 {
		return
	}
	a.count += files
	for _, known := range a.sources {
		if known == source {
			return
		}
	}
	a.sources = append(a.sources, source)
}

func fromSubscription(id string) func(rules.Rule) bool {
	return func(r rules.Rule) bool {
		return r.Source.Origin == rules.OriginSubscription && r.Source.Subscription == id
	}
}

// autoApplySubscriptionRulesLocked writes the pending, non-conflicting changes of subscription rules
// when the "apply updates automatically" option is on. It goes through the same engine as SyncRules,
// so overwritten files are backed up first, conflicts and local edits are skipped, and nothing is
// committed or pushed.
func (s *Service) autoApplySubscriptionRulesLocked(applied *autoApplied) {
	if !s.cfg.AutoApplyUpdates {
		return
	}
	all, err := s.allRulesLocked()
	if err != nil {
		return
	}
	for _, sub := range s.cfg.Subscriptions {
		filter := fromSubscription(sub.ID)
		if !anyRule(all, filter) {
			continue
		}
		result, err := s.engineLocked().SyncMatching(all, filter)
		if err != nil {
			continue
		}
		applied.add(sub.Name, result.Written+result.Deleted)
	}
}

func anyRule(all []rules.Rule, match func(rules.Rule) bool) bool {
	for _, r := range all {
		if match(r) {
			return true
		}
	}
	return false
}

// emitAutoAppliedLocked reports the files written by a periodic check, if any.
func (s *Service) emitAutoAppliedLocked(applied *autoApplied) {
	if s.emitFunc == nil || !s.cfg.AutoApplyUpdates || applied.count == 0 {
		return
	}
	s.emitFunc(rulesAutoAppliedEvent, AutoAppliedPayload{Count: applied.count, Sources: applied.sources})
}
