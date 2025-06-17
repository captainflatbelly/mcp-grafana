package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"io/ioutil"



    "mcp-grafana-local/prompts"
	mcpgrafana "mcp-grafana-local"
	"mcp-grafana-local/tools"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func maybeAddTools(s *server.MCPServer, tf func(*server.MCPServer), enabledTools []string, disable bool, category string) {
	if !slices.Contains(enabledTools, category) {
		slog.Debug("Not enabling tools", "category", category)
		return
	}
	if disable {
		slog.Info("Disabling tools", "category", category)
		return
	}
	slog.Debug("Enabling tools", "category", category)
	tf(s)
}

// disabledTools indicates whether each category of tools should be disabled.
type disabledTools struct {
	enabledTools string

	search, datasource, incident,
	prometheus, loki, alerting,
	dashboard, oncall, asserts, sift, admin, file bool
}

// Configuration for the Grafana client.
type grafanaConfig struct {
	// Whether to enable debug mode for the Grafana transport.
	debug bool
}

func (dt *disabledTools) addFlags() {
	flag.StringVar(&dt.enabledTools, "enabled-tools", "search,datasource,incident,prometheus,loki,alerting,dashboard,oncall,asserts,sift,admin,file", "A comma separated list of tools enabled for this server. Can be overwritten entirely or by disabling specific components, e.g. --disable-search.")

	flag.BoolVar(&dt.search, "disable-search", false, "Disable search tools")
	flag.BoolVar(&dt.datasource, "disable-datasource", false, "Disable datasource tools")
	flag.BoolVar(&dt.incident, "disable-incident", false, "Disable incident tools")
	flag.BoolVar(&dt.prometheus, "disable-prometheus", false, "Disable prometheus tools")
	flag.BoolVar(&dt.loki, "disable-loki", false, "Disable loki tools")
	flag.BoolVar(&dt.alerting, "disable-alerting", false, "Disable alerting tools")
	flag.BoolVar(&dt.dashboard, "disable-dashboard", false, "Disable dashboard tools")
	flag.BoolVar(&dt.oncall, "disable-oncall", false, "Disable oncall tools")
	flag.BoolVar(&dt.asserts, "disable-asserts", false, "Disable asserts tools")
	flag.BoolVar(&dt.sift, "disable-sift", false, "Disable sift tools")
	flag.BoolVar(&dt.admin, "disable-admin", false, "Disable admin tools")
	flag.BoolVar(&dt.file, "disable-file", false, "Disable file tools")
}

func (gc *grafanaConfig) addFlags() {
	flag.BoolVar(&gc.debug, "debug", false, "Enable debug mode for the Grafana transport")
}

func (dt *disabledTools) addTools(s *server.MCPServer) {
	enabledTools := strings.Split(dt.enabledTools, ",")
	maybeAddTools(s, tools.AddSearchTools, enabledTools, dt.search, "search")
	maybeAddTools(s, tools.AddDatasourceTools, enabledTools, dt.datasource, "datasource")
	maybeAddTools(s, tools.AddIncidentTools, enabledTools, dt.incident, "incident")
	maybeAddTools(s, tools.AddPrometheusTools, enabledTools, dt.prometheus, "prometheus")
	maybeAddTools(s, tools.AddLokiTools, enabledTools, dt.loki, "loki")
	maybeAddTools(s, tools.AddAlertingTools, enabledTools, dt.alerting, "alerting")
	maybeAddTools(s, tools.AddDashboardTools, enabledTools, dt.dashboard, "dashboard")
	maybeAddTools(s, tools.AddOnCallTools, enabledTools, dt.oncall, "oncall")
	maybeAddTools(s, tools.AddAssertsTools, enabledTools, dt.asserts, "asserts")
	maybeAddTools(s, tools.AddSiftTools, enabledTools, dt.sift, "sift")
	maybeAddTools(s, tools.AddAdminTools, enabledTools, dt.admin, "admin")
	maybeAddTools(s, tools.AddFileTools, enabledTools, dt.file, "file")
}


func newServer(dt disabledTools) *server.MCPServer {
	s := server.NewMCPServer(
		"mcp-grafana",
		"0.1.0",
	)
	dt.addTools(s)
	resource := mcp.NewResource("cmd/mcp-grafana/details.txt",
		"Static Resource",
		mcp.WithMIMEType("text/plain"),
		mcp.WithResourceDescription("Contains details about harsh. Use this resource to learn and answer anything about harsh"))
	
	s.AddResource(resource, HandleReadResource)
	slog.Info("Resource registered", "uri", resource.URI)
	cwd, _ := os.Getwd()
	fmt.Println("Working directory:", cwd)

// 	prompt := mcp.NewPrompt("extract_promql_from_grafana",
//     mcp.WithPromptDescription(
//         "Extract and execute the Prometheus query for 'Rate of Calls Offer Accepted' from the Grafana dashboard 'CCAAS-Prod'",
//     ),
//     mcp.WithArgument("dashboard_uid",
//         mcp.ArgumentDescription("UID of the Grafana dashboard"),
//         mcp.RequiredArgument(),
//     ),
//     mcp.WithArgument("panel_name",
//         mcp.ArgumentDescription("Name of the panel to search in (e.g., 'Voice Agent State')"),
//         mcp.RequiredArgument(),
//     ),
//     mcp.WithArgument("query_variable",
//         mcp.ArgumentDescription("Name of the Prometheus query/metric (e.g., 'Rate of Calls Offer Accepted')"),
//         mcp.RequiredArgument(),
//     ),
//     mcp.WithArgument("datasource_uid",
//         mcp.ArgumentDescription("UID of the Prometheus datasource"),
//         mcp.RequiredArgument(),
//     ),
// )

// 	s.AddPrompt(prompt, handleExtractQuery)
	//prompts.RegisterPrompts(s)
	prompts.RegisterPrompts(s);
	return s
}


func HandleReadResource(
    ctx context.Context,
    request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
    data, err := ioutil.ReadFile("details.txt") 
    if err != nil {
        return nil, err
    }
	slog.Info("Read resource", "uri", data, "size", len(data))
    return []mcp.ResourceContents{
        mcp.TextResourceContents{
            URI:      "details.txt",
            MIMEType: "text/plain",
            Text:     string(data),
        },
    }, nil
}

func run(transport, addr, basePath string, logLevel slog.Level, dt disabledTools, gc grafanaConfig) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
	s := newServer(dt)

	switch transport {
	case "stdio":
		srv := server.NewStdioServer(s)
		srv.SetContextFunc(mcpgrafana.ComposedStdioContextFunc(gc.debug))
		slog.Info("Starting Grafana MCP server using stdio transport")
		return srv.Listen(context.Background(), os.Stdin, os.Stdout)
	case "sse":
		srv := server.NewSSEServer(s,
			server.WithSSEContextFunc(mcpgrafana.ComposedSSEContextFunc(gc)),
			server.WithStaticBasePath(basePath),
		)
		slog.Info("Starting Grafana MCP server using SSE transport", "address", addr, "basePath", basePath)
		if err := srv.Start(addr); err != nil {
			return fmt.Errorf("Server error: %v", err)
		}

	default:
		return fmt.Errorf(
			"Invalid transport type: %s. Must be 'stdio' or 'sse'",
			transport,
		)
	}
	return nil
}

func main() {
	var transport string
	flag.StringVar(&transport, "t", "sse", "Transport type (stdio or sse)")
	flag.StringVar(
		&transport,
		"transport",
		"sse",
		"Transport type (stdio or sse)",
	)

	addr := flag.String("sse-address", "localhost:8005", "The host and port to start the sse server on")
	basePath := flag.String("base-path", "", "Base path for the sse server")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	var dt disabledTools
	dt.addFlags()
	var gc grafanaConfig
	gc.addFlags()
	flag.Parse()

	if err := run(transport, *addr, *basePath, parseLevel(*logLevel), dt, gc); err != nil {
		panic(err)
	}
}

func parseLevel(level string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}
	return l
}


// type PromptTemplate struct {
//     Name        string
//     Description string
//     Template    string
//     Variables   []string
// }
 



// func handleExtractQuery(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
//     args := req.Params.Arguments
//     if args == nil {
//         return nil, fmt.Errorf("missing prompt arguments")
//     }

//     // Required parameters
//     dashboardUID, ok := args["dashboardUID"]
//     if !ok || dashboardUID == "" {
//         return nil, fmt.Errorf("argument 'dashboardUID' is required")
//     }
//     metric, ok := args["metric"]
//     if !ok || metric == "" {
//         return nil, fmt.Errorf("argument 'metric' is required")
//     }
//     prometheusUID, ok := args["prometheusUID"]
//     if !ok || prometheusUID == "" {
//         return nil, fmt.Errorf("argument 'prometheusUID' is required")
//     }
//     panelTitle, ok := args["panelTitle"]
//     if !ok || panelTitle == "" {
//         return nil, fmt.Errorf("argument 'panelTitle' is required")
//     }

//     // Substitute variables into the system prompt template
//     promptText := fmt.Sprintf(
//         `From the Grafana dashboard titled "CCAAS-Prod" (%s), extract the Prometheus query used to calculate %s

// Use the Prometheus datasource UID: %s
// Task Steps:

// ... (omitted steps) ...
// END`,
//         dashboardUID,
//         metric,
//         prometheusUID,
//     )

//     // // If variable resolution is needed (e.g. $foo in promptText), detect now:
//     // unresolvedVars := detectTemplateVars(promptText) // implement detection logic
//     // if len(unresolvedVars) > 0 {
//     //     // Prompt user to supply values
//     //     var buffer strings.Builder
//     //     buffer.WriteString(promptText + "\n\n")
//     //     buffer.WriteString("The following variables are unresolved:\n")
//     //     for _, v := range unresolvedVars {
//     //         buffer.WriteString(fmt.Sprintf("• %s\n", v))
//     //     }
//     //     buffer.WriteString("Please provide values for these variables before proceeding.")

//     //     return &mcp.GetPromptResult{
//     //         Description: "Resolve template variables",
//     //         Messages: []mcp.PromptMessage{
//     //             {
//     //                 Role:    "user",
//     //                 Content: mcp.NewTextContent(buffer.String()),
//     //             },
//     //         },
//     //     }, nil
//     // }

//     // All variables are resolved, now return final system prompt
//     return &mcp.GetPromptResult{
//         Description: "Ready to execute query extraction",
//         Messages: []mcp.PromptMessage{
//             {
//                 Role:    "user",
//                 Content: mcp.NewTextContent(promptText),
//             },
//         },
//     }, nil
// }
