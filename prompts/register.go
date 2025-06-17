package prompts

import "github.com/mark3labs/mcp-go/server"

func RegisterPrompts(s *server.MCPServer) {
	s.AddPrompt(extractQueryPrompt(), handleExtractQuery)
	// Add more prompts as needed
}
