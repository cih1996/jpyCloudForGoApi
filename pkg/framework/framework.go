package framework

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
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

