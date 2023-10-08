package conf

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/zddava/goext/enum"
	"github.com/zddava/gowrap/consul"
	"github.com/zddava/gowrap/json"
	"golang.org/x/exp/slices"
)

const (
	KEY_HTTP_PORT           = "port"
	KEY_DYNAMIC_ROUTE       = "dynamic_route"
	KEY_HTTP_ROOT           = "db_root"
	KEY_CONSUL_API_BASE     = "consul_api_base"
	KEY_CONSUL_SERVICE_NAME = "consul_service_name"
	KEY_CONSUL_SERVICE_HOST = "consul_service_host"

	DEFAULT_HTTP_PORT    = 8080
	DEFAULT_DYNAMIC_POST = true
	DEFAULT_HTTP_ROOT    = "http-server-root"

	CONTENT_TYPE_CHARSET  = "charset="
	CONTENT_TYPE_BOUNDARY = "boundary="

	MIME_TYPE_JSON = "application/json"
	MIME_TYPE_YAML = "application/yaml"

	DEFAULT_FILE_EXT = ".json"
)

type (
	HttpServer struct {
		Port              int16
		DynamicRoute      bool
		DBRoot            string
		ConsulApiBase     string
		ConsulServiceName string
		ConsulServiceHost string
		StaticRoutes      RouteMap
	}

	RouteMap    map[string]Route
	HTTP_METHOD enum.Enum
	HTTP_ACTION enum.Enum

	Route struct {
		Path          string
		Method        *HTTP_METHOD
		Action        *HTTP_ACTION
		File          string
		Single        bool
		Id            []string
		Fields        []string
		UniqueNotList bool
		Resolver      HttpFileResolver
	}

	RouteInfoMap map[string]RouteInfo

	RouteInfo struct {
		Path          string   `toml:"path,omitempty"`
		Method        string   `toml:"method,omitempty"`
		Action        string   `toml:"action,omitempty"`
		Format        string   `toml:"format,omitempty"`
		File          string   `toml:"file,omitempty"`
		Single        bool     `toml:"single,omitempty"`
		Id            []string `toml:"id,omitempty"`
		Fields        []string `toml:"fields,omitempty"`
		UniqueNotList bool     `toml:"unique_not_list,omitempty"`
	}

	HttpFileModel struct {
		PostResponse map[string]any `json:"post_response,omitempty"`
		DelResponse  map[string]any `json:"del_response,omitempty"`
		Datum        map[string]any `json:"datum,omitempty"`
		Data         []any          `json:"data,omitempty"`
	}

	HttpFileType struct {
		MimeType       string
		FileExts       []string
		DefaultFileExt string
		Resolver       HttpFileResolver
	}

	HttpFileResolver interface {
		Marshal(v any) ([]byte, error)
		Unmarshal(data []byte, v any) error
		ContentType() string
	}

	JsonFileResolver struct{}

	YamlFileResolver struct{}
)

var (
	HTTP_METHOD_GET    = enum.InitEnum[HTTP_METHOD]("GET", "GET")
	HTTP_METHOD_POST   = enum.InitEnum[HTTP_METHOD]("POST", "POST")
	HTTP_METHOD_PUT    = enum.InitEnum[HTTP_METHOD]("PUT", "PUT")
	HTTP_METHOD_DELETE = enum.InitEnum[HTTP_METHOD]("DELETE", "DELETE")

	HTTP_ACTION_APPEND = enum.InitEnum[HTTP_ACTION]("a", "append")
	HTTP_ACTION_READ   = enum.InitEnum[HTTP_ACTION]("r", "read")
	HTTP_ACTION_WRITE  = enum.InitEnum[HTTP_ACTION]("w", "write")
	HTTP_ACTION_DELETE = enum.InitEnum[HTTP_ACTION]("d", "delete")

	DEFAULT_RESPONSE = map[string]bool{"success": true}

	FILE_EXT_JSON = []string{"json"}
	FILE_EXT_YAML = []string{"yaml", "yml"}

	JsonResolver = JsonFileResolver{}
	YamlResolver = YamlFileResolver{}

	MimeTypeMap = make(map[string]HttpFileType)
	FileExtMap  = make(map[string]HttpFileType)
)

func (resolver JsonFileResolver) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (resolver JsonFileResolver) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (resolver JsonFileResolver) ContentType() string {
	return MIME_TYPE_JSON
}

func (resolver YamlFileResolver) Marshal(v any) ([]byte, error) {
	// TODO
	return nil, nil
}

func (resolver YamlFileResolver) Unmarshal(data []byte, v any) error {
	// TODO
	return nil
}

func (resolver YamlFileResolver) ContentType() string {
	return MIME_TYPE_YAML
}

func init() {
	jsonType := HttpFileType{MimeType: MIME_TYPE_JSON, FileExts: FILE_EXT_JSON, DefaultFileExt: ".json", Resolver: JsonResolver}
	ymlType := HttpFileType{MimeType: MIME_TYPE_YAML, FileExts: FILE_EXT_YAML, DefaultFileExt: ".yml", Resolver: YamlResolver}

	MimeTypeMap[MIME_TYPE_JSON] = jsonType
	MimeTypeMap[MIME_TYPE_YAML] = ymlType

	for _, ext := range FILE_EXT_JSON {
		FileExtMap[ext] = jsonType
	}
	for _, ext := range FILE_EXT_YAML {
		FileExtMap[ext] = ymlType
	}
}

func (ri *RouteInfo) resolve(root string) (key string, route Route, err error) {
	// path
	if !strings.HasPrefix(ri.Path, "/") {
		ri.Path = "/" + ri.Path
	}
	route.Path = ri.Path

	// method
	ri.Method = strings.ToUpper(ri.Method)
	if ri.Method == "" {
		ri.Method = HTTP_METHOD_GET.Code
	}
	route.Method = enum.ParseEnum[HTTP_METHOD](ri.Method)
	key = ri.Path + "_" + ri.Method

	// action
	if ri.Action != "" {
		route.Action = enum.ParseEnum[HTTP_ACTION](ri.Action)
		if route.Action == nil {
			route.Action = route.Method.defaultAction()
		}
	} else {
		route.Action = route.Method.defaultAction()
	}

	// file
	if ri.File != "" {
		route.File = filepath.Join(root, ri.File)
	} else {
		var dbfile string
		if filepath.Ext(ri.Path) == "" {
			if strings.HasSuffix(ri.Path, "/") {
				ri.Path = ri.Path + "index"
			}

			if ri.Format == "" {
				dbfile = ri.Path + DEFAULT_FILE_EXT
			} else {
				dbfile = ri.Path + "." + ri.Format
			}
		} else {
			dbfile = ri.Path
		}

		route.File = filepath.Join(root, dbfile)
	}

	// resolver
	ext, set := strings.ToLower(filepath.Ext(route.File)), false
	if ext == "" {
		if ri.Format == "" {
			set = true
			route.Resolver = JsonResolver
		} else {
			ext = ri.Format
		}
	}

	ext, _ = strings.CutPrefix(ext, ".")

	if !set {
		if r, ok := FileExtMap[ext]; ok {
			route.Resolver = r.Resolver
		} else {
			err = fmt.Errorf("unknown file format: %s", ext)
			return
		}
	}

	// single
	route.Single = ri.Single

	// id
	route.Id = ri.Id

	// fields
	route.Fields = ri.Fields

	// UniqueNotList
	route.UniqueNotList = ri.UniqueNotList

	return
}

func (method *HTTP_METHOD) defaultAction() *HTTP_ACTION {
	switch method {
	case HTTP_METHOD_GET:
		return HTTP_ACTION_READ
	case HTTP_METHOD_POST:
		return HTTP_ACTION_APPEND
	case HTTP_METHOD_PUT:
		return HTTP_ACTION_APPEND
	case HTTP_METHOD_DELETE:
		return HTTP_ACTION_DELETE
	}

	return HTTP_ACTION_READ
}

func parseHttpServer(configPath string) (*HttpServer, error) {
	config := &HttpServer{
		Port:         DEFAULT_HTTP_PORT,
		DynamicRoute: DEFAULT_DYNAMIC_POST,
		DBRoot:       DEFAULT_HTTP_ROOT,
	}

	if !FileExists(configPath) {
		if !FileExists(DEFAULT_HTTP_ROOT) {
			return nil, nil
		}
		return config, nil
	}

	var m map[string]any
	_, err := toml.DecodeFile(configPath, &m)
	if err != nil {
		return nil, err
	}

	// parse generic properties
	if port, ok := m[KEY_HTTP_PORT]; ok {
		config.Port = int16(port.(int64))
		delete(m, KEY_HTTP_PORT)
	}
	if dynamicPost, ok := m[KEY_DYNAMIC_ROUTE]; ok {
		config.DynamicRoute = dynamicPost.(bool)
		delete(m, KEY_DYNAMIC_ROUTE)
	}
	if dbRoot, ok := m[KEY_HTTP_ROOT]; ok {
		config.DBRoot = dbRoot.(string)
		delete(m, KEY_HTTP_ROOT)
	}
	if apiBase, ok := m[KEY_CONSUL_API_BASE]; ok {
		config.ConsulApiBase = apiBase.(string)
		delete(m, KEY_CONSUL_API_BASE)
	}
	if serviceName, ok := m[KEY_CONSUL_SERVICE_NAME]; ok {
		config.ConsulServiceName = serviceName.(string)
		delete(m, KEY_CONSUL_SERVICE_NAME)
	}
	if serviceHost, ok := m[KEY_CONSUL_SERVICE_HOST]; ok {
		config.ConsulServiceHost = serviceHost.(string)
		delete(m, KEY_CONSUL_SERVICE_HOST)
	}

	// parse static routes
	if len(m) == 0 {
		return config, nil
	}

	buf := new(bytes.Buffer)
	err = toml.NewEncoder(buf).Encode(m)
	if err != nil {
		return nil, err
	}

	var rim RouteInfoMap

	_, err = toml.NewDecoder(buf).Decode(&rim)
	if err != nil {
		return nil, err
	}

	if len(rim) == 0 {
		return config, nil
	}

	config.StaticRoutes = make(RouteMap)
	for _, ri := range rim {
		key, route, err := ri.resolve(config.DBRoot)
		if err != nil {
			return nil, err
		}

		if _, ok := config.StaticRoutes[key]; ok {
			return nil, fmt.Errorf("duplicate route path: %s", ri.Path)
		}

		config.StaticRoutes[key] = route
	}

	return config, nil
}

func (route Route) ServHTTP(w http.ResponseWriter, r *http.Request, values url.Values) {
	switch route.Action {
	case HTTP_ACTION_READ:
		route.ServRead(w, r, values)
	case HTTP_ACTION_WRITE:
		route.ServWrite(w, r, values)
	case HTTP_ACTION_APPEND:
		route.ServAppend(w, r, values)
	case HTTP_ACTION_DELETE:
		route.ServDelete(w, r, values)
	}
}

func (route Route) WriteResponse(w http.ResponseWriter, data any) bool {
	if bytes, err := route.Resolver.Marshal(data); err == nil {
		w.Write(bytes)
		return true
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
}

func (route Route) matchQuery(data []any, values url.Values) []any {
	if len(values) == 0 {
		return data
	}

	matched := make([]any, 0)
	for _, datum := range data {
		datumValue := reflect.ValueOf(datum)
		if datumValue.Kind() != reflect.Map {
			continue
		}

		count := 0
		for key, value := range values {
			mapValue := datumValue.MapIndex(reflect.ValueOf(key))
			for _, val := range value {
				if mapValue.Kind() == reflect.Interface || mapValue.Kind() == reflect.Pointer {
					mapValue = mapValue.Elem()
				}
				if mapValue.String() == val {
					count++
					break
				}
			}
		}

		if count == len(values) {
			matched = append(matched, datum)
		}
	}

	return matched
}

func (route Route) project(data []any) []any {
	if len(route.Fields) == 0 {
		return data
	}

	projected := make([]any, 0)
	for _, datum := range data {
		datumValue := reflect.ValueOf(datum)
		if datumValue.Kind() != reflect.Map {
			projected = append(projected, datum)
			continue
		}

		newDatum := make(map[string]any)
		for _, key := range datumValue.MapKeys() {
			keyVal := key
			if key.Kind() == reflect.Interface || key.Kind() == reflect.Pointer {
				keyVal = key.Elem()
			}

			if slices.Contains(route.Fields, keyVal.String()) {
				newDatum[keyVal.String()] = datumValue.MapIndex(key).Interface()
			}
		}

		projected = append(projected, newDatum)
	}

	return projected
}

func (route Route) ServRead(w http.ResponseWriter, r *http.Request, values url.Values) {
	if !FileExists(route.File) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	bytes, err := os.ReadFile(route.File)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	model := HttpFileModel{}
	if err := route.Resolver.Unmarshal(bytes, &model); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", route.Resolver.ContentType())

	if route.Single {
		route.WriteResponse(w, model.Datum)
	} else {
		list := route.project(route.matchQuery(model.Data, values))
		if route.UniqueNotList && len(list) == 1 {
			route.WriteResponse(w, list[0])
		} else {
			route.WriteResponse(w, list)
		}

	}
}

func (route Route) readModel() (model HttpFileModel, err error) {
	created := false
	if !FileExists(route.File) {
		if _, err = os.Create(route.File); err != nil {
			return
		}
		created = true
	}

	if created {
		model = HttpFileModel{PostResponse: map[string]any{}, DelResponse: map[string]any{}, Datum: map[string]any{}}
	} else {
		var bytes []byte
		bytes, err = os.ReadFile(route.File)
		if err != nil {
			return
		}

		if err = route.Resolver.Unmarshal(bytes, &model); err != nil {
			return
		}
	}

	return
}

func (route Route) doWriteOrAppendData(w http.ResponseWriter, model *HttpFileModel) {
	bytes, err := route.Resolver.Marshal(model)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(route.File, bytes, 0666); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", route.Resolver.ContentType())

	if route.Action == HTTP_ACTION_DELETE {
		if len(model.DelResponse) > 0 {
			route.WriteResponse(w, model.DelResponse)
		} else {
			route.WriteResponse(w, DEFAULT_RESPONSE)
		}
	} else {
		if len(model.PostResponse) > 0 {
			route.WriteResponse(w, model.PostResponse)
		} else {
			route.WriteResponse(w, DEFAULT_RESPONSE)
		}
	}

}

func (route Route) ServWrite(w http.ResponseWriter, r *http.Request, values url.Values) {
	model, err := route.readModel()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	for k := range model.Datum {
		delete(model.Datum, k)
	}

	switch route.Method {
	case HTTP_METHOD_GET:
		fallthrough
	case HTTP_METHOD_DELETE:
		for k, v := range values {
			if len(v) == 1 {
				model.Datum[k] = v[0]
			} else if len(v) > 1 {
				model.Datum[k] = v
			}
		}
	case HTTP_METHOD_POST:
		fallthrough
	case HTTP_METHOD_PUT:
		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := route.Resolver.Unmarshal(bytes, &model.Datum); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	route.doWriteOrAppendData(w, &model)
}

func (route Route) ServAppend(w http.ResponseWriter, r *http.Request, values url.Values) {
	model, err := route.readModel()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	newDatum := make(map[string]any)
	switch route.Method {
	case HTTP_METHOD_GET:
		fallthrough
	case HTTP_METHOD_DELETE:
		for k, v := range values {
			if len(v) == 1 {
				model.Datum[k] = v[0]
			} else if len(v) > 1 {
				model.Datum[k] = v
			}
		}
	case HTTP_METHOD_POST:
		fallthrough
	case HTTP_METHOD_PUT:
		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := route.Resolver.Unmarshal(bytes, &newDatum); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// check if unique
	if len(route.Id) > 0 {
		for _, datum := range model.Data {
			datumValue := reflect.ValueOf(datum)
			if datumValue.Kind() != reflect.Map {
				continue
			}

			unique := false
			for _, key := range route.Id {
				mapValue := datumValue.MapIndex(reflect.ValueOf(key))
				if mapValue.Kind() == reflect.Interface || mapValue.Kind() == reflect.Pointer {
					mapValue = mapValue.Elem()
				}

				if mapValue.String() != newDatum[key] {
					unique = true
					break
				}
			}
			if !unique {
				w.WriteHeader(http.StatusConflict)
				return
			}
		}
	}

	model.Data = append(model.Data, newDatum)

	route.doWriteOrAppendData(w, &model)
}

func (route Route) ServDelete(w http.ResponseWriter, r *http.Request, values url.Values) {
	model, err := route.readModel()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	if len(values) == 0 {
		if len(model.PostResponse) > 0 {
			route.WriteResponse(w, model.DelResponse)
		} else {
			route.WriteResponse(w, DEFAULT_RESPONSE)
		}
		return
	}

	remaining := make([]any, 0)

	for _, datum := range model.Data {
		datumValue := reflect.ValueOf(datum)
		if datumValue.Kind() != reflect.Map {
			remaining = append(remaining, datum)
			continue
		}

		count := 0
		for key, value := range values {
			mapValue := datumValue.MapIndex(reflect.ValueOf(key))
			if mapValue.Kind() == reflect.Interface || mapValue.Kind() == reflect.Pointer {
				mapValue = mapValue.Elem()
			}

			for _, val := range value {
				if mapValue.String() == val {
					count++
					break
				}
			}
		}

		if count != len(values) {
			remaining = append(remaining, datum)
		}
	}

	model.Data = remaining
	route.doWriteOrAppendData(w, &model)

}

func _parseContentType(r *http.Request) (mimeType string) {
	if ctValues, ok := r.Header["Content-Type"]; ok {
		if len(ctValues) == 0 {
			return MIME_TYPE_JSON
		}

		values := strings.Split(strings.ToLower(ctValues[0]), ";")
		mimeType = values[0]
		return
	} else {
		return MIME_TYPE_JSON
	}
}

func (server *HttpServer) staticRouteMatch(r *http.Request) (route Route, values url.Values, ok bool) {
	values = r.URL.Query()
	key := r.URL.Path + "_" + r.Method
	if route, ok = server.StaticRoutes[key]; ok {
		return
	}

	if strings.HasSuffix(r.URL.Path, "/") {
		return route, values, false
	}

	// handle path variable
	path, _ := strings.CutPrefix(r.URL.Path, "/")
	paths := strings.Split(path, "/")
	if len(paths) < 2 {
		return route, values, false
	}

	if len(paths) == 2 {
		if route, ok = server.StaticRoutes["/_"+r.Method]; ok {
			values.Add(paths[0], paths[1])
			return
		}
	}

	cutted := paths[0 : len(paths)-2]
	pvar := make(map[string]string)
	pvar[paths[len(paths)-2]] = paths[len(paths)-1]
	for {
		joinedPath := strings.Join(cutted, "/")
		joinedPath = "/" + joinedPath

		if route, ok = server.StaticRoutes[joinedPath+"_"+r.Method]; ok {
			for k, v := range pvar {
				values.Add(k, v)
			}
			return
		}

		if len(cutted) < 2 {
			break
		}

		pvar[cutted[len(cutted)-2]] = cutted[len(cutted)-1]
		cutted = cutted[0 : len(cutted)-2]
	}

	return route, values, false
}

func (server *HttpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("uri: %s, method: %s", r.RequestURI, r.Method)

	var route Route
	var ok bool
	var values url.Values

	if route, values, ok = server.staticRouteMatch(r); !ok {
		if server.DynamicRoute {
			mimeType := _parseContentType(r)

			route = Route{Path: r.URL.Path}
			route.Method = enum.ParseEnum[HTTP_METHOD](r.Method)
			if route.Method == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			route.Action = route.Method.defaultAction()
			if ft, ok := MimeTypeMap[mimeType]; ok {
				route.Resolver = ft.Resolver
			} else {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}

			if filepath.Ext(r.URL.Path) == "" {
				if ft, found := MimeTypeMap[mimeType]; found {
					route.File = filepath.Join(server.DBRoot, r.URL.Path+ft.DefaultFileExt)
				} else {
					w.WriteHeader(http.StatusUnsupportedMediaType)
					return
				}
			} else {
				route.File = filepath.Join(server.DBRoot, r.URL.Path)
			}

			route.Id = []string{"id"}
			route.Single = false

			values = r.URL.Query()
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	fmt.Println(route, values)

	route.ServHTTP(w, r, values)
}

func (server *HttpServer) Listen() {
	log.Printf("http server config: %v", server)

	go func() {
		var consulClient *consul.ConsulClient
		var consulInstanceId string
		if server.ConsulApiBase != "" && server.ConsulServiceName != "" {
			consulClient = consul.NewClient(server.ConsulApiBase)
			consulInstanceId = server.ConsulServiceName + "-" + strconv.Itoa(rand.Int())
			serviceHost := server.ConsulServiceHost
			if serviceHost == "" {
				serviceHost = "127.0.0.1"
			}
			consulClient.Register(server.ConsulServiceName, consulInstanceId, "/health", serviceHost, int(server.Port), nil, nil)
		}

		http.ListenAndServe(":"+strconv.FormatInt(int64(server.Port), 10), server)

		if server.ConsulApiBase != "" && server.ConsulServiceName != "" {
			consulClient.Deregister(consulInstanceId)
		}
	}()

	log.Printf("http server listen on :%d", server.Port)
}
