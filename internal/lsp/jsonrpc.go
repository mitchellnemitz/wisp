// Package lsp implements a Language Server Protocol server for wisp. It speaks
// JSON-RPC 2.0 over a stream (stdio in production) and reuses the compiler's
// own lexer, parser, type checker, and formatter -- it never reimplements
// language logic, so the server cannot diverge from the compiler.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// JSON-RPC 2.0 error codes (subset used by this server).
const (
	codeParseError     = -32700
	codeInvalidParams  = -32602
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// errParseError signals that a frame was read whole but its body was not valid
// JSON. The body bytes were consumed, so the stream stays aligned and the next
// readMessage reads the following message -- the server replies with a
// parse-error response and keeps serving.
var errParseError = errors.New("lsp: request body is not valid JSON")

// errBadFrame signals framing that cannot be resynchronized (missing or
// non-integer Content-Length). The caller stops reading.
var errBadFrame = errors.New("lsp: malformed message frame")

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// message is an incoming or outgoing JSON-RPC message. Method present with an
// ID is a request; Method present without an ID is a notification.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (m *message) isRequest() bool { return m.Method != "" && len(m.ID) > 0 }

// conn is the framed JSON-RPC transport over an io.Reader/io.Writer.
type conn struct {
	r *bufio.Reader
	w io.Writer
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

// readMessage reads one Content-Length-framed message. Unrecognized header
// lines are tolerated (skipped). Returns io.EOF at end of stream, errParseError
// for a whole-but-invalid-JSON body (stream stays aligned), or errBadFrame for
// unrecoverable framing.
func (c *conn) readMessage() (*message, error) {
	contentLength := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			// EOF mid- or pre-header: end of stream.
			return nil, io.EOF
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break // blank line terminates the header block
		}
		colon := strings.IndexByte(trimmed, ':')
		if colon < 0 {
			continue // skip a malformed header line; keep reading
		}
		name := strings.TrimSpace(trimmed[:colon])
		value := strings.TrimSpace(trimmed[colon+1:])
		if strings.EqualFold(name, "Content-Length") {
			n, convErr := strconv.Atoi(value)
			if convErr != nil || n < 0 {
				return nil, errBadFrame
			}
			contentLength = n
		}
		// Other headers (e.g. Content-Type) are ignored.
	}
	if contentLength < 0 {
		return nil, errBadFrame
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, body); err != nil {
		// A clean end-of-stream before any body byte is end-of-input; a partial
		// body (Content-Length larger than the bytes that arrived) is a framing
		// desync, not a normal shutdown.
		if err == io.ErrUnexpectedEOF {
			return nil, errBadFrame
		}
		return nil, io.EOF
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, errParseError
	}
	return &m, nil
}

// write frames and writes a single JSON payload.
func (c *conn) write(payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// respond writes a success response for the given request id.
func (c *conn) respond(id json.RawMessage, result any) error {
	return c.write(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{"2.0", id, result})
}

// respondError writes an error response. A nil id is encoded as JSON null
// (used for a parse error where no id could be recovered).
func (c *conn) respondError(id json.RawMessage, code int, msg string) error {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return c.write(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   rpcError        `json:"error"`
	}{"2.0", id, rpcError{Code: code, Message: msg}})
}

// notify writes a server-to-client notification (no id).
func (c *conn) notify(method string, params any) error {
	return c.write(struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", method, params})
}
