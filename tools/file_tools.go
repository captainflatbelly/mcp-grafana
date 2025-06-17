package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"


	"github.com/mark3labs/mcp-go/server"


	mcpgrafana "mcp-grafana-local"
)

// FileMeta holds metadata about a file.
type FileMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// Configurable JSON file path for file metadata
const fileMetaJSONPath = "tools/file_meta.json"

type ListFilesParams struct{}  // define an empty but named struct

func listFilesFromJSONTool(ctx context.Context, args ListFilesParams) ([]FileMeta, error) {
    bytes, err := ioutil.ReadFile(fileMetaJSONPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read file metadata JSON: %w", err)
    }
    var files []FileMeta
    if err := json.Unmarshal(bytes, &files); err != nil {
        return nil, fmt.Errorf("failed to parse file metadata JSON: %w", err)
    }
    return files, nil
}


var ListFiles = mcpgrafana.MustTool(
	"list_files",
	"Gain additional details/context about the prompt from the files available in the system.",
	listFilesFromJSONTool,
)

// ReadFileParams specifies the file path to read.
type ReadFileParams struct {
	Path string `json:"path" jsonschema:"description=Path to the file to be read."`
}

// readFileTool reads the file at the given path and returns its contents.
func readFileTool(ctx context.Context, args ReadFileParams) (string, error) {
	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", args.Path, err)
	}
	return string(data), nil
}

var ReadFile = mcpgrafana.MustTool(
	"read_file",
	"Gain additional details/context about the prompt from a specific file.",
	readFileTool,
)

// AskFilePathParams for requesting file path from user.
type AskFilePathParams struct {
	Path string `json:"path" jsonschema:"description=Please provide the full path to the file you want to read."`
}

// askForFilePathTool prompts the user for a file path, then returns its contents.
func askForFilePathTool(ctx context.Context, args AskFilePathParams) (string, error) {
	if args.Path == "" {
		return "", fmt.Errorf("no file path provided")
	}
	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", args.Path, err)
	}
	return string(data), nil
}

var AskForFilePath = mcpgrafana.MustTool(
	"ask_for_file_path",
	"Prompt the user to provide additional context/details about the promptcle by asking for a file path.",
	askForFilePathTool,
)

// Register all file tools to the MCP server.
func AddFileTools(mcpServer *server.MCPServer) {
	ListFiles.Register(mcpServer)
	ReadFile.Register(mcpServer)
	AskForFilePath.Register(mcpServer)
}