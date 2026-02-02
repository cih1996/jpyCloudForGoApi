package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// HandlerFunc defines the generic handler signature
type HandlerFunc[Req any, Resp any] func(context.Context, *Req) (*Resp, error)

// RouteDefinition stores metadata about a registered route
type RouteDefinition struct {
	Path        string
	Method      string
	Description string
	ReqType     reflect.Type
	RespType    reflect.Type
	Handler     interface{} // Stores the generic handler
}

// Registry stores all registered routes
var Registry = []RouteDefinition{}

// Register registers a new route with the framework
func Register[Req any, Resp any](method, path, description string, handler HandlerFunc[Req, Resp]) {
	var req Req
	var resp Resp
	Registry = append(Registry, RouteDefinition{
		Path:        path,
		Method:      method,
		Description: description,
		ReqType:     reflect.TypeOf(req),
		RespType:    reflect.TypeOf(resp),
		Handler:     handler,
	})
}

// BindHTTP binds all registered routes to a Gin engine
func BindHTTP(r *gin.Engine) {
	for _, route := range Registry {
		handlerVal := reflect.ValueOf(route.Handler)

		r.Handle(route.Method, route.Path, func(c *gin.Context) {
			// Create new instance of Request type
			reqPtr := reflect.New(route.ReqType)
			reqInterface := reqPtr.Interface()

			// Bind request
			var err error
			if c.Request.Method == "GET" {
				err = c.ShouldBindQuery(reqInterface)
			} else {
				err = c.ShouldBindJSON(reqInterface)
			}

			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
				return
			}

			// Call handler
			args := []reflect.Value{
				reflect.ValueOf(c.Request.Context()),
				reqPtr,
			}
			results := handlerVal.Call(args)

			// Parse results: (*Resp, error)
			respVal := results[0]
			errVal := results[1]

			if !errVal.IsNil() {
				// Assuming error interface
				errObj := errVal.Interface().(error)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errObj.Error()})
				return
			}

			if respVal.IsNil() {
				c.JSON(http.StatusOK, gin.H{"success": true})
			} else {
				c.JSON(http.StatusOK, respVal.Interface())
			}
		})
	}
}

// GetDocs returns a list of registered routes for documentation
func GetDocs() []map[string]interface{} {
	docs := []map[string]interface{}{}
	for _, route := range Registry {
		docs = append(docs, map[string]interface{}{
			"path":        route.Path,
			"method":      route.Method,
			"description": route.Description,
			"request":     getTypeStructure(route.ReqType),
			"response":    getTypeStructure(route.RespType),
		})
	}
	return docs
}

// Helper to describe type structure (simplified)
func getTypeStructure(t reflect.Type) interface{} {
	if t == nil {
		return "nil"
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return t.String()
	}

	fields := map[string]string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = field.Name
		}
		jsonTag = strings.Split(jsonTag, ",")[0]
		fields[jsonTag] = field.Type.String()
	}
	return fields
}

// --- WebSocket Support ---

// WsRequest is the envelope for WebSocket messages
type WsRequest struct {
	Path string          `json:"path"`
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

// WsResponse is the envelope for WebSocket responses
type WsResponse struct {
	ID      string      `json:"id"`
	Path    string      `json:"path"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// StartWS starts a WebSocket server on the specified port
func StartWS(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logs.Error("WebSocket upgrade failed:", err)
			return
		}
		defer conn.Close()
		handleWSConnection(conn)
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logs.Info("WebSocket service started on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logs.Error("WebSocket server failed:", err)
	}
}

func handleWSConnection(conn *websocket.Conn) {
	for {
		var reqEnvelope WsRequest
		if err := conn.ReadJSON(&reqEnvelope); err != nil {
			logs.Info("WebSocket read error (client likely disconnected):", err)
			break
		}

		go processWSMessage(conn, reqEnvelope)
	}
}

func processWSMessage(conn *websocket.Conn, req WsRequest) {
	resp := WsResponse{
		ID:      req.ID,
		Path:    req.Path,
		Success: false,
	}

	// Find Route
	var matchedRoute *RouteDefinition
	for _, route := range Registry {
		if route.Path == req.Path {
			matchedRoute = &route
			break
		}
	}

	if matchedRoute == nil {
		resp.Message = "Route not found"
		conn.WriteJSON(resp)
		return
	}

	// Create request instance
	reqPtr := reflect.New(matchedRoute.ReqType)
	reqInterface := reqPtr.Interface()

	// Unmarshal Data into Request Struct
	if len(req.Data) > 0 {
		if err := json.Unmarshal(req.Data, reqInterface); err != nil {
			resp.Message = "Invalid data format: " + err.Error()
			conn.WriteJSON(resp)
			return
		}
	}

	// Call Handler
	handlerVal := reflect.ValueOf(matchedRoute.Handler)
	args := []reflect.Value{
		reflect.ValueOf(context.Background()),
		reqPtr,
	}
	results := handlerVal.Call(args)

	// Parse results
	respVal := results[0]
	errVal := results[1]

	if !errVal.IsNil() {
		errObj := errVal.Interface().(error)
		resp.Message = errObj.Error()
		conn.WriteJSON(resp)
		return
	}

	resp.Success = true
	if !respVal.IsNil() {
		resp.Data = respVal.Interface()
	}
	conn.WriteJSON(resp)
}
