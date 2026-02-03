package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pirsch-analytics/pirsch/v6/pkg"
	"github.com/pirsch-analytics/pirsch/v6/pkg/db"
	"github.com/pirsch-analytics/pirsch/v6/pkg/model"
)

// Pages aggregates statistics regarding pages.
type Pages struct {
	analyzer *Analyzer
	store    db.Store
}

// Hostname returns the visitor count, session count, bounce rate, and views grouped by hostname.
func (pages *Pages) Hostname(filter *Filter) ([]model.HostnameStats, error) {
	filter = pages.analyzer.getFilter(filter)
	q, args := filter.buildQuery([]Field{
		FieldHostname,
		FieldVisitors,
		FieldViews,
		FieldSessions,
		FieldBounces,
		FieldRelativeVisitors,
		FieldRelativeViews,
		FieldBounceRate,
	}, []Field{
		FieldHostname,
	}, []Field{
		FieldVisitors,
		FieldHostname,
	}, nil, "")
	stats, err := pages.store.SelectHostnameStats(filter.Ctx, q, args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// ByPath returns the visitor count, session count, bounce rate, views, and average time on the page grouped by hostname, path, and (optional) page title.
func (pages *Pages) ByPath(filter *Filter) ([]model.PageStats, error) {
	return pages.byPath(filter, false)
}

// ByEventPath returns the visitor count, session count, bounce rate, views, and average time on the page grouped by hostname, event path, and (optional) title.
func (pages *Pages) ByEventPath(filter *Filter) ([]model.PageStats, error) {
	if len(filter.EventName) == 0 {
		return []model.PageStats{}, nil
	}

	return pages.byPath(filter, true)
}

func (pages *Pages) byPath(filter *Filter, eventPath bool) ([]model.PageStats, error) {
	filter = pages.analyzer.getFilter(filter)
	pathField := FieldPath

	if eventPath {
		pathField = FieldEventPath
	}

	fields := []Field{
		pathField,
		FieldHostname,
		FieldVisitors,
		FieldSessions,
		FieldRelativeVisitors,
		FieldViews,
		FieldRelativeViews,
		FieldBounces,
		FieldBounceRate,
	}
	groupBy := []Field{
		pathField,
		FieldHostname,
	}
	orderBy := []Field{
		FieldVisitors,
		pathField,
	}

	if filter.IncludeTitle {
		if eventPath {
			fields = append(fields, FieldEventTitle)
			groupBy = append(groupBy, FieldEventTitle)
			orderBy = append(orderBy, FieldEventTitle)
		} else {
			fields = append(fields, FieldTitle)
			groupBy = append(groupBy, FieldTitle)
			orderBy = append(orderBy, FieldTitle)
		}
	}

	q, args := filter.buildQuery(fields, groupBy, orderBy, []Field{
		FieldPath,
		FieldVisitors,
		FieldViews,
		FieldSessions,
		FieldBounces,
	}, "imported_page")
	stats, err := pages.store.SelectPageStats(filter.Ctx, filter.IncludeTitle, false, q, args...)

	if err != nil {
		return nil, err
	}

	if filter.IncludeTimeOnPage {
		n := len(stats)

		for start := 0; start < n; start += 1000 {
			end := start + 1000

			if end > n {
				end = n
			}

			pathList := getPathList(stats[start:end])
			top, err := pages.avgTimeOnPage(filter, pathList)

			if err != nil {
				return nil, err
			}

			for i := range stats {
				for j := range top {
					if stats[i].Path == top[j].Path {
						stats[i].AverageTimeSpentSeconds = top[j].AverageTimeSpentSeconds
						break
					}
				}
			}
		}
	}

	return stats, nil
}

// Entry returns the visitor count and time on the page grouped by hostname, path, and (optional) page title for the first page visited.
func (pages *Pages) Entry(filter *Filter) ([]model.EntryStats, error) {
	filter = pages.analyzer.getFilter(filter)
	var sortVisitors pkg.Direction

	if len(filter.Sort) > 0 && filter.Sort[0].Field == FieldVisitors {
		sortVisitors = filter.Sort[0].Direction
		filter.Sort = filter.Sort[:0]
	}

	fields := []Field{
		FieldEntryPath,
		FieldHostname,
		FieldEntries,
		FieldEntryRate,
	}
	groupBy := []Field{
		FieldEntryPath,
		FieldHostname,
	}
	orderBy := []Field{
		FieldEntries,
		FieldEntryPath,
	}

	if filter.IncludeTitle {
		fields = append(fields, FieldEntryTitle)
		groupBy = append(groupBy, FieldEntryTitle)
		orderBy = append(orderBy, FieldEntryTitle)
	}

	q, args := filter.buildQuery(fields, groupBy, orderBy, []Field{
		FieldEntryPath,
		FieldVisitors,
	}, "imported_entry_page")
	stats, err := pages.store.SelectEntryStats(filter.Ctx, filter.IncludeTitle, q, args...)

	if err != nil {
		return nil, err
	}

	n := len(stats)

	for start := 0; start < n; start += 1000 {
		end := start + 1000

		if end > n {
			end = n
		}

		pathList := getPathList(stats[start:end])
		total, err := pages.totalVisitorsSessions(filter, pathList)

		if err != nil {
			return nil, err
		}

		for i := range stats {
			for j := range total {
				if stats[i].Path == total[j].Path {
					stats[i].Visitors = total[j].Visitors
					stats[i].Sessions = total[j].Sessions
					break
				}
			}
		}

		if filter.IncludeTimeOnPage {
			top, err := pages.avgTimeOnPage(filter, pathList)

			if err != nil {
				return nil, err
			}

			for i := range stats {
				for j := range top {
					if stats[i].Path == top[j].Path {
						stats[i].AverageTimeSpentSeconds = top[j].AverageTimeSpentSeconds
						break
					}
				}
			}
		}
	}

	if sortVisitors != "" {
		if sortVisitors == pkg.DirectionASC {
			sort.Slice(stats, func(i, j int) bool {
				return stats[i].Visitors < stats[j].Visitors
			})
		} else {
			sort.Slice(stats, func(i, j int) bool {
				return stats[i].Visitors > stats[j].Visitors
			})
		}
	}

	return stats, nil
}

// Exit returns the visitor count and time on the page grouped by hostname, path, and (optional) page title for the last page visited.
func (pages *Pages) Exit(filter *Filter) ([]model.ExitStats, error) {
	filter = pages.analyzer.getFilter(filter)
	var sortVisitors pkg.Direction

	if len(filter.Sort) > 0 && filter.Sort[0].Field == FieldVisitors {
		sortVisitors = filter.Sort[0].Direction
		filter.Sort = filter.Sort[:0]
	}

	fields := []Field{
		FieldExitPath,
		FieldHostname,
		FieldExits,
		FieldExitRate,
	}
	groupBy := []Field{
		FieldExitPath,
		FieldHostname,
	}
	orderBy := []Field{
		FieldExits,
		FieldExitPath,
	}

	if filter.IncludeTitle {
		fields = append(fields, FieldExitTitle)
		groupBy = append(groupBy, FieldExitTitle)
		orderBy = append(orderBy, FieldExitTitle)
	}

	q, args := filter.buildQuery(fields, groupBy, orderBy, []Field{
		FieldExitPath,
		FieldVisitors,
	}, "imported_exit_page")
	stats, err := pages.store.SelectExitStats(filter.Ctx, filter.IncludeTitle, q, args...)

	if err != nil {
		return nil, err
	}

	n := len(stats)

	for start := 0; start < n; start += 1000 {
		end := start + 1000

		if end > n {
			end = n
		}

		pathList := getPathList(stats[start:end])
		total, err := pages.totalVisitorsSessions(filter, pathList)

		if err != nil {
			return nil, err
		}

		for i := range stats {
			for j := range total {
				if stats[i].Path == total[j].Path {
					stats[i].Visitors = total[j].Visitors
					stats[i].Sessions = total[j].Sessions
					break
				}
			}
		}
	}

	if sortVisitors != "" {
		if sortVisitors == pkg.DirectionASC {
			sort.Slice(stats, func(i, j int) bool {
				return stats[i].Visitors < stats[j].Visitors
			})
		} else {
			sort.Slice(stats, func(i, j int) bool {
				return stats[i].Visitors > stats[j].Visitors
			})
		}
	}

	return stats, nil
}

// Conversions return the visitor count, views, conversion rate, and custom metric for conversion goals.
func (pages *Pages) Conversions(filter *Filter) (*model.ConversionsStats, error) {
	filter = pages.analyzer.getFilter(filter)
	fields := []Field{
		FieldVisitors,
		FieldViews,
		FieldCR,
	}
	includeCustomMetric := false

	if len(filter.EventName) > 0 && filter.CustomMetricType != "" && filter.CustomMetricKey != "" {
		fields = append(fields, FieldEventMetaCustomMetricAvg, FieldEventMetaCustomMetricTotal)
		includeCustomMetric = true
	}

	q, args := filter.buildQuery(fields, nil, []Field{FieldVisitors}, nil, "")
	stats, err := pages.store.GetConversionsStats(filter.Ctx, q, includeCustomMetric, args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// ConversionsByPage returns the event conversion rate grouped by page path.
// Shows what percentage of visitors to each page triggered the specified event.
// Requires filter.EventName to be set. Optionally filters by event meta key/value.
func (pages *Pages) ConversionsByPage(filter *Filter) ([]model.PageConversionStats, error) {
	if len(filter.EventName) == 0 {
		return []model.PageConversionStats{}, nil
	}

	filter = pages.analyzer.getFilter(filter)
	fields := []Field{
		FieldPath,
		FieldHostname,
		FieldVisitors,
		FieldViews,
		FieldEventCount,
		FieldEventVisitors,
		FieldCR,
	}
	groupBy := []Field{
		FieldPath,
		FieldHostname,
	}
	orderBy := []Field{
		FieldCR,
		FieldVisitors,
		FieldPath,
	}

	if filter.IncludeTitle {
		fields = append(fields, FieldTitle)
		groupBy = append(groupBy, FieldTitle)
		orderBy = append(orderBy, FieldTitle)
	}

	includeCustomMetric := false

	if filter.CustomMetricType != "" && filter.CustomMetricKey != "" {
		fields = append(fields, FieldEventMetaCustomMetricAvg, FieldEventMetaCustomMetricTotal)
		includeCustomMetric = true
	}

	q, args := filter.buildQuery(fields, groupBy, orderBy, nil, "")
	stats, err := pages.store.SelectPageConversionStats(filter.Ctx, filter.IncludeTitle, includeCustomMetric, q, args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// ConversionsByPageBreakdown returns the event conversion rate grouped by page path with meta value breakdown.
// Shows what percentage of visitors to each page triggered the specified event, broken down by meta key values.
// Requires filter.EventName and filter.EventMetaKey to be set.
func (pages *Pages) ConversionsByPageBreakdown(filter *Filter) ([]model.PageConversionMetaStats, error) {
	if len(filter.EventName) == 0 || len(filter.EventMetaKey) == 0 {
		return []model.PageConversionMetaStats{}, nil
	}

	filter = pages.analyzer.getFilter(filter)

	// Step 1: Get paginated pages with page-level totals (respects LIMIT/OFFSET)
	pageFields := []Field{
		FieldPath,
		FieldHostname,
		FieldVisitors,
		FieldViews,
		FieldEventCount,
		FieldEventVisitors,
		FieldCR,
	}
	pageGroupBy := []Field{
		FieldPath,
		FieldHostname,
	}
	pageOrderBy := []Field{
		FieldCR,
		FieldVisitors,
		FieldPath,
	}

	q, args := filter.buildQuery(pageFields, pageGroupBy, pageOrderBy, nil, "")
	pageStats, err := pages.store.SelectPageConversionStats(filter.Ctx, false, false, q, args...)

	if err != nil {
		return nil, err
	}

	if len(pageStats) == 0 {
		return []model.PageConversionMetaStats{}, nil
	}

	// Step 2: Get meta value breakdown for those specific pages (no pagination)
	paths := make([]string, len(pageStats))
	for i, p := range pageStats {
		paths[i] = p.Path
	}

	breakdownFilter := pages.analyzer.getFilter(filter)
	breakdownFilter.Path = nil
	breakdownFilter.AnyPath = paths
	breakdownFilter.Offset = 0
	breakdownFilter.Limit = 0

	breakdownFields := []Field{
		FieldPath,
		FieldHostname,
		FieldVisitors,
		FieldViews,
		FieldEventCount,
		FieldEventVisitors,
		FieldCR,
		FieldEventMetaValues,
	}
	breakdownGroupBy := []Field{
		FieldPath,
		FieldHostname,
		FieldEventMetaValues,
	}
	breakdownOrderBy := []Field{
		FieldPath,
		FieldEventMetaValues,
	}

	includeCustomMetric := false

	if filter.CustomMetricType != "" && filter.CustomMetricKey != "" {
		breakdownFields = append(breakdownFields, FieldEventMetaCustomMetricAvg, FieldEventMetaCustomMetricTotal)
		includeCustomMetric = true
	}

	q, args = breakdownFilter.buildQuery(breakdownFields, breakdownGroupBy, breakdownOrderBy, nil, "")
	metaRows, err := pages.store.SelectPageConversionMetaStats(filter.Ctx, includeCustomMetric, q, args...)

	if err != nil {
		return nil, err
	}

	// Step 3: Combine page totals with meta breakdown
	return pages.combinePageConversionMetaStats(pageStats, metaRows), nil
}

// combinePageConversionMetaStats combines page-level stats with meta value breakdown rows.
func (pages *Pages) combinePageConversionMetaStats(pageStats []model.PageConversionStats, metaRows []model.PageConversionMetaRow) []model.PageConversionMetaStats {
	// Build meta values lookup by path+hostname
	metaLookup := make(map[string][]model.MetaValueStats)

	for _, row := range metaRows {
		key := row.Path + "\x00" + row.Hostname
		metaLookup[key] = append(metaLookup[key], model.MetaValueStats{
			Value:             row.MetaValue,
			Events:            row.Events,
			EventVisitors:     row.EventVisitors,
			CR:                row.CR,
			CustomMetricAvg:   row.CustomMetricAvg,
			CustomMetricTotal: row.CustomMetricTotal,
		})
	}

	// Build results maintaining page order from first query
	results := make([]model.PageConversionMetaStats, 0, len(pageStats))

	for _, page := range pageStats {
		key := page.Path + "\x00" + page.Hostname
		metaValues := metaLookup[key]

		if metaValues == nil {
			metaValues = []model.MetaValueStats{}
		} else {
			// Sort meta values by CR in descending order
			sort.Slice(metaValues, func(i, j int) bool {
				return metaValues[i].CR > metaValues[j].CR
			})
		}

		results = append(results, model.PageConversionMetaStats{
			Path:          page.Path,
			Hostname:      page.Hostname,
			Visitors:      page.Visitors,
			Views:         page.Views,
			Events:        page.Events,
			EventVisitors: page.EventVisitors,
			CR:            page.CR,
			MetaValues:    metaValues,
		})
	}

	return results
}

func (pages *Pages) totalVisitorsSessions(filter *Filter, paths []string) ([]model.TotalVisitorSessionStats, error) {
	if len(paths) == 0 {
		return []model.TotalVisitorSessionStats{}, nil
	}

	filter = pages.analyzer.getFilter(filter)
	filter.Path = nil
	filter.EntryPath = nil
	filter.ExitPath = nil
	filter.AnyPath = paths
	filter.PathPattern = nil
	filter.Tags = nil
	filter.Search = nil
	filter.IncludeTitle = false
	filter.Sort = nil
	filter.Offset = 0
	filter.Limit = 0
	q, args := filter.buildQuery([]Field{
		FieldPath,
		FieldVisitors,
		FieldSessions,
		FieldViews,
	}, []Field{
		FieldPath,
	}, []Field{
		FieldVisitors,
		FieldSessions,
	}, nil, "")
	stats, err := pages.store.SelectTotalVisitorSessionStats(filter.Ctx, q, args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (pages *Pages) avgTimeOnPage(filter *Filter, paths []string) ([]model.AvgTimeSpentStats, error) {
	if len(paths) == 0 {
		return []model.AvgTimeSpentStats{}, nil
	}

	filter = pages.analyzer.getFilter(filter)
	filter.Sort = nil
	filter.Search = nil
	q := queryBuilder{
		filter: filter,
		from:   pageViews,
		search: filter.Search,
	}
	fields := q.getFields()
	hasPath := false

	for _, field := range fields {
		if field == FieldPath.Name {
			hasPath = true
			break
		}
	}

	if !hasPath {
		fields = append(fields, FieldPath.Name)
	}

	var query strings.Builder
	query.WriteString(fmt.Sprintf(`SELECT path, round(avg(time_on_page)) average_time_spent_seconds
		FROM (
			SELECT nth_value(%s, 2) OVER (PARTITION BY v.visitor_id, v.session_id ORDER BY v."time" ASC Rows BETWEEN CURRENT ROW AND 1 FOLLOWING) AS time_on_page,
				%s
			FROM page_view v `, pages.analyzer.timeOnPageQuery(filter), strings.Join(fields, ",")))

	if len(filter.EntryPath) > 0 || len(filter.ExitPath) > 0 {
		sessionsQuery := queryBuilder{
			filter: filter,
			from:   sessions,
			fields: []Field{
				FieldVisitorID,
				FieldSessionID,
			},
			groupBy: []Field{
				FieldVisitorID,
				FieldSessionID,
			},
		}
		str, args := sessionsQuery.query()
		q.args = append(q.args, args...)
		query.WriteString(fmt.Sprintf(`INNER JOIN (%s) s ON v.visitor_id = s.visitor_id AND v.session_id = s.session_id `, str))
	}

	if len(filter.EventName) > 0 {
		eventsQuery := queryBuilder{
			filter: filter,
			from:   events,
			fields: []Field{
				FieldVisitorID,
				FieldSessionID,
			},
			groupBy: []Field{
				FieldVisitorID,
				FieldSessionID,
			},
		}
		str, args := eventsQuery.query()
		q.args = append(q.args, args...)
		query.WriteString(fmt.Sprintf(`INNER JOIN (%s) ev ON v.visitor_id = ev.visitor_id AND v.session_id = ev.session_id `, str))
	}

	whereTime := q.whereTime()
	q.whereFields()
	pathInQuery := queryBuilder{
		filter: &Filter{
			AnyPath: paths,
		},
	}
	pathInQuery.whereFieldPathIn()
	pathIn := pathInQuery.where[len(pathInQuery.where)-1].eqContains[0]
	q.args = append(q.args, pathInQuery.args...)
	query.WriteString(fmt.Sprintf(`%s)
		WHERE time_on_page > 0 %s
		AND %s
		GROUP BY path`, whereTime, q.q.String(), pathIn))
	stats, err := pages.store.SelectAvgTimeSpentStats(filter.Ctx, query.String(), q.args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

func getPathList[T interface{ GetPath() string }](stats []T) []string {
	paths := make(map[string]struct{})

	for i := range stats {
		paths[stats[i].GetPath()] = struct{}{}
	}

	pathList := make([]string, 0, len(paths))

	for path := range paths {
		pathList = append(pathList, path)
	}

	return pathList
}

// ConversionsByPath returns the event conversion stats grouped by path and hostname.
// Shows visitors, views, and total events (sum of event counts) for each path.
// Requires filter.EventName to be set.
func (pages *Pages) ConversionsByPath(filter *Filter) ([]model.PathConversionStats, error) {
	if len(filter.EventName) == 0 {
		return []model.PathConversionStats{}, nil
	}

	filter = pages.analyzer.getFilter(filter)
	q, args := pages.buildConversionsByPathQuery(filter)
	stats, err := pages.store.SelectPathConversionStats(filter.Ctx, q, args...)

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// ConversionsByPathBreakdown returns the event conversion stats grouped by path with meta value breakdown.
// Shows what percentage of visitors to each path triggered the specified event, broken down by meta key values.
// Requires filter.EventName and filter.EventMetaKey to be set.
func (pages *Pages) ConversionsByPathBreakdown(filter *Filter) ([]model.PathConversionMetaStats, error) {
	if len(filter.EventName) == 0 || len(filter.EventMetaKey) == 0 {
		return []model.PathConversionMetaStats{}, nil
	}

	filter = pages.analyzer.getFilter(filter)

	// Step 1: Get paginated paths with path-level totals (respects LIMIT/OFFSET)
	q, args := pages.buildConversionsByPathQuery(filter)
	pathStats, err := pages.store.SelectPathConversionStats(filter.Ctx, q, args...)

	if err != nil {
		return nil, err
	}

	if len(pathStats) == 0 {
		return []model.PathConversionMetaStats{}, nil
	}

	// Step 2: Get meta value breakdown for those specific paths (no pagination)
	paths := make([]string, len(pathStats))
	for i, p := range pathStats {
		paths[i] = p.Path
	}

	breakdownFilter := pages.analyzer.getFilter(filter)
	breakdownFilter.Path = nil
	breakdownFilter.AnyPath = paths
	breakdownFilter.Offset = 0
	breakdownFilter.Limit = 0

	q, args = pages.buildConversionsByPathBreakdownQuery(breakdownFilter)
	metaRows, err := pages.store.SelectPathConversionMetaStats(filter.Ctx, q, args...)

	if err != nil {
		return nil, err
	}

	// Step 3: Combine path totals with meta breakdown
	return pages.combinePathConversionMetaStats(pathStats, metaRows), nil
}

// combinePathConversionMetaStats combines path-level stats with meta value breakdown rows.
func (pages *Pages) combinePathConversionMetaStats(pathStats []model.PathConversionStats, metaRows []model.PathConversionMetaRow) []model.PathConversionMetaStats {
	// Build meta values lookup by path+hostname
	metaLookup := make(map[string][]model.PathMetaValueStats)

	for _, row := range metaRows {
		key := row.Path + "\x00" + row.Hostname
		metaLookup[key] = append(metaLookup[key], model.PathMetaValueStats{
			Value:  row.MetaValue,
			Events: row.Events,
			CR:     row.CR,
		})
	}

	// Build results maintaining path order from first query
	results := make([]model.PathConversionMetaStats, 0, len(pathStats))

	for _, path := range pathStats {
		key := path.Path + "\x00" + path.Hostname
		metaValues := metaLookup[key]

		if metaValues == nil {
			metaValues = []model.PathMetaValueStats{}
		} else {
			// Sort meta values by CR in descending order
			sort.Slice(metaValues, func(i, j int) bool {
				return metaValues[i].CR > metaValues[j].CR
			})
		}

		results = append(results, model.PathConversionMetaStats{
			Path:       path.Path,
			Hostname:   path.Hostname,
			Visitors:   path.Visitors,
			Views:      path.Views,
			Events:     path.Events,
			CR:         path.CR,
			MetaValues: metaValues,
		})
	}

	return results
}

// pathConversionQueryParts holds the reusable parts for path conversion queries.
type pathConversionQueryParts struct {
	clientClause string
	clientArgs   []any
	timeClause   string
	timeArgs     []any
	sampleClause string
	sampleFactor string
}

// buildPathConversionQueryParts builds the common query parts for path conversion queries.
func (pages *Pages) buildPathConversionQueryParts(filter *Filter) pathConversionQueryParts {
	parts := pathConversionQueryParts{}
	tz := filter.Timezone.String()

	// Build client IDs clause
	clientIdsStr, clientIdArgs := clientIdsToString(filter.ClientIDs)
	if len(clientIdArgs) > 0 {
		parts.clientClause = "client_id IN (" + clientIdsStr + ") "
		parts.clientArgs = clientIdArgs
	}

	// Build time clauses
	if !filter.From.IsZero() && !filter.To.IsZero() && filter.From.Equal(filter.To) {
		parts.timeArgs = append(parts.timeArgs, filter.From.Format("2006-01-02"))
		parts.timeClause = fmt.Sprintf("toDate(time, '%s') = toDate(?) ", tz)
	} else {
		if !filter.From.IsZero() {
			parts.timeArgs = append(parts.timeArgs, filter.From.Format("2006-01-02"))
			parts.timeClause += fmt.Sprintf("toDate(time, '%s') >= toDate(?) ", tz)
		}
		if !filter.To.IsZero() {
			if parts.timeClause != "" {
				parts.timeClause += "AND "
			}
			parts.timeArgs = append(parts.timeArgs, filter.To.Format("2006-01-02"))
			parts.timeClause += fmt.Sprintf("toDate(time, '%s') <= toDate(?) ", tz)
		}
	}

	// Build sample clause
	if filter.Sample > 0 {
		parts.sampleClause = fmt.Sprintf("SAMPLE %d ", filter.Sample)
		parts.sampleFactor = "*any(_sample_factor)"
	}

	return parts
}

// buildPathConversionPVWhere builds the WHERE clause and args for the page_view subquery.
func (pages *Pages) buildPathConversionPVWhere(filter *Filter, parts pathConversionQueryParts) (string, []any) {
	var args []any
	pvWhere := "WHERE "

	if parts.clientClause != "" {
		pvWhere += parts.clientClause
		args = append(args, parts.clientArgs...)
		if parts.timeClause != "" {
			pvWhere += "AND " + parts.timeClause
			args = append(args, parts.timeArgs...)
		}
	} else if parts.timeClause != "" {
		pvWhere += parts.timeClause
		args = append(args, parts.timeArgs...)
	}

	// Add hostname filter
	if len(filter.Hostname) > 0 {
		hostnamePlaceholders := make([]string, len(filter.Hostname))
		for i := range filter.Hostname {
			hostnamePlaceholders[i] = "?"
			args = append(args, filter.Hostname[i])
		}
		if pvWhere != "WHERE " {
			pvWhere += "AND "
		}
		pvWhere += "hostname IN (" + strings.Join(hostnamePlaceholders, ",") + ") "
	}

	// Add path filters
	if len(filter.Path) > 0 {
		pathPlaceholders := make([]string, len(filter.Path))
		for i := range filter.Path {
			pathPlaceholders[i] = "?"
			args = append(args, filter.Path[i])
		}
		if pvWhere != "WHERE " {
			pvWhere += "AND "
		}
		pvWhere += "path IN (" + strings.Join(pathPlaceholders, ",") + ") "
	} else if len(filter.PathPattern) > 0 {
		for _, pattern := range filter.PathPattern {
			if pvWhere != "WHERE " {
				pvWhere += "AND "
			}
			if strings.HasPrefix(pattern, "!") {
				args = append(args, pattern[1:])
				pvWhere += "match(path, ?) = 0 "
			} else {
				args = append(args, pattern)
				pvWhere += "match(path, ?) = 1 "
			}
		}
	} else if len(filter.AnyPath) > 0 {
		pathPlaceholders := make([]string, len(filter.AnyPath))
		for i := range filter.AnyPath {
			pathPlaceholders[i] = "?"
			args = append(args, filter.AnyPath[i])
		}
		if pvWhere != "WHERE " {
			pvWhere += "AND "
		}
		pvWhere += "path IN (" + strings.Join(pathPlaceholders, ",") + ") "
	}

	// Add search filters
	for _, search := range filter.Search {
		if search.Input == "" {
			continue
		}
		if pvWhere != "WHERE " {
			pvWhere += "AND "
		}
		if search.Field.Name == FieldPath.Name || search.Field.Name == FieldHostname.Name {
			if strings.HasPrefix(search.Input, "!") {
				args = append(args, fmt.Sprintf("%%%s%%", search.Input[1:]))
				pvWhere += fmt.Sprintf("ilike(%s, ?) = 0 ", search.Field.Name)
			} else {
				args = append(args, fmt.Sprintf("%%%s%%", search.Input))
				pvWhere += fmt.Sprintf("ilike(%s, ?) = 1 ", search.Field.Name)
			}
		}
	}

	return pvWhere, args
}

// buildPathConversionEvWhere builds the WHERE clause and args for the event subquery.
func (pages *Pages) buildPathConversionEvWhere(filter *Filter, parts pathConversionQueryParts, includeMetaKey bool) (string, []any) {
	var args []any
	evWhere := "WHERE "

	if parts.clientClause != "" {
		evWhere += parts.clientClause
		args = append(args, parts.clientArgs...)
		if parts.timeClause != "" {
			evWhere += "AND " + parts.timeClause
			args = append(args, parts.timeArgs...)
		}
	} else if parts.timeClause != "" {
		evWhere += parts.timeClause
		args = append(args, parts.timeArgs...)
	}

	// Add event_name filter
	eventPlaceholders := make([]string, len(filter.EventName))
	for i := range filter.EventName {
		eventPlaceholders[i] = "?"
		args = append(args, filter.EventName[i])
	}
	if evWhere != "WHERE " {
		evWhere += "AND "
	}
	evWhere += "event_name IN (" + strings.Join(eventPlaceholders, ",") + ") "

	// Add event meta key filter for breakdown queries
	// Uses 'k' alias from ARRAY JOIN event_meta_keys AS k, event_meta_values AS v
	if includeMetaKey && len(filter.EventMetaKey) > 0 {
		metaKeyPlaceholders := make([]string, len(filter.EventMetaKey))
		for i := range filter.EventMetaKey {
			metaKeyPlaceholders[i] = "?"
			args = append(args, filter.EventMetaKey[i])
		}
		if evWhere != "WHERE " {
			evWhere += "AND "
		}
		evWhere += "k IN (" + strings.Join(metaKeyPlaceholders, ",") + ") "
	}

	// Add event meta key-value filters (for filtering by specific meta values)
	// Uses event_meta_values[indexOf(event_meta_keys, key)] = value pattern
	for key, value := range filter.EventMeta {
		if evWhere != "WHERE " {
			evWhere += "AND "
		}
		args = append(args, key, value)
		evWhere += "event_meta_values[indexOf(event_meta_keys, ?)] = ? "
	}

	// Add path filters
	if len(filter.Path) > 0 {
		pathPlaceholders := make([]string, len(filter.Path))
		for i := range filter.Path {
			pathPlaceholders[i] = "?"
			args = append(args, filter.Path[i])
		}
		if evWhere != "WHERE " {
			evWhere += "AND "
		}
		evWhere += "path IN (" + strings.Join(pathPlaceholders, ",") + ") "
	} else if len(filter.PathPattern) > 0 {
		for _, pattern := range filter.PathPattern {
			if evWhere != "WHERE " {
				evWhere += "AND "
			}
			if strings.HasPrefix(pattern, "!") {
				args = append(args, pattern[1:])
				evWhere += "match(path, ?) = 0 "
			} else {
				args = append(args, pattern)
				evWhere += "match(path, ?) = 1 "
			}
		}
	} else if len(filter.AnyPath) > 0 {
		pathPlaceholders := make([]string, len(filter.AnyPath))
		for i := range filter.AnyPath {
			pathPlaceholders[i] = "?"
			args = append(args, filter.AnyPath[i])
		}
		if evWhere != "WHERE " {
			evWhere += "AND "
		}
		evWhere += "path IN (" + strings.Join(pathPlaceholders, ",") + ") "
	}

	// Add search filters
	for _, search := range filter.Search {
		if search.Input == "" {
			continue
		}
		if evWhere != "WHERE " {
			evWhere += "AND "
		}
		if search.Field.Name == FieldPath.Name {
			if strings.HasPrefix(search.Input, "!") {
				args = append(args, fmt.Sprintf("%%%s%%", search.Input[1:]))
				evWhere += "ilike(path, ?) = 0 "
			} else {
				args = append(args, fmt.Sprintf("%%%s%%", search.Input))
				evWhere += "ilike(path, ?) = 1 "
			}
		}
	}

	return evWhere, args
}

func (pages *Pages) buildConversionsByPathQuery(filter *Filter) (string, []any) {
	parts := pages.buildPathConversionQueryParts(filter)

	pvWhere, pvArgs := pages.buildPathConversionPVWhere(filter, parts)
	evWhere, evArgs := pages.buildPathConversionEvWhere(filter, parts, false)

	args := append(pvArgs, evArgs...)

	// Build the query
	query := fmt.Sprintf(`SELECT 
		pv.path, 
		pv.hostname, 
		toUInt64(greatest(pv.visitors%s, 0)) AS visitors, 
		toUInt64(greatest(pv.views%s, 0)) AS views, 
		toUInt64(greatest(coalesce(e.events, 0)%s, 0)) AS events,
		if(pv.visitors > 0, coalesce(e.event_visitors, 0) / pv.visitors, 0) AS cr
	FROM (
		SELECT path, hostname, uniq(visitor_id) AS visitors, count(*) AS views
		FROM "page_view" %s
		%s
		GROUP BY path, hostname
	) pv
	LEFT JOIN (
		SELECT path, count(*) AS events, uniq(visitor_id) AS event_visitors
		FROM "event" %s
		%s
		GROUP BY path
	) e ON pv.path = e.path`,
		parts.sampleFactor, parts.sampleFactor, parts.sampleFactor,
		parts.sampleClause, pvWhere,
		parts.sampleClause, evWhere)

	// When filtering by EventMeta, only show paths that have matching events
	if len(filter.EventMeta) > 0 {
		query += " WHERE e.events > 0"
	}

	query += " ORDER BY cr DESC, visitors DESC, pv.path ASC"

	// Add pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	return query, args
}

func (pages *Pages) buildConversionsByPathBreakdownQuery(filter *Filter) (string, []any) {
	parts := pages.buildPathConversionQueryParts(filter)

	pvWhere, pvArgs := pages.buildPathConversionPVWhere(filter, parts)
	evWhere, evArgs := pages.buildPathConversionEvWhere(filter, parts, true)

	args := append(pvArgs, evArgs...)

	// Build the query with meta value breakdown
	// Uses ARRAY JOIN with aliases: event_meta_keys AS k, event_meta_values AS v
	query := fmt.Sprintf(`SELECT 
		pv.path, 
		pv.hostname, 
		toUInt64(greatest(pv.visitors%s, 0)) AS visitors, 
		toUInt64(greatest(pv.views%s, 0)) AS views, 
		toUInt64(greatest(coalesce(e.events, 0)%s, 0)) AS events,
		if(pv.visitors > 0, coalesce(e.events, 0) / pv.visitors, 0) AS cr,
		e.meta_value
	FROM (
		SELECT path, hostname, uniq(visitor_id) AS visitors, count(*) AS views
		FROM "page_view" %s
		%s
		GROUP BY path, hostname
	) pv
	LEFT JOIN (
		SELECT path, v AS meta_value, count(*) AS events
		FROM "event" %s
		ARRAY JOIN event_meta_keys AS k, event_meta_values AS v
		%s
		GROUP BY path, v
	) e ON pv.path = e.path
	WHERE e.meta_value IS NOT NULL AND e.meta_value != ''
	ORDER BY pv.path ASC, e.meta_value ASC`,
		parts.sampleFactor, parts.sampleFactor, parts.sampleFactor,
		parts.sampleClause, pvWhere,
		parts.sampleClause, evWhere)

	return query, args
}
