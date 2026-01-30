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
func (events *Events) Breakdown(filter *Filter) ([]model.EventStats, error) {
	filter = events.analyzer.getFilter(filter)

	if len(filter.EventName) == 0 || len(filter.EventMetaKey) == 0 {
		return []model.EventStats{}, nil
	}

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

	// Calculate CR for each breakdown based on filtered total visitors
	for i := range stats {
		if totalVisitors > 0 {
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
