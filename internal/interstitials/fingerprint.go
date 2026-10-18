package interstitials

// ScheduleKey excludes cosmetic branding and asset names, which cannot change timing.
func (s Snapshot) ScheduleKey() string {
	if !s.Branding.Enabled || len(s.Rules) == 0 {
		return ""
	}
	assets := []any{}
	for _, a := range s.Assets {
		used := false
		for _, r := range s.Rules {
			if !r.Enabled || r.Kind != a.Kind {
				continue
			}
			if r.Tag == "" {
				used = true
			}
			for _, tag := range a.Tags {
				if tag == r.Tag {
					used = true
				}
			}
		}
		if !used {
			continue
		}
		assets = append(assets, []any{a.ID, a.Kind, a.Source, a.MediaID, a.Duration, a.Enabled, a.Available, a.Tags, a.ActiveFrom, a.ActiveUntil})
	}
	rules := append([]Rule{}, s.Rules...)
	for i := range rules {
		rules[i].Name = ""
	}
	return Fingerprint([]any{rules, assets})
}
