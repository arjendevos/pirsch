package analyzer

import (
	"github.com/pirsch-analytics/pirsch/v6/pkg/db"
	"github.com/pirsch-analytics/pirsch/v6/pkg/model"
)

// Events aggregate statistics regarding events.
type Events struct {
	analyzer *Analyzer
	store    db.Store
}

// Events return the visitor count, views, and conversion rate for custom events.
func (events *Events) Events(filter *Filter) ([]model.EventStats, error) {
	filter = events.analyzer.getFilter(filter)

	// Get total visitors with all filters applied (for CR calculation)
	totalVisitors, err := events.getTotalVisitors(filter)
	if err != nil {
		return nil, err
	}

	q, args := filter.buildQuery([]Field{
		FieldEventName,
		FieldCount,
		FieldVisitors,
		FieldViews,
		FieldCR,
		FieldEventTimeSpent,
		FieldEventMetaKeys,
	}, []Field{
		FieldEventName,
	}, []Field{
		FieldVisitors,
		FieldEventName,
	}, nil, "")
	stats, err := events.store.SelectEventStats(filter.Ctx, false, q, args...)

	if err != nil {
		return nil, err
	}

	// Calculate CR for each event based on filtered total visitors
	for i := range stats {
		if totalVisitors > 0 {
			stats[i].CR = float64(stats[i].Visitors) / float64(totalVisitors)
		}
	}

	return stats, nil
}

// Breakdown returns the visitor count, views, and conversion rate for a custom event grouping them by a meta-value for a given key.
// The Filter.EventName and Filter.EventMetaKey must be set, or otherwise the result set will be empty.
// When filtering by EventMetaKey, the CR is calculated per-value based on visitors who have the corresponding tag.
// For example, if EventMetaKey is "template", and a result has MetaValue "lunar", the CR is calculated as:
// event_visitors_with_lunar / visitors_with_lunar_template_tag
// If no visitors have the corresponding tag, falls back to total visitors for CR calculation.
func (events *Events) Breakdown(filter *Filter) ([]model.EventStats, error) {
	filter = events.analyzer.getFilter(filter)

	if len(filter.EventName) == 0 || len(filter.EventMetaKey) == 0 {
		return []model.EventStats{}, nil
	}

	// Get total visitors for fallback CR calculation
	totalVisitors, err := events.getTotalVisitors(filter)
	if err != nil {
		return nil, err
	}

	q, args := filter.buildQuery([]Field{
		FieldEventName,
		FieldCount,
		FieldVisitors,
		FieldViews,
		FieldCR,
		FieldEventTimeSpent,
		FieldEventMetaValues,
	}, []Field{
		FieldEventName,
		FieldEventMetaValues,
	}, []Field{
		FieldVisitors,
		FieldEventMetaValues,
	}, nil, "")
	stats, err := events.store.SelectEventStats(filter.Ctx, true, q, args...)

	if err != nil {
		return nil, err
	}

	// Calculate CR for each breakdown based on visitors with the corresponding tag value
	// This uses the EventMetaKey as the tag key and the MetaValue as the tag value
	// Falls back to total visitors if no visitors have the tag

	// Collect all unique meta values to batch query tag visitors
	metaValues := make([]string, 0, len(stats))
	for _, stat := range stats {
		if stat.MetaValue != "" {
			metaValues = append(metaValues, stat.MetaValue)
		}
	}

	// Get visitor counts for all tag values in a single query
	tagVisitorsMap, err := events.getVisitorsByTagValues(filter, filter.EventMetaKey[0], metaValues)
	if err != nil {
		return nil, err
	}

	for i := range stats {
		tagVisitors := tagVisitorsMap[stats[i].MetaValue]
		if tagVisitors > 0 {
			stats[i].CR = float64(stats[i].Visitors) / float64(tagVisitors)
		} else if totalVisitors > 0 {
			stats[i].CR = float64(stats[i].Visitors) / float64(totalVisitors)
		}
	}

	return stats, nil
}

// List returns events as a list. The metadata is grouped as key-value pairs.
func (events *Events) List(filter *Filter) ([]model.EventListStats, error) {
	filter = events.analyzer.getFilter(filter)
	q, args := filter.buildQuery([]Field{
		FieldEventName,
		FieldEventMeta,
		FieldVisitors,
		FieldCount,
	}, []Field{
		FieldEventName,
		FieldEventMeta,
	}, []Field{
		FieldCount,
		FieldEventName,
	}, nil, "")
	stats, err := events.store.SelectEventListStats(filter.Ctx, q, args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// getTotalVisitors returns the total number of unique visitors for the given filter.
// This respects all filters EXCEPT event-specific filters (event name, event meta)
// so that CR is calculated as: event_visitors / total_session_visitors
func (events *Events) getTotalVisitors(filter *Filter) (int, error) {
	// Clone the filter and clear Sort and event-specific filters
	filterCopy := *filter
	filterCopy.Sort = nil
	filterCopy.EventName = nil
	filterCopy.EventMetaKey = nil
	filterCopy.EventMeta = nil

	// Build a query to get total unique visitors from session table with non-event filters
	q, args := filterCopy.buildQuery([]Field{
		FieldVisitors,
	}, nil, nil, nil, "")

	return events.store.GetTotalUniqueVisitorStats(filterCopy.Ctx, q, args...)
}

// getVisitorsByTagValues returns a map of tag values to their visitor counts.
// This batches all tag value lookups into a single query for better performance.
func (events *Events) getVisitorsByTagValues(filter *Filter, tagKey string, tagValues []string) (map[string]int, error) {
	if len(tagValues) == 0 {
		return make(map[string]int), nil
	}

	// Clone the filter and clear Sort and event-specific filters
	filterCopy := *filter
	filterCopy.Sort = nil
	filterCopy.EventName = nil
	filterCopy.EventMetaKey = nil
	filterCopy.EventMeta = nil

	// Set the tag key filter to query by this tag
	filterCopy.Tag = []string{tagKey}

	// Build a query to get visitors grouped by tag value (same fields as tag breakdown)
	q, args := filterCopy.buildQuery([]Field{
		FieldTagValue,
		FieldVisitors,
		FieldViews,
		FieldRelativeVisitors,
		FieldRelativeViews,
	}, []Field{
		FieldTagValue,
	}, []Field{
		FieldVisitors,
		FieldTagValue,
	}, nil, "")

	stats, err := events.store.SelectTagStats(filterCopy.Ctx, true, q, args...)
	if err != nil {
		return nil, err
	}

	// Build a map of tag value -> visitors
	result := make(map[string]int, len(stats))
	for _, stat := range stats {
		result[stat.Value] = stat.Visitors
	}

	return result, nil
}
