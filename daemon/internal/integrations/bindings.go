package integrations

func CloneBindingSummaries(items []BindingSummary) []BindingSummary {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]BindingSummary, len(items))
	copy(cloned, items)
	return cloned
}
