package conf

import (
	"flag"
	"log"
	"os"
)

var (
	httpServer = flag.String("http-server", "http.server.conf", "http server config")
	// TODO
	tcpServer  = flag.String("tcp-server", "tcp.server.conf", "tcp server config")
	udpServer  = flag.String("udp-server", "udp.server.conf", "udp server config")
	tcpClient  = flag.String("tcp-client", "tcp.client.conf", "tcp client config")
	udpClient  = flag.String("udp-client", "udp.client.conf", "udp client config")
	mqttClient = flag.String("mqtt-client", "mqtt.client.conf", "mqtt client config")
)

type (
	ServerConfig interface {
		Listen()
	}

	ClientConfig interface {
		Start()
	}
)

func FileExists(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	return os.IsExist(err)
}

func ParseAndRun() {
	configs := make([]any, 0)

	if FileExists(*httpServer) {
		httpServer, err := parseHttpServer(*httpServer)
		if err != nil {
			log.Printf("parse error: %s", err.Error())
		} else {
			configs = append(configs, httpServer)
		}
	}

	if len(configs) == 0 {
		log.Printf("no conf file found, exiting... %s", "\n\n")
		flag.Usage()
		return
	}

	for _, config := range configs {
		switch config := config.(type) {
		case ServerConfig:
			config.Listen()
		case ClientConfig:
			config.Start()
		}
	}

}
