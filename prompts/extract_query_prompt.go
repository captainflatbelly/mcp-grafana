package prompts

import (
	"context"
	"fmt"
	"bytes"
	"text/template"

	"github.com/mark3labs/mcp-go/mcp"
)

func extractQueryPrompt() mcp.Prompt {
    return mcp.NewPrompt(
        "extract_promql_from_grafana",
        mcp.WithPromptDescription("Extract and execute a Prometheus query from a Grafana dashboard"),
        mcp.WithArgument("dashboardUID", mcp.ArgumentDescription("UID of the Grafana dashboard"), mcp.RequiredArgument()),
		 mcp.WithArgument("metric", mcp.ArgumentDescription("Name of the Prometheus metric"), mcp.RequiredArgument()),
        mcp.WithArgument("panelTitle", mcp.ArgumentDescription("Name of the panel"), mcp.RequiredArgument()),
        mcp.WithArgument("datasourceUID", mcp.ArgumentDescription("UID of the Prometheus datasource"), mcp.RequiredArgument()),
    )
}

func handleExtractQuery(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	if args == nil {
		return nil, fmt.Errorf("missing prompt arguments")
	}

	dashboardUID, ok := args["dashboardUID"]
	if !ok || dashboardUID == "" {
		return nil, fmt.Errorf("argument 'dashboardUID' is required")
	}
	metric, ok := args["metric"]
	if !ok || metric == "" {
		return nil, fmt.Errorf("argument 'metric' is required")
	}
	prometheusUID, ok := args["datasourceUID"]
	if !ok || prometheusUID == "" {
		return nil, fmt.Errorf("argument 'datasourceUID' is required")
	}
	panelTitle, ok := args["panelTitle"]
	if !ok || panelTitle == "" {
		return nil, fmt.Errorf("argument 'panelTitle' is required")
	}


	const promptTemplate = `
		From the Grafana dashboard with Dashboard UID = ({{.DashboardUID}}), extract the Prometheus query used to calculate {{.Metric}}.

		Use the Prometheus datasource UID: {{.DatasourceUID}}.

		Task Steps:

		Query Extraction  
		Locate the exact Prometheus query from the dashboard panel for {{.Metric}} under the {{.PanelTitle}} panel.

		Variable Resolution  
		If the query contains any template variables:  
		- Identify and list them.  
		- Prompt me to provide concrete values for each unresolved variable.  
		- Substitute the values into the query.

		Time Range Input  
		- Prompt me to enter a time range (for example: now-1h to now or ISO timestamps). Do not execute the query until the time range is specified.

		Query Execution  
		- Once all variables are resolved and the time range is defined, execute the query against the Prometheus datasource and retrieve the time series data as (timestamp, throttle percentage) pairs.

		Spike Detection Algorithm  
		- Analyze the data for spikes using the following method:  
		For each pair of adjacent throttle percentage values:  
		- Calculate the relative increase using the formula:  
			(current - previous) / max(previous, 1e-6)  
		- If the relative increase is greater than or equal to 0.5 (i.e., 50%) and the absolute increase is greater than or equal to 10 percentage points, mark it as a spike.

		Output Requirements:  
		- Return the final substituted Prometheus query.  
		- Display the time series data in tabular format.  
		- If a spike is detected:  
		- Show the timestamps where the spike started and peaked.  
		- Show the values before and after the spike.  
		- Show the relative and absolute increase between the values.`

	// Prepare data for substitution
	data := map[string]string{
		"DashboardUID":  dashboardUID,
		"Metric":        metric,
		"DatasourceUID": prometheusUID,
		"PanelTitle":    panelTitle,
	}

	tpl, err := template.New("queryPrompt").Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("error parsing prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("error executing prompt template: %w", err)
	}

	// Return the prompt
	return &mcp.GetPromptResult{
		Description: "Ready to execute query extraction",
		Messages: []mcp.PromptMessage{
			{
				Role:    "user",
				Content: mcp.NewTextContent(buf.String()),
			},
		},
	}, nil
}
