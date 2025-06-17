package tools

import (
	"context"
	"fmt"
	"regexp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"log/slog"

	"github.com/grafana/grafana-openapi-client-go/models"
	mcpgrafana "mcp-grafana-local"
)

type GetDashboardByUIDParams struct {
	UID string `json:"uid" jsonschema:"required,description=The UID of the dashboard"`
}

func getDashboardByUID(ctx context.Context, args GetDashboardByUIDParams) (*models.DashboardFullWithMeta, error) {
	c := mcpgrafana.GrafanaClientFromContext(ctx)
	dashboard, err := c.Dashboards.GetDashboardByUID(args.UID)
	if err != nil {
		return nil, fmt.Errorf("get dashboard by uid %s: %w", args.UID, err)
	}
	return dashboard.Payload, nil
}

type UpdateDashboardParams struct {
	Dashboard map[string]interface{} `json:"dashboard" jsonschema:"required,description=The full dashboard JSON"`
	FolderUID string                 `json:"folderUid" jsonschema:"optional,description=The UID of the dashboard's folder"`
	Message   string                 `json:"message" jsonschema:"optional,description=Set a commit message for the version history"`
	Overwrite bool                   `json:"overwrite" jsonschema:"optional,description=Overwrite the dashboard if it exists. Otherwise create one"`
	UserID    int64                  `json:"userId" jsonschema:"optional,ID of the user making the change"`
}

// updateDashboard can be used to save an existing dashboard, or create a new one.
// DISCLAIMER: Large-sized dashboard JSON can exhaust context windows. We will
// implement features that address this in https://github.com/grafana/mcp-grafana/issues/101.
func updateDashboard(ctx context.Context, args UpdateDashboardParams) (*models.PostDashboardOKBody, error) {
	c := mcpgrafana.GrafanaClientFromContext(ctx)
	cmd := &models.SaveDashboardCommand{
		Dashboard: args.Dashboard,
		FolderUID: args.FolderUID,
		Message:   args.Message,
		Overwrite: args.Overwrite,
		UserID:    args.UserID,
	}
	dashboard, err := c.Dashboards.PostDashboard(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to save dashboard: %w", err)
	}
	return dashboard.Payload, nil
}

var GetDashboardByUID = mcpgrafana.MustTool(
	"get_dashboard_by_uid",
	"Retrieves the complete dashboard, including panels, variables, and settings, for a specific dashboard identified by its UID.",
	getDashboardByUID,
	mcp.WithTitleAnnotation("Get dashboard details"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

var UpdateDashboard = mcpgrafana.MustTool(
	"update_dashboard",
	"Create or update a dashboard",
	updateDashboard,
	mcp.WithTitleAnnotation("Create or update dashboard"),
	mcp.WithDestructiveHintAnnotation(true),
)

type DashboardPanelQueriesParams struct {
	UID string `json:"uid" jsonschema:"required,description=The UID of the dashboard"`
}

type datasourceInfo struct {
	UID  string `json:"uid"`
	Type string `json:"type"`
}

type panelQuery struct {
	Title      string         `json:"title"`
	Query      string         `json:"query"`
	Datasource datasourceInfo `json:"datasource"`
	Variables  []string       `json:"variables"`
}

func GetDashboardPanelQueriesTool(ctx context.Context, args DashboardPanelQueriesParams) ([]panelQuery, error) {
	var variableRegex = regexp.MustCompile(`\$\w+`)

	

	// Recursive function to extract queries from any level of panel nesting
	var extractQueries func(panels []any) []panelQuery
	extractQueries = func(panels []any) []panelQuery {
		var result []panelQuery

		for _, p := range panels {
			panel, ok := p.(map[string]any)
			if !ok {
				continue
			}

			// If this is a row panel with nested panels in "collapsed"
			if collapsed, ok := panel["collapsed"].([]any); ok {
				result = append(result, extractQueries(collapsed)...)
			}

			// Some nested dashboards use "panels" as a key within a panel (rare)
			if innerPanels, ok := panel["panels"].([]any); ok {
				result = append(result, extractQueries(innerPanels)...)
			}

			title, _ := panel["title"].(string)

			// Handle datasource
			var dsInfo datasourceInfo
			if dsField, dsExists := panel["datasource"]; dsExists && dsField != nil {
				if dsMap, ok := dsField.(map[string]any); ok {
					if uid, ok := dsMap["uid"].(string); ok {
						dsInfo.UID = uid
					}
					if dsType, ok := dsMap["type"].(string); ok {
						dsInfo.Type = dsType
					}
				}
			}

			// Extract queries
			targets, ok := panel["targets"].([]any)
			if !ok {
				continue
			}
			for _, t := range targets {
				target, ok := t.(map[string]any)
				if !ok {
					continue
				}
				expr, _ := target["expr"].(string)
				if expr != "" {
					rawVars := variableRegex.FindAllString(expr, -1)
					var uniqueVars []string
					seen := make(map[string]bool)
					for _, v := range rawVars {
						name := v[1:]
						if !seen[name] {
							seen[name] = true
							uniqueVars = append(uniqueVars, name)
						}
					}

					result = append(result, panelQuery{
						Title:      title,
						Query:      expr,
						Datasource: dsInfo,
						Variables:  uniqueVars,
					})
				}
			}
		}

		return result
	}

	// Load the dashboard
	dashboard, err := getDashboardByUID(ctx, GetDashboardByUIDParams(args))
	if err != nil {
		return nil, fmt.Errorf("get dashboard by uid: %w", err)
	}

	db, ok := dashboard.Dashboard.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("dashboard is not a JSON object")
	}

	panels, ok := db["panels"].([]any)
	if !ok {
		return nil, fmt.Errorf("panels is not a JSON array")
	}

	// Extract all queries recursively
	return extractQueries(panels), nil
}


var GetDashboardPanelQueries = mcpgrafana.MustTool(
	"get_dashboard_panel_queries",
	"Get the title, query string, and datasource information for each panel in a dashboard. The datasource is an object with fields `uid` (which may be a concrete UID or a template variable like \"$datasource\") and `type`. If the datasource UID is a template variable, it won't be usable directly for queries. Returns an array of objects, each representing a panel, with fields: title, query, and datasource (an object with uid and type).",
	GetDashboardPanelQueriesTool,
	mcp.WithTitleAnnotation("Get dashboard panel queries"),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithReadOnlyHintAnnotation(true),
)

func AddDashboardTools(mcp *server.MCPServer) {
	slog.Info("Registering dashboard tools")
	GetDashboardByUID.Register(mcp)
	UpdateDashboard.Register(mcp)
	GetDashboardPanelQueries.Register(mcp)
}
