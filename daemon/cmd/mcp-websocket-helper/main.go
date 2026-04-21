package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:19234", "listen address")
	path := flag.String("path", "/mcp", "websocket path")
	token := flag.String("token", "", "optional bearer token")
	flag.Parse()

	upgrader := websocket.Upgrader{}
	http.HandleFunc(*path, func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(*token) != "" && strings.TrimSpace(r.Header.Get("Authorization")) != "Bearer "+strings.TrimSpace(*token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		for {
			var req map[string]any
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			method, _ := req["method"].(string)
			id := req["id"]
			switch method {
			case "initialize":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "mcp-websocket-helper", "version": "1.0.0"}}})
			case "notifications/initialized":
			case "tools/list":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"tools": []map[string]any{{"name": "lookup", "title": "Lookup", "description": "Lookup tool", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}}}}}})
			case "tools/call":
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "websocket helper ok"}}}})
			default:
				_ = conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "method not found"}})
			}
		}
	})

	log.Printf("mcp websocket helper listening on %s%s", *listen, *path)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
