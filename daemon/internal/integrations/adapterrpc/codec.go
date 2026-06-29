package adapterrpc

import (
	"bufio"
	"encoding/json"
	"io"
)

// WriteMessage encodes v as a single newline-delimited JSON frame. Used by both the daemon
// (requests) and the adapter (responses).
func WriteMessage(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

// ReadRequest decodes one request frame. Used by the adapter side.
func ReadRequest(r *bufio.Reader) (Request, error) {
	line, err := r.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return Request{}, err
	}
	var req Request
	if uerr := json.Unmarshal(trimNewline(line), &req); uerr != nil {
		return Request{}, uerr
	}
	return req, nil
}

// ReadResponse decodes one response frame. Used by the daemon side.
func ReadResponse(r *bufio.Reader) (Response, error) {
	line, err := r.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return Response{}, err
	}
	var resp Response
	if uerr := json.Unmarshal(trimNewline(line), &resp); uerr != nil {
		return Response{}, uerr
	}
	return resp, nil
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
