// @tag-block architecture
// @tag layer: "protocol"
// @tag domain: "tagdoc-core"
// @tag depends-on: ["query", "storage"]
// @tag description: "LSP server — Go to Definition, Find References for @tag annotations via stdin/stdout JSON-RPC"
//
// Package lsp provides a Language Server Protocol implementation for Marionette.
//
// It wraps the core tag engine in an LSP interface, enabling IDE features like
// "Go to Definition" and "Find References" for @tag annotations.
//
// The server communicates via stdin/stdout using JSON-RPC over LSP.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/xyzzyapps/marionette/src/pkg/query"
)

// Server is the LSP server for Marionette.
type Server struct {
	engine      *query.Engine
	reader      *bufio.Reader
	writer      io.Writer
	initialized bool
	capabilities
}

// capabilities represents the server's LSP capabilities.
type capabilities struct {
	TextDocumentSyncKind int `json:"textDocumentSync"`
	DefinitionProvider   bool `json:"definitionProvider"`
	ReferencesProvider   bool `json:"referencesProvider"`
}

// NewServer creates a new LSP server.
func NewServer(engine *query.Engine) *Server {
	return &Server{
		engine: engine,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		capabilities: capabilities{
			TextDocumentSyncKind: 1, // Full document sync
			DefinitionProvider:   true,
			ReferencesProvider:   true,
		},
	}
}

// Run starts the LSP server loop, reading JSON-RPC messages from stdin.
func (s *Server) Run() error {
	log.SetOutput(os.Stderr)

	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		if err := s.handleMessage(msg); err != nil {
			log.Printf("Error handling message: %v", err)
		}
	}
}

// LSP message structures
type jsonrpcMessage struct {
	ID     *int             `json:"id"`
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params,omitempty"`
}

type responseMessage struct {
	ID     int         `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *lspError   `json:"error,omitempty"`
}

type lspError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type notificationMessage struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params,omitempty"`
}

// readMessage reads a single LSP message from stdin.
func (s *Server) readMessage() (*jsonrpcMessage, error) {
	// Read Content-Length header
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

	// Read JSON body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(s.reader, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var msg jsonrpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &msg, nil
}

// handleMessage routes an LSP message to the appropriate handler.
func (s *Server) handleMessage(msg *jsonrpcMessage) error {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		s.initialized = true
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(msg)
	case "textDocument/didChange":
		return s.handleDidChange(msg)
	case "textDocument/didSave":
		return s.handleDidSave(msg)
	case "textDocument/definition":
		return s.handleDefinition(msg)
	case "textDocument/references":
		return s.handleReferences(msg)
	case "shutdown":
		return s.handleShutdown(msg)
	case "exit":
		os.Exit(0)
		return nil
	default:
		// Respond with empty result for unknown methods
		if msg.ID != nil {
			return s.sendResponse(*msg.ID, struct{}{}, nil)
		}
		return nil
	}
}

// handleInitialize responds to the LSP initialize request.
func (s *Server) handleInitialize(msg *jsonrpcMessage) error {
	result := map[string]interface{}{
		"capabilities": s.capabilities,
		"serverInfo": map[string]string{
			"name":    "tagdoc-lsp",
			"version": "0.1.0",
		},
	}
	return s.sendResponse(*msg.ID, result, nil)
}

// handleDidOpen processes textDocument/didOpen notification.
func (s *Server) handleDidOpen(msg *jsonrpcMessage) error {
	return nil // Tags are already indexed
}

// handleDidChange processes textDocument/didChange notification.
func (s *Server) handleDidChange(msg *jsonrpcMessage) error {
	return nil // For now, re-indexing on save is sufficient
}

// handleDidSave processes textDocument/didSave notification.
func (s *Server) handleDidSave(msg *jsonrpcMessage) error {
	return nil // Could trigger re-index of the saved file
}

// LSP position and location types
type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

// textDocumentDefinitionParams mirrors the LSP type.
type textDocumentDefinitionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position lspPosition `json:"position"`
}

// handleDefinition responds to textDocument/definition (Go to Definition).
func (s *Server) handleDefinition(msg *jsonrpcMessage) error {
	var params textDocumentDefinitionParams
	if err := json.Unmarshal(*msg.Params, &params); err != nil {
		return s.sendResponse(*msg.ID, nil, &lspError{Code: -32602, Message: "Invalid params"})
	}

	filePath := uriToPath(params.TextDocument.URI)
	tags, err := s.engine.GetFileTags(filePath)
	if err != nil || len(tags) == 0 {
		return s.sendResponse(*msg.ID, nil, nil)
	}

	// Find the tag at the cursor position
	for _, tag := range tags {
		if int(tag.Range.Start.Line) <= params.Position.Line &&
			int(tag.Range.End.Line) >= params.Position.Line {
			loc := lspLocation{
				URI: pathToURI(tag.File),
				Range: lspRange{
					Start: lspPosition{Line: int(tag.Range.Start.Line), Character: int(tag.Range.Start.Column)},
					End:   lspPosition{Line: int(tag.Range.End.Line), Character: int(tag.Range.End.Column)},
				},
			}
			return s.sendResponse(*msg.ID, loc, nil)
		}
	}

	return s.sendResponse(*msg.ID, nil, nil)
}

// textDocumentReferencesParams mirrors the LSP type.
type textDocumentReferencesParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position lspPosition `json:"position"`
	Context  struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

// handleReferences responds to textDocument/references (Find References).
func (s *Server) handleReferences(msg *jsonrpcMessage) error {
	var params textDocumentReferencesParams
	if err := json.Unmarshal(*msg.Params, &params); err != nil {
		return s.sendResponse(*msg.ID, nil, &lspError{Code: -32602, Message: "Invalid params"})
	}

	filePath := uriToPath(params.TextDocument.URI)
	tags, err := s.engine.GetFileTags(filePath)
	if err != nil || len(tags) == 0 {
		return s.sendResponse(*msg.ID, []lspLocation{}, nil)
	}

	// Find the tag at cursor to extract key/value for reference search
	var targetTag *struct{ key, value string }
	for _, tag := range tags {
		if int(tag.Range.Start.Line) <= params.Position.Line &&
			int(tag.Range.End.Line) >= params.Position.Line {
			// Use the first metadata key-value as reference target
			for k, v := range tag.Metadata {
				val := fmt.Sprintf("%v", v)
				targetTag = &struct{ key, value string }{k, val}
				break
			}
			break
		}
	}

	if targetTag == nil {
		return s.sendResponse(*msg.ID, []lspLocation{}, nil)
	}

	// Find all tags with the same key-value pair
	refs, err := s.engine.FindReferences(targetTag.key, targetTag.value)
	if err != nil {
		return s.sendResponse(*msg.ID, []lspLocation{}, nil)
	}

	var locations []lspLocation
	for _, ref := range refs {
		locations = append(locations, lspLocation{
			URI: pathToURI(ref.File),
			Range: lspRange{
				Start: lspPosition{Line: int(ref.Range.Start.Line), Character: int(ref.Range.Start.Column)},
				End:   lspPosition{Line: int(ref.Range.End.Line), Character: int(ref.Range.End.Column)},
			},
		})
	}

	return s.sendResponse(*msg.ID, locations, nil)
}

// handleShutdown processes the shutdown request.
func (s *Server) handleShutdown(msg *jsonrpcMessage) error {
	return s.sendResponse(*msg.ID, nil, nil)
}

// sendResponse writes a JSON-RPC response to stdout.
func (s *Server) sendResponse(id int, result interface{}, lspErr *lspError) error {
	resp := responseMessage{
		ID:     id,
		Result: result,
		Error:  lspErr,
	}

	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal response: %w", marshalErr)
	}

	_, writeErr := fmt.Fprintf(s.writer, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return writeErr
}

// uriToPath converts a file:// URI to a local file path.
func uriToPath(uri string) string {
	// Remove file:// prefix
	path := strings.TrimPrefix(uri, "file://")
	if strings.HasPrefix(path, "/") && len(path) > 2 && path[2] == ':' {
		// Windows path: /C:/path -> C:/path
		path = path[1:]
	}
	return path
}

// pathToURI converts a local file path to a file:// URI.
func pathToURI(path string) string {
	return "file://" + path
}
