package mcpgrafana

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"strings"
// 	"github.com/mark3labs/mcp-go/mcp"
// )

// // Struct definition
// type PromptTemplate struct {
// 	Name        string
// 	Description string
// 	Template    string
// 	Variables   []string
// }

// // Exported template map
// var PromptTemplates = map[string]PromptTemplate{
// 	"grafana_cpu_throttle_analysis": {
// 		Name:        "Kubernetes Pod CPU Throttle Spike Detection",
// 		Description: "Extract and analyze the CPU Throttle Percentage Prometheus query from a Grafana dashboard panel, resolve variables, prompt for a time range, and detect spikes.",
// 		Template: `
// **Task Context**
// From the Grafana dashboard titled "{{.dashboard_title}}" (UID: {{.dashboard_uid}}), extract the Prometheus query for CPU Throttle Percentage from the appropriate panel.

// **Datasource**
// Prometheus datasource UID: {{.datasource_uid}}

// **Steps**
// 1. Extract the Prometheus query for CPU Throttle Percentage.
// 2. If the query contains variables (e.g., $kubernetes_pod_name, $namespace), identify and prompt for values.
// 3. Prompt for a time range (e.g., now-1h to now or ISO timestamps).
// 4. Substitute all variables and the time range into the query.
// 5. Execute the query against the specified Prometheus datasource.
// 6. Analyze the results for spikes using:
//    - relative_increase = (current - previous) / max(previous, 1e-6)
//    - Mark as spike if relative_increase >= 0.5 and absolute increase >= 10 percentage points.

// **Output**
// - Final substituted Prometheus query.
// - Tabular time series data.
// - If a spike is detected:
//   - Timestamps of spike start and peak.
//   - Values before and after the spike.
//   - Relative and absolute increases.

// **Dashboard Info**
// - Title: {{.dashboard_title}}
// - UID: {{.dashboard_uid}}
// - Datasource UID: {{.datasource_uid}}
// `,
// 		Variables: []string{"dashboard_title", "dashboard_uid", "datasource_uid"},
// 	},
// }

// // Exported handler
// func HandleGrafanaPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
// 	templateName := req.Params.Arguments["template"]
// 	variablesJSON := req.Params.Arguments["variables"]

// 	var variables map[string]interface{}
// 	if err := json.Unmarshal([]byte(variablesJSON), &variables); err != nil {
// 		return nil, fmt.Errorf("failed to parse 'variables' JSON: %v", err)
// 	}

// 	template, ok := PromptTemplates[templateName]
// 	if !ok {
// 		return nil, fmt.Errorf("unknown template: %s", templateName)
// 	}

// 	content := template.Template
// 	for _, variable := range template.Variables {
// 		if value, exists := variables[variable]; exists {
// 			placeholder := fmt.Sprintf("{{.%s}}", variable)
// 			content = strings.ReplaceAll(content, placeholder, fmt.Sprintf("%v", value))
// 		}
// 	}

// 	return &mcp.GetPromptResult{
// 		Description: template.Description,
// 		Messages: []mcp.PromptMessage{
// 			{
// 				Role:    "user",
// 				Content: mcp.NewTextContent(content),
// 			},
// 		},
// 	}, nil
// }

// // Context functions - you may need to implement these based on your requirements
// // These are placeholders and may not exist in your actual implementation
// var ComposedStdioContextFunc func(bool) func(context.Context) context.Context
// var ComposedHTTPContextFunc func(bool) func(context.Context) context.Context