package server

import (
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// describe is the API as an OpenAPI 3 document, made from the routes
// themselves so it can't drift from them.
func describe(routes []route) map[string]any {
	schemas := map[string]any{
		"Error": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"error": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code":    map[string]any{"type": "string"},
						"message": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	paths := map[string]any{}
	for _, rt := range routes {
		item, _ := paths[apiPrefix+rt.path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[apiPrefix+rt.path] = item
		}

		item[strings.ToLower(rt.method)] = operation(rt, schemas)
	}

	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "cents",
			"version":     Version,
			"license":     map[string]any{"name": "MIT", "url": "https://opensource.org/licenses/MIT"},
			"description": "Personal finance tracker. Amounts are typed as text (\"12.50\") and returned in cents. Errors answer with an Error: 400 for a request that can't be read, 401 without a valid token, 403 for a read-only token's change, 404 for a missing record, 409 for a conflict (in_use: archive or merge instead), 422 for values to fix (the message says what), 502 when the exchange rate source failed, 500 when the database did.",
		},
		"servers": []any{map[string]any{"url": "/", "description": "This server."}},
		"paths":   paths,
		"components": map[string]any{
			"schemas":         schemas,
			"securitySchemes": map[string]any{"token": map[string]any{"type": "http", "scheme": "bearer"}},
		},
		"security": []any{map[string]any{"token": []any{}}},
	}
}

func operation(rt route, schemas map[string]any) map[string]any {
	op := map[string]any{"operationId": rt.name, "summary": rt.summary}

	var parameters []any
	for _, name := range pathParams(rt.path) {
		kind := "string"
		if idParams[name] {
			kind = "integer"
		}

		parameter := map[string]any{"name": name, "in": "path", "required": true, "schema": map[string]any{"type": kind}}
		if name == "month" {
			parameter["description"] = "YYYY-MM, or " + currentMonth + " for the running month"
		}
		if name == "list" {
			lists := make([]string, 0, len(settingLists))
			for list := range settingLists {
				lists = append(lists, list)
			}

			slices.Sort(lists)
			parameter["schema"] = map[string]any{"type": "string", "enum": lists}
		}

		parameters = append(parameters, parameter)
	}

	for _, spec := range rt.query {
		name, kind, _ := strings.Cut(spec, ":")
		types := map[string]string{"": "string", "int": "integer", "bool": "boolean"}
		parameters = append(parameters, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": types[kind]}})
	}

	if parameters != nil {
		op["parameters"] = parameters
	}

	if rt.in != nil {
		op["requestBody"] = map[string]any{
			"required": !rt.optionalBody,
			"content":  map[string]any{"application/json": map[string]any{"schema": schema(rt.in, schemas)}},
		}
	}

	success := map[string]any{"description": "Done."}
	status := rt.status
	switch {
	case rt.contentType != "":
		success["content"] = map[string]any{strings.Split(rt.contentType, ";")[0]: map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}
	case rt.out != nil:
		success["content"] = map[string]any{"application/json": map[string]any{"schema": schema(rt.out, schemas)}}
	case status == http.StatusOK:
		status = http.StatusNoContent
	}

	refused := map[string]any{"description": "Refused: see the API's description of errors.", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}}}
	failed := map[string]any{"description": "Failed on the server.", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}}}
	op["responses"] = map[string]any{strconv.Itoa(status): success, "4XX": refused, "5XX": failed}

	return op
}

var timeType = reflect.TypeFor[time.Time]()

// schema is t's JSON schema; named structs go to schemas, by name.
func schema(t reflect.Type, schemas map[string]any) map[string]any {
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t.Kind() == reflect.Pointer:
		s := schema(t.Elem(), schemas)
		if ref, ok := s["$ref"]; ok {
			return map[string]any{"allOf": []any{map[string]any{"$ref": ref}}, "nullable": true}
		}

		s["nullable"] = true
		return s
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schema(t.Elem(), schemas)}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schema(t.Elem(), schemas)}
	case reflect.Struct:
		name := t.Name()
		if name == "" {
			return object(t, schemas)
		}

		if _, done := schemas[name]; !done {
			schemas[name] = map[string]any{} // a placeholder while it's made, for types that refer to themselves
			schemas[name] = object(t, schemas)
		}

		return map[string]any{"$ref": "#/components/schemas/" + name}
	default:
		return map[string]any{}
	}
}

func object(t reflect.Type, schemas map[string]any) map[string]any {
	properties := map[string]any{}
	var add func(t reflect.Type)
	add = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous && f.Tag.Get("json") == "" && f.Type.Kind() == reflect.Struct {
				add(f.Type)
				continue
			}

			if name := jsonName(f); name != "" {
				properties[name] = schema(f.Type, schemas)
			}
		}
	}
	add(t)

	return map[string]any{"type": "object", "properties": properties}
}
