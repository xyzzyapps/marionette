// @tag-block architecture
// @tag layer: "protocol"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["query", "storage"]
// @tag description: "MCP server — 6 AI agent tools (tagdoc_search, pii_scan, architecture, compliance, feature_flags, depends)"
//
// Package lsp — MCP server implementation for AI agent integration.
//
// The Model Context Protocol (MCP) server exposes @tag annotations as
// callable tools. AI agents can query tags directly without running CLI
// commands. Communicates via stdin/stdout using JSON-RPC.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/xyzzyapps/marionette/src/pkg/models"
	"github.com/xyzzyapps/marionette/src/pkg/query"
	"github.com/xyzzyapps/marionette/src/pkg/storage"
)

// MCPServer provides an MCP interface exposing @tag tools to AI agents.
type MCPServer struct {
	engine *query.Engine
	store  *storage.Store
	reader *bufio.Reader
	writer io.Writer
}

// NewMCPServer creates a new MCP server.
func NewMCPServer(engine *query.Engine, store *storage.Store) *MCPServer {
	return &MCPServer{
		engine: engine,
		store:  store,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}
}

// Run starts the MCP server loop, reading JSON-RPC messages from stdin.
func (s *MCPServer) Run() error {
	log.SetOutput(os.Stderr)

	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Printf("MCP read error: %v", err)
			continue
		}

		if err := s.handleMessage(msg); err != nil {
			log.Printf("MCP handle error: %v", err)
		}
	}
}

// JSON-RPC message structures
type mcpMessage struct {
	ID     *int             `json:"id"`
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	ID     int         `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// readMessage reads a single JSON-RPC message from stdin (Content-Length header + JSON body).
func (s *MCPServer) readMessage() (*mcpMessage, error) {
	contentLength := 0
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("invalid content length: %d", contentLength)
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(s.reader, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var msg mcpMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}
	return &msg, nil
}

// handleMessage routes an MCP message to the appropriate handler.
func (s *MCPServer) handleMessage(msg *mcpMessage) error {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(msg)
	case "shutdown":
		return s.handleShutdown(msg)
	case "exit":
		os.Exit(0)
		return nil
	default:
		if msg.ID != nil {
			return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", msg.Method)})
		}
		return nil
	}
}

// handleInitialize responds to the MCP initialize request.
func (s *MCPServer) handleInitialize(msg *mcpMessage) error {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "tagdoc-mcp",
			"version": "0.2.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]bool{
				"list": true,
				"call": true,
			},
		},
	}
	return s.sendResponse(*msg.ID, result, nil)
}

// MCPToolDefinition describes an MCP tool for the tools/list response.
type MCPToolDefinition struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	InputSchema map[string]interface{}   `json:"inputSchema"`
}

// handleToolsList returns the list of available MCP tools.
func (s *MCPServer) handleToolsList(msg *mcpMessage) error {
	tools := []MCPToolDefinition{
		{
			Name:        "tagdoc_search",
			Description: "Search tags by kind, key, or value. Returns matching @tag annotations with file locations and metadata.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"kind":  map[string]interface{}{"type": "string", "description": "@tag-block kind (e.g., api-endpoint)"},
					"key":   map[string]interface{}{"type": "string", "description": "Metadata key to search"},
					"value": map[string]interface{}{"type": "string", "description": "Metadata value to search"},
				},
			},
		},
		{
			Name:        "tagdoc_pii_scan",
			Description: "List all PII-tagged code locations. Finds tags with security:pii-access or pii keys for compliance auditing.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"format": map[string]interface{}{"type": "string", "description": "Output format: text (default) or json"},
				},
			},
		},
		{
			Name:        "tagdoc_architecture",
			Description: "List architecture entry points. Shows all @tag-block architecture annotations with layer, domain, and dependency metadata.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"domain": map[string]interface{}{"type": "string", "description": "Filter by domain name"},
				},
			},
		},
		{
			Name:        "tagdoc_compliance",
			Description: "Generate a security/GDPR compliance report. Shows all security and gdpr-impact tagged code with risk levels.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"min_severity": map[string]interface{}{"type": "string", "description": "Minimum severity: low, medium, high, critical"},
				},
			},
		},
		{
			Name:        "tagdoc_feature_flags",
			Description: "List feature flag declarations and status. Shows rollout percentages, owners, and feature flag metadata.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{"type": "string", "description": "Filter by status (e.g., active, permanent, deprecated)"},
				},
			},
		},
		{
			Name:        "tagdoc_depends",
			Description: "Find tag dependency relationships. Shows which tags reference a given key, value, or service name.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]interface{}{"type": "string", "description": "Key, value, or service name to find dependencies for"},
				},
			},
		},
	}
	return s.sendResponse(*msg.ID, map[string]interface{}{"tools": tools}, nil)
}

// MCPToolCallParams represents the parameters for a tools/call request.
type MCPToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// handleToolsCall routes a tool call to the appropriate handler.
func (s *MCPServer) handleToolsCall(msg *mcpMessage) error {
	var params MCPToolCallParams
	if msg.Params != nil {
		if err := json.Unmarshal(*msg.Params, &params); err != nil {
			return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32602, Message: "Invalid params"})
		}
	}

	if params.Arguments == nil {
		params.Arguments = make(map[string]interface{})
	}

	switch params.Name {
	case "tagdoc_search":
		return s.handleToolSearch(msg, params.Arguments)
	case "tagdoc_pii_scan":
		return s.handleToolPIIScan(msg, params.Arguments)
	case "tagdoc_architecture":
		return s.handleToolArchitecture(msg, params.Arguments)
	case "tagdoc_compliance":
		return s.handleToolCompliance(msg, params.Arguments)
	case "tagdoc_feature_flags":
		return s.handleToolFeatureFlags(msg, params.Arguments)
	case "tagdoc_depends":
		return s.handleToolDepends(msg, params.Arguments)
	default:
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32601, Message: fmt.Sprintf("Tool not found: %s", params.Name)})
	}
}

// ── MCP Tool Implementations ───────────────────────────────────────────

// handleToolSearch implements the tagdoc_search MCP tool.
func (s *MCPServer) handleToolSearch(msg *mcpMessage, args map[string]interface{}) error {
	kind, _ := args["kind"].(string)
	key, _ := args["key"].(string)
	value, _ := args["value"].(string)

	tags, err := s.store.QueryTags(models.Query{})
	if err != nil {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32603, Message: err.Error()})
	}

	var results []map[string]interface{}
	for _, t := range tags {
		// Apply filters
		if kind != "" {
			if k, ok := t.Metadata["kind"]; !ok || fmt.Sprintf("%v", k) != kind {
				continue
			}
		}
		if key != "" {
			if _, ok := t.Metadata[key]; !ok {
				continue
			}
		}
		if value != "" {
			found := false
			for _, v := range t.Metadata {
				if strings.Contains(fmt.Sprintf("%v", v), value) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		ename := ""
		if t.EntityName != nil {
			ename = *t.EntityName
		}
		results = append(results, map[string]interface{}{
			"id":          t.ID,
			"file":        t.File,
			"language":    t.Language,
			"tag_type":    string(t.TagType),
			"entity_name": ename,
			"line":        t.Range.Start.Line,
			"metadata":    t.Metadata,
		})
	}

	return s.sendResponse(*msg.ID, map[string]interface{}{
		"count":  len(results),
		"results": results,
	}, nil)
}

// handleToolPIIScan implements the tagdoc_pii_scan MCP tool.
func (s *MCPServer) handleToolPIIScan(msg *mcpMessage, args map[string]interface{}) error {
	tags, err := s.store.QueryTags(models.Query{})
	if err != nil {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32603, Message: err.Error()})
	}

	var piiLocations []map[string]interface{}
	for _, t := range tags {
		isPII := false
		piiKeys := []string{}
		for k, v := range t.Metadata {
			keyLower := strings.ToLower(k)
			valLower := strings.ToLower(fmt.Sprintf("%v", v))
			if strings.Contains(keyLower, "pii") || strings.Contains(valLower, "pii") {
				isPII = true
				piiKeys = append(piiKeys, fmt.Sprintf("%s=%v", k, v))
			}
		}
		if !isPII {
			continue
		}

		ename := ""
		if t.EntityName != nil {
			ename = *t.EntityName
		}
		piiLocations = append(piiLocations, map[string]interface{}{
			"file":        t.File,
			"language":    t.Language,
			"entity_name": ename,
			"line":        t.Range.Start.Line,
			"pii_tags":    piiKeys,
			"compliance":  extractComplianceInfo(t.Metadata),
		})
	}

	return s.sendResponse(*msg.ID, map[string]interface{}{
		"count":      len(piiLocations),
		"pii_locations": piiLocations,
	}, nil)
}

// handleToolArchitecture implements the tagdoc_architecture MCP tool.
func (s *MCPServer) handleToolArchitecture(msg *mcpMessage, args map[string]interface{}) error {
	domainFilter, _ := args["domain"].(string)

	tags, err := s.store.QueryTags(models.Query{})
	if err != nil {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32603, Message: err.Error()})
	}

	var entryPoints []map[string]interface{}
	for _, t := range tags {
		if kind, ok := t.Metadata["kind"]; !ok || fmt.Sprintf("%v", kind) != "architecture" {
			continue
		}
		if domainFilter != "" {
			if d, ok := t.Metadata["domain"]; !ok || fmt.Sprintf("%v", d) != domainFilter {
				continue
			}
		}

		ename := ""
		if t.EntityName != nil {
			ename = *t.EntityName
		}
		entryPoints = append(entryPoints, map[string]interface{}{
			"file":        t.File,
			"language":    t.Language,
			"entity_name": ename,
			"line":        t.Range.Start.Line,
			"metadata":    t.Metadata,
		})
	}

	return s.sendResponse(*msg.ID, map[string]interface{}{
		"count":        len(entryPoints),
		"entry_points": entryPoints,
	}, nil)
}

// handleToolCompliance implements the tagdoc_compliance MCP tool.
func (s *MCPServer) handleToolCompliance(msg *mcpMessage, args map[string]interface{}) error {
	tags, err := s.store.QueryTags(models.Query{})
	if err != nil {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32603, Message: err.Error()})
	}

	var complianceItems []map[string]interface{}
	for _, t := range tags {
		secVal, hasSec := t.Metadata["security"]
		gdprVal, hasGDPR := t.Metadata["gdpr-impact"]
		hasPII := false
		for k, v := range t.Metadata {
			if strings.Contains(strings.ToLower(k), "pii") || strings.Contains(strings.ToLower(fmt.Sprintf("%v", v)), "pii") {
				hasPII = true
				break
			}
		}
		if !hasSec && !hasGDPR && !hasPII {
			continue
		}

		ename := ""
		if t.EntityName != nil {
			ename = *t.EntityName
		}

		severity := "low"
		if hasGDPR {
			sevStr := fmt.Sprintf("%v", gdprVal)
			switch strings.ToLower(sevStr) {
			case "critical", "high":
				severity = "high"
			case "medium":
				severity = "medium"
			}
		}
		if hasPII && severity == "low" {
			severity = "medium"
		}

		complianceItems = append(complianceItems, map[string]interface{}{
			"file":        t.File,
			"language":    t.Language,
			"entity_name": ename,
			"line":        t.Range.Start.Line,
			"security":    secVal,
			"gdpr_impact": gdprVal,
			"has_pii":     hasPII,
			"severity":    severity,
		})
	}

	return s.sendResponse(*msg.ID, map[string]interface{}{
		"count":    len(complianceItems),
		"findings": complianceItems,
		"summary":  summarizeCompliance(complianceItems),
	}, nil)
}

// handleToolFeatureFlags implements the tagdoc_feature_flags MCP tool.
func (s *MCPServer) handleToolFeatureFlags(msg *mcpMessage, args map[string]interface{}) error {
	statusFilter, _ := args["status"].(string)

	tags, err := s.store.QueryTags(models.Query{})
	if err != nil {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32603, Message: err.Error()})
	}

	var flags []map[string]interface{}
	for _, t := range tags {
		if kind, ok := t.Metadata["kind"]; !ok || fmt.Sprintf("%v", kind) != "feature-flag" {
			continue
		}
		if statusFilter != "" {
			if s, ok := t.Metadata["status"]; !ok || fmt.Sprintf("%v", s) != statusFilter {
				continue
			}
		}

		ename := ""
		if t.EntityName != nil {
			ename = *t.EntityName
		}
		flags = append(flags, map[string]interface{}{
			"file":        t.File,
			"entity_name": ename,
			"line":        t.Range.Start.Line,
			"metadata":    t.Metadata,
		})
	}

	return s.sendResponse(*msg.ID, map[string]interface{}{
		"count":  len(flags),
		"flags":  flags,
	}, nil)
}

// handleToolDepends implements the tagdoc_depends MCP tool.
func (s *MCPServer) handleToolDepends(msg *mcpMessage, args map[string]interface{}) error {
	target, _ := args["target"].(string)
	if target == "" {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32602, Message: "target parameter is required"})
	}

	tags, err := s.store.QueryTags(models.Query{})
	if err != nil {
		return s.sendResponse(*msg.ID, nil, &mcpError{Code: -32603, Message: err.Error()})
	}

	var related []map[string]interface{}
	for _, t := range tags {
		for k, v := range t.Metadata {
			valStr := fmt.Sprintf("%v", v)
			if strings.Contains(k, target) || strings.Contains(valStr, target) {
				ename := ""
				if t.EntityName != nil {
					ename = *t.EntityName
				}
				related = append(related, map[string]interface{}{
					"file":        t.File,
					"language":    t.Language,
					"entity_name": ename,
					"line":        t.Range.Start.Line,
					"matched_key":   k,
					"matched_value": valStr,
					"metadata":      t.Metadata,
				})
				break
			}
		}
	}

	return s.sendResponse(*msg.ID, map[string]interface{}{
		"count":    len(related),
		"target":   target,
		"related":  related,
	}, nil)
}

// ── Helpers ────────────────────────────────────────────────────────────

// extractComplianceInfo extracts compliance-related metadata from a tag.
func extractComplianceInfo(m map[string]interface{}) map[string]interface{} {
	info := make(map[string]interface{})
	for k, v := range m {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "gdpr") || strings.Contains(kl, "pii") ||
			strings.Contains(kl, "audit") || strings.Contains(kl, "compliance") ||
			strings.Contains(kl, "security") || strings.Contains(kl, "requires-review") {
			info[k] = v
		}
	}
	return info
}

// summarizeCompliance generates a summary of compliance findings.
func summarizeCompliance(items []map[string]interface{}) map[string]interface{} {
	high := 0
	medium := 0
	low := 0
	for _, item := range items {
		switch item["severity"] {
		case "high":
			high++
		case "medium":
			medium++
		default:
			low++
		}
	}
	return map[string]interface{}{
		"total_findings": len(items),
		"high_severity":  high,
		"medium_severity": medium,
		"low_severity":   low,
	}
}

// handleShutdown processes the shutdown request.
func (s *MCPServer) handleShutdown(msg *mcpMessage) error {
	return s.sendResponse(*msg.ID, nil, nil)
}

// sendResponse writes a JSON-RPC response to stdout.
func (s *MCPServer) sendResponse(id int, result interface{}, mcpErr *mcpError) error {
	resp := mcpResponse{ID: id, Result: result, Error: mcpErr}
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	_, err = fmt.Fprintf(s.writer, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}
