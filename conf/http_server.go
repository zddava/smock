package conf

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/BurntSushi/toml"
)

type (
	HttpServer struct {
		Port   int16
		Routes []Route
	}

	RouteMap map[string]Route

	Route struct {
		Path   string   `toml:"path,omitempty"`
		Method string   `toml:"method,omitempty"`
		Action string   `toml:"action,omitempty"`
		Format string   `toml:"format,omitempty"`
		File   string   `toml:"file,omitempty"`
		Id     []string `toml:"id,omitempty"`
	}
)

func parseHttpServer(path string) (*HttpServer, error) {
	var m map[string]any
	_, err := toml.DecodeFile(path, &m)
	if err != nil {
		return nil, err
	}

	config := HttpServer{}
	if port, ok := m["port"]; ok {
		config.Port = int16(port.(int64))
		delete(m, "port")
	} else {
		config.Port = 8080
	}

	if len(m) == 0 {
		return &config, nil
	}

	buf := new(bytes.Buffer)
	err = toml.NewEncoder(buf).Encode(m)
	if err != nil {
		return nil, err
	}

	var rm RouteMap

	_, err = toml.NewDecoder(buf).Decode(&rm)
	if err != nil {
		return nil, err
	}

	for _, v := range rm {
		config.Routes = append(config.Routes, v)
	}

	return &config, nil
}

func (server *HttpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, r.RequestURI)
}

func (server *HttpServer) Listen() {
	// TODO 开启监听
	// TODO 读取route
	// TODO 读取db
	// TODO 合并配置
	http.ListenAndServe(":"+strconv.FormatInt(int64(server.Port), 10), server)
}
