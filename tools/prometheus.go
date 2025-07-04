package tools

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	
	mcpgrafana "mcp-grafana-local"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

var (
	matchTypeMap = map[string]labels.MatchType{
		"":   labels.MatchEqual,
		"=":  labels.MatchEqual,
		"!=": labels.MatchNotEqual,
		"=~": labels.MatchRegexp,
		"!~": labels.MatchNotRegexp,
	}
)

func promClientFromContext(ctx context.Context, uid string) (promv1.API, error) {
	// First check if the datasource exists
	_, err := getDatasourceByUID(ctx, GetDatasourceByUIDParams{UID: uid})
	if err != nil {
		return nil, err
	}

	var (
		grafanaURL             = mcpgrafana.GrafanaURLFromContext(ctx)
		apiKey                 = mcpgrafana.GrafanaAPIKeyFromContext(ctx)
		accessToken, userToken = mcpgrafana.OnBehalfOfAuthFromContext(ctx)
	)
	url := fmt.Sprintf("%s/api/datasources/proxy/uid/%s", strings.TrimRight(grafanaURL, "/"), uid)
	rt := api.DefaultRoundTripper
	if accessToken != "" && userToken != "" {
		rt = config.NewHeadersRoundTripper(&config.Headers{
			Headers: map[string]config.Header{
				"X-Access-Token": config.Header{
					Secrets: []config.Secret{config.Secret(accessToken)},
				},
				"X-Grafana-Id": config.Header{
					Secrets: []config.Secret{config.Secret(userToken)},
				},
			},
		}, rt)
	} else if apiKey != "" {
		rt = config.NewAuthorizationCredentialsRoundTripper(
			"Bearer", config.NewInlineSecret(apiKey), rt,
		)
	}
	c, err := api.NewClient(api.Config{
		Address:      url,
		RoundTripper: rt,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus client: %w", err)
	}

	return promv1.NewAPI(c), nil
}

type ListPrometheusMetricMetadataParams struct {
	DatasourceUID  string `json:"datasourceUid" jsonschema:"required,description=The UID of the datasource to query"`
	Limit          int    `json:"limit" jsonschema:"description=The maximum number of metrics to return"`
	LimitPerMetric int    `json:"limitPerMetric" jsonschema:"description=The maximum number of metrics to return per metric"`
	Metric         string `json:"metric" jsonschema:"description=The metric to query"`
}

func listPrometheusMetricMetadata(ctx context.Context, args ListPrometheusMetricMetadataParams) (map[string][]promv1.Metadata, error) {
	promClient, err := promClientFromContext(ctx, args.DatasourceUID)
	if err != nil {
		return nil, fmt.Errorf("getting Prometheus client: %w", err)
	}

	limit := args.Limit
	if limit == 0 {
		limit = 10
	}

	metadata, err := promClient.Metadata(ctx, args.Metric, fmt.Sprintf("%d", limit))
	if err != nil {
		return nil, fmt.Errorf("listing Prometheus metric metadata: %w", err)
	}
	return metadata, nil
}

var ListPrometheusMetricMetadata = mcpgrafana.MustTool(
	"list_prometheus_metric_metadata",
	"List Prometheus metric metadata. Returns metadata about metrics currently scraped from targets. Note: This endpoint is experimental.",
	listPrometheusMetricMetadata,
	mcp.WithTitleAnnotation("List Prometheus metric metadata"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

// QueryPrometheusParams allows 'from' and 'to' to be RFC3339, epoch ms string, or relative time ("now-5m", "now-1h").
type QueryPrometheusParams struct {
    DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the datasource to query"`
    Expr          string `json:"expr" jsonschema:"required,description=The PromQL expression to query"`
    From          string `json:"from" jsonschema:"required,description=Start time (RFC3339, epoch ms, or relative to now like 'now-5m')"`
    To            string `json:"to" jsonschema:"required,description=End time (RFC3339, epoch ms, or relative to now like 'now')"`
    StepSeconds   int    `json:"stepSeconds,omitempty" jsonschema:"description=Time series step size in seconds. Required if queryType is 'range'"`
    QueryType     string `json:"queryType,omitempty" jsonschema:"description=The type of query to use. Either 'range' or 'instant'"`
	Variables     map[string]string `json:"variables,omitempty"` 
}


// parseUserTime handles RFC3339, epoch ms, or relative ("now-5m", "now-1h").
func parseUserTime(input string, now time.Time) (time.Time, error) {
    input = strings.TrimSpace(input)
    if input == "" {
        return time.Time{}, fmt.Errorf("empty time string")
    }
    if input == "now" {
        return now, nil
    }
    // Try epoch ms
    if ms, err := strconv.ParseInt(input, 10, 64); err == nil && ms > 1000000000000 {
        return time.Unix(0, ms*int64(time.Millisecond)), nil
    }
    // Try RFC3339
    if t, err := time.Parse(time.RFC3339, input); err == nil {
        return t, nil
    }
    // Try relative: now-5m, now-1h, now-2h30m, now-2d
    relRe := regexp.MustCompile(`^now-(\d+d)?(\d+h)?(\d+m)?(\d+s)?$`)
    if matches := relRe.FindStringSubmatch(input); matches != nil {
        durStr := ""
        for i := 1; i <= 4; i++ {
            if matches[i] != "" {
                durStr += matches[i]
            }
        }
        if d, err := parseDurationWithDays(durStr); err == nil {
            return now.Add(-d), nil
        }
    }
    return time.Time{}, fmt.Errorf("invalid time format: %s", input)
}

// parseDurationWithDays supports days in duration (e.g. "2d5h30m").
func parseDurationWithDays(s string) (time.Duration, error) {
    // Replace "d" with hours for time.ParseDuration
    re := regexp.MustCompile(`(\d+)d`)
    s = re.ReplaceAllStringFunc(s, func(dayStr string) string {
        days, _ := strconv.Atoi(strings.TrimSuffix(dayStr, "d"))
        return fmt.Sprintf("%dh", days*24)
    })
    return time.ParseDuration(s)
}

type UnresolvedVariablesError struct {
	Missing []string
}

func (e *UnresolvedVariablesError) Error() string {
	return fmt.Sprintf("unresolved variables in query: %v", e.Missing, "please prompt user to provide variable values and resolve them")
}


func queryPrometheus(ctx context.Context, args QueryPrometheusParams) (model.Value, error) {
	var variableRegex = regexp.MustCompile(`\$\w+`)
    promClient, err := promClientFromContext(ctx, args.DatasourceUID)
    if err != nil {
        return nil, fmt.Errorf("getting Prometheus client: %w", err)
    }

    queryType := args.QueryType
    if queryType == "" {
        queryType = "range"
    }

	queryType = "range"

	expr := args.Expr
	for name, value := range args.Variables {
		expr = strings.ReplaceAll(expr, "$"+name, value)
	}

	unresolved := variableRegex.FindAllString(expr, -1)
	if len(unresolved) > 0 {
		return nil, &UnresolvedVariablesError{Missing: unresolved}
	}


    now := time.Now()
    var fromTime, toTime time.Time

    fromTime, err = parseUserTime(args.From, now)
    if err != nil {
        return nil, fmt.Errorf("parsing from time: %w", err)
    }

    if queryType == "range" {
        toTime, err = parseUserTime(args.To, now)
        if err != nil {
            return nil, fmt.Errorf("parsing to time: %w", err)
        }
        // Declare and initialize stepSeconds here.
        // You can choose a default, e.g., 60 seconds.
        stepSeconds := 30
        step := time.Duration(stepSeconds) * time.Second
        result, _, err := promClient.QueryRange(ctx, expr, promv1.Range{
            Start: fromTime,
            End:   toTime,
            Step:  step,
        })
        if err != nil {
            return nil, fmt.Errorf("querying Prometheus range: %w", err)
        }
        return result, nil
    } else if queryType == "instant" {
        result, _, err := promClient.Query(ctx, expr, fromTime)
        if err != nil {
            return nil, fmt.Errorf("querying Prometheus instant: %w", err)
        }
        return result, nil
    }

    return nil, fmt.Errorf("invalid query type: %s", queryType)
}

var QueryPrometheus = mcpgrafana.MustTool(
	"query_prometheus",
	"Query Prometheus using a PromQL expression. Supports both instant queries (at a single point in time) and range queries (over a time range). Time can be specified either in RFC3339 format or as relative time expressions like 'now', 'now-1h', 'now-30m', etc.",
	queryPrometheus,
	mcp.WithTitleAnnotation("Query Prometheus metrics"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

type ListPrometheusMetricNamesParams struct {
	DatasourceUID string `json:"datasourceUid" jsonschema:"required,description=The UID of the datasource to query"`
	Regex         string `json:"regex" jsonschema:"description=The regex to match against the metric names"`
	Limit         int    `json:"limit,omitempty" jsonschema:"description=The maximum number of results to return"`
	Page          int    `json:"page,omitempty" jsonschema:"description=The page number to return"`
}

func listPrometheusMetricNames(ctx context.Context, args ListPrometheusMetricNamesParams) ([]string, error) {
	promClient, err := promClientFromContext(ctx, args.DatasourceUID)
	if err != nil {
		return nil, fmt.Errorf("getting Prometheus client: %w", err)
	}

	limit := args.Limit
	if limit == 0 {
		limit = 10
	}

	page := args.Page
	if page == 0 {
		page = 1
	}

	// Get all metric names by querying for __name__ label values
	labelValues, _, err := promClient.LabelValues(ctx, "__name__", nil, time.Time{}, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("listing Prometheus metric names: %w", err)
	}

	// Filter by regex if provided
	matches := []string{}
	if args.Regex != "" {
		re, err := regexp.Compile(args.Regex)
		if err != nil {
			return nil, fmt.Errorf("compiling regex: %w", err)
		}
		for _, val := range labelValues {
			if re.MatchString(string(val)) {
				matches = append(matches, string(val))
			}
		}
	} else {
		for _, val := range labelValues {
			matches = append(matches, string(val))
		}
	}

	// Apply pagination
	start := (page - 1) * limit
	end := start + limit
	if start >= len(matches) {
		matches = []string{}
	} else if end > len(matches) {
		matches = matches[start:]
	} else {
		matches = matches[start:end]
	}

	return matches, nil
}

var ListPrometheusMetricNames = mcpgrafana.MustTool(
	"list_prometheus_metric_names",
	"List metric names in a Prometheus datasource. Retrieves all metric names and then filters them locally using the provided regex. Supports pagination.",
	listPrometheusMetricNames,
	mcp.WithTitleAnnotation("List Prometheus metric names"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

type LabelMatcher struct {
	Name  string `json:"name" jsonschema:"required,description=The name of the label to match against"`
	Value string `json:"value" jsonschema:"required,description=The value to match against"`
	Type  string `json:"type" jsonschema:"required,description=One of the '=' or '!=' or '=~' or '!~'"`
}

type Selector struct {
	Filters []LabelMatcher `json:"filters"`
}

func (s Selector) String() string {
	b := strings.Builder{}
	b.WriteRune('{')
	for i, f := range s.Filters {
		if f.Type == "" {
			f.Type = "="
		}
		b.WriteString(fmt.Sprintf(`%s%s'%s'`, f.Name, f.Type, f.Value))
		if i < len(s.Filters)-1 {
			b.WriteString(", ")
		}
	}
	b.WriteRune('}')
	return b.String()
}

// Matches runs the matchers against the given labels and returns whether they match the selector.
func (s Selector) Matches(lbls labels.Labels) (bool, error) {
	matchers := make(labels.Selector, 0, len(s.Filters))

	for _, filter := range s.Filters {
		matchType, ok := matchTypeMap[filter.Type]
		if !ok {
			return false, fmt.Errorf("invalid matcher type: %s", filter.Type)
		}

		matcher, err := labels.NewMatcher(matchType, filter.Name, filter.Value)
		if err != nil {
			return false, fmt.Errorf("creating matcher: %w", err)
		}

		matchers = append(matchers, matcher)
	}

	return matchers.Matches(lbls), nil
}

type ListPrometheusLabelNamesParams struct {
	DatasourceUID string     `json:"datasourceUid" jsonschema:"required,description=The UID of the datasource to query"`
	Matches       []Selector `json:"matches,omitempty" jsonschema:"description=Optionally\\, a list of label matchers to filter the results by"`
	StartRFC3339  string     `json:"startRfc3339,omitempty" jsonschema:"description=Optionally\\, the start time of the time range to filter the results by"`
	EndRFC3339    string     `json:"endRfc3339,omitempty" jsonschema:"description=Optionally\\, the end time of the time range to filter the results by"`
	Limit         int        `json:"limit,omitempty" jsonschema:"description=Optionally\\, the maximum number of results to return"`
}

func listPrometheusLabelNames(ctx context.Context, args ListPrometheusLabelNamesParams) ([]string, error) {
	promClient, err := promClientFromContext(ctx, args.DatasourceUID)
	if err != nil {
		return nil, fmt.Errorf("getting Prometheus client: %w", err)
	}

	limit := args.Limit
	if limit == 0 {
		limit = 100
	}

	var startTime, endTime time.Time
	if args.StartRFC3339 != "" {
		if startTime, err = time.Parse(time.RFC3339, args.StartRFC3339); err != nil {
			return nil, fmt.Errorf("parsing start time: %w", err)
		}
	}
	if args.EndRFC3339 != "" {
		if endTime, err = time.Parse(time.RFC3339, args.EndRFC3339); err != nil {
			return nil, fmt.Errorf("parsing end time: %w", err)
		}
	}

	var matchers []string
	for _, m := range args.Matches {
		matchers = append(matchers, m.String())
	}

	labelNames, _, err := promClient.LabelNames(ctx, matchers, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("listing Prometheus label names: %w", err)
	}

	// Apply limit
	if len(labelNames) > limit {
		labelNames = labelNames[:limit]
	}

	return labelNames, nil
}

var ListPrometheusLabelNames = mcpgrafana.MustTool(
	"list_prometheus_label_names",
	"List label names in a Prometheus datasource. Allows filtering by series selectors and time range.",
	listPrometheusLabelNames,
	mcp.WithTitleAnnotation("List Prometheus label names"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

type ListPrometheusLabelValuesParams struct {
	DatasourceUID string     `json:"datasourceUid" jsonschema:"required,description=The UID of the datasource to query"`
	LabelName     string     `json:"labelName" jsonschema:"required,description=The name of the label to query"`
	Matches       []Selector `json:"matches,omitempty" jsonschema:"description=Optionally\\, a list of selectors to filter the results by"`
	StartRFC3339  string     `json:"startRfc3339,omitempty" jsonschema:"description=Optionally\\, the start time of the query"`
	EndRFC3339    string     `json:"endRfc3339,omitempty" jsonschema:"description=Optionally\\, the end time of the query"`
	Limit         int        `json:"limit,omitempty" jsonschema:"description=Optionally\\, the maximum number of results to return"`
}

func listPrometheusLabelValues(ctx context.Context, args ListPrometheusLabelValuesParams) (model.LabelValues, error) {
	promClient, err := promClientFromContext(ctx, args.DatasourceUID)
	if err != nil {
		return nil, fmt.Errorf("getting Prometheus client: %w", err)
	}

	limit := args.Limit
	if limit == 0 {
		limit = 100
	}

	var startTime, endTime time.Time
	if args.StartRFC3339 != "" {
		if startTime, err = time.Parse(time.RFC3339, args.StartRFC3339); err != nil {
			return nil, fmt.Errorf("parsing start time: %w", err)
		}
	}
	if args.EndRFC3339 != "" {
		if endTime, err = time.Parse(time.RFC3339, args.EndRFC3339); err != nil {
			return nil, fmt.Errorf("parsing end time: %w", err)
		}
	}

	var matchers []string
	for _, m := range args.Matches {
		matchers = append(matchers, m.String())
	}

	labelValues, _, err := promClient.LabelValues(ctx, args.LabelName, matchers, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("listing Prometheus label values: %w", err)
	}

	// Apply limit
	if len(labelValues) > limit {
		labelValues = labelValues[:limit]
	}

	return labelValues, nil
}

var ListPrometheusLabelValues = mcpgrafana.MustTool(
	"list_prometheus_label_values",
	"Get the values for a specific label name in Prometheus. Allows filtering by series selectors and time range.",
	listPrometheusLabelValues,
	mcp.WithTitleAnnotation("List Prometheus label values"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

func AddPrometheusTools(mcp *server.MCPServer) {
	ListPrometheusMetricMetadata.Register(mcp)
	QueryPrometheus.Register(mcp)
	ListPrometheusMetricNames.Register(mcp)
	ListPrometheusLabelNames.Register(mcp)
	ListPrometheusLabelValues.Register(mcp)
}
