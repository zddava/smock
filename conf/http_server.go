package conf

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/zddava/goext/enum"
	"github.com/zddava/gowrap/json"
)

const (
	KEY_HTTP_PORT     = "port"
	KEY_DYNAMIC_ROUTE = "dynamic_route"
	KEY_HTTP_ROOT     = "db_root"

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
		Port         int16
		DynamicRoute bool
		DBRoot       string
		StaticRoutes RouteMap
	}

	RouteMap    map[string]Route
	HTTP_METHOD enum.Enum
	HTTP_ACTION enum.Enum

	Route struct {
		Path        string
		Method      *HTTP_METHOD
		Action      *HTTP_ACTION
		File        string
		Id          []string
		ContentType string
		Resolver    HttpFileResolver
	}

	RouteInfoMap map[string]RouteInfo

	RouteInfo struct {
		Path   string   `toml:"path,omitempty"`
		Method string   `toml:"method,omitempty"`
		Action string   `toml:"action,omitempty"`
		Format string   `toml:"format,omitempty"`
		File   string   `toml:"file,omitempty"`
		Id     []string `toml:"id,omitempty"`
	}

	HttpFileModel struct {
		PostResponse map[string]any   `json:"post_response,omitempty"`
		DelResponse  map[string]any   `json:"del_response,omitempty"`
		Data         []map[string]any `json:"data,omitempty"`
	}

	HttpFileResolver interface {
		Read() (string, error)
		Write(bytes []byte) error
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

	FILE_EXT_JSON = []string{"json"}
	FILE_EXT_YAML = []string{"yaml", "yml"}

	JsonResolver = JsonFileResolver{}
	YamlResolver = YamlFileResolver{}

	MimeTypeMap = make(map[string]HttpFileResolver)
	FileExtMap  = make(map[string]HttpFileResolver)
	MimeExtMap  = make(map[string]string)
)

func (resolver JsonFileResolver) Read() (string, error) {
	// TODO
	return "", nil
}

func (resolver JsonFileResolver) Write(bytes []byte) error {
	// TODO
	return nil
}

func (resolver YamlFileResolver) Read() (string, error) {
	// TODO
	return "", nil
}

func (resolver YamlFileResolver) Write(bytes []byte) error {
	// TODO
	return nil
}

func init() {
	MimeTypeMap[MIME_TYPE_JSON] = JsonResolver
	MimeTypeMap[MIME_TYPE_YAML] = YamlResolver
	MimeExtMap[MIME_TYPE_JSON] = ".json"
	MimeExtMap[MIME_TYPE_YAML] = ".yml"

	for _, ext := range FILE_EXT_JSON {
		FileExtMap[ext] = JsonResolver
	}

	for _, ext := range FILE_EXT_YAML {
		FileExtMap[ext] = YamlResolver
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
			route.Resolver = r
		} else {
			err = fmt.Errorf("unknown file format: %s", ext)
			return
		}
	}

	// id
	route.Id = ri.Id

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

func (server *HttpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("uri: %s, method: %s, headers: %v", r.RequestURI, r.Method, r.Header)

	key := r.URL.Path + "_" + r.Method
	var route Route
	var ok bool
	mimeType := _parseContentType(r)

	if route, ok = server.StaticRoutes[key]; !ok {
		if server.DynamicRoute {
			route = Route{Path: r.URL.Path}
			route.Method = enum.ParseEnum[HTTP_METHOD](r.Method)
			if route.Method == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			route.Action = route.Method.defaultAction()
			if route.Resolver, ok = MimeTypeMap[mimeType]; !ok {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}
			if filepath.Ext(r.URL.Path) == "" {
				if ext, found := MimeExtMap[mimeType]; found {
					route.File = filepath.Join(server.DBRoot, r.URL.Path+ext)
				} else {
					w.WriteHeader(http.StatusUnsupportedMediaType)
					return
				}
			} else {
				route.File = filepath.Join(server.DBRoot, r.URL.Path)
			}

			route.Id = []string{"id"}
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	log.Println(route)

	switch route.Action {
	case HTTP_ACTION_READ:
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
		if err := json.Unmarshal(bytes, &model); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		filtered := make([]any, 0)
		for paramKey, paramValues := range r.URL.Query() {
			for _, datum := range model.Data {
				if f, ok := datum[paramKey]; ok {
					switch val := f.(type) {
					case string:
						for _, paramValue := range paramValues {
							if val == paramValue {
								filtered = append(filtered, datum)
								break
							}
						}
					default:
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if len(filtered) > 0 {
			if bytes, err := json.Marshal(filtered); err == nil {
				w.Write(bytes)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			if bytes, err := json.Marshal(model.Data); err == nil {
				w.Write(bytes)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	case HTTP_ACTION_WRITE:

	case HTTP_ACTION_APPEND:

	case HTTP_ACTION_DELETE:

	}

}

func (server *HttpServer) Listen() {
	log.Printf("http server config: %v", server)

	go http.ListenAndServe(":"+strconv.FormatInt(int64(server.Port), 10), server)

	log.Printf("http server listen on :%d", server.Port)
}
