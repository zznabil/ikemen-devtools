package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MaxFrameSize = 16 << 20

var ErrMalformedFrame = errors.New("lsp: malformed JSON-RPC frame")

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MarshalJSON enforces JSON-RPC response exclusivity: a response has either
// result or error, never both. A nil successful result is still encoded as
// "result":null (for example, shutdown).
func (r Response) MarshalJSON() ([]byte, error) {
	if r.Error != nil {
		return json.Marshal(struct {
			JSONRPC string      `json:"jsonrpc"`
			ID      interface{} `json:"id"`
			Error   *Error      `json:"error"`
		}{JSONRPC: r.JSONRPC, ID: r.ID, Error: r.Error})
	}
	return json.Marshal(struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Result  interface{} `json:"result"`
	}{JSONRPC: r.JSONRPC, ID: r.ID, Result: r.Result})
}

// ReadFrame reads one LSP Content-Length framed JSON message.
func ReadFrame(r io.Reader) ([]byte, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	headers := make(map[string]string)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, ErrMalformedFrame
		}
		headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	length, err := strconv.Atoi(headers["content-length"])
	if err != nil || length < 0 || length > MaxFrameSize {
		return nil, ErrMalformedFrame
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(br, body); err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, ErrMalformedFrame
	}
	return body, nil
}

// WriteFrame writes one LSP Content-Length framed JSON message.
func WriteFrame(w io.Writer, body []byte) error {
	if len(body) > MaxFrameSize || !json.Valid(body) {
		return ErrMalformedFrame
	}
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body))
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func encodeFrame(v interface{}) ([]byte, error) { return json.Marshal(v) }
func decodeRequest(body []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil || req.JSONRPC != "2.0" || req.Method == "" {
		return req, ErrMalformedFrame
	}
	return req, nil
}

func rawID(raw json.RawMessage) interface{} {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value interface{}
	if decoder.Decode(&value) != nil {
		return nil
	}
	return value
}

func isNotification(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) == 0
}
