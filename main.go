package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	IMAGE = "mritd/shadowsocks:3.3.4-20200409"
)

type ServerConfig struct {
	Name string `yaml:"name"`
	Port string `yaml:"port,omitempty"`
	Key  string `yaml:"key,omitempty"`
}

type ClientConfig struct {
	Name       string `yaml:"name"`
	Port       string `yaml:"port,omitempty"`
	KCP        bool   `yaml:"kcp,omitempty"`
	RemoteIP   string `yaml:"remote_ip,omitempty"`
	RemotePort string `yaml:"remote_port,omitempty"`
	Key        string `yaml:"key,omitempty"`
}

type Config struct {
	Image   string          `yaml:"image,omitempty"`
	Servers []*ServerConfig `yaml:"servers,omitempty"`
	Clients []*ClientConfig `yaml:"clients,omitempty"`
}

type Service struct {
	Image         string            `yaml:"image,omitempty"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Command       string            `yaml:"command,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
}

type Network struct {
	Name   string `yaml:"name"`
	Driver string `yaml:"driver"`
}

type DockerCompose struct {
	Version  string              `yaml:"version"`
	Services map[string]*Service `yaml:"services"`
	Networks map[string]*Network `yaml:"networks"`
}

func main() {
	init := flag.Bool("init", false, "init shadowsocks config file")
	config := flag.String("config", "./shadowsocks.yml", "shadowsocks config file")
	flag.Parse()

	var c Config
	if *init {
		c.Image = IMAGE
		for i := 0; i < 3; i++ {
			server := &ServerConfig{
				Name: fmt.Sprintf("server-%d", i),
				Port: "9000",
				Key:  "12345678",
			}
			c.Servers = append(c.Servers, server)
		}

		for i := 0; i < 3; i++ {
			client := &ClientConfig{
				Name:       fmt.Sprintf("client-%d", i),
				Port:       fmt.Sprintf("%d", 1080+i),
				KCP:        i%2 == 0,
				RemoteIP:   "127.0.0.1",
				RemotePort: "9000",
				Key:        "12345678",
			}
			c.Clients = append(c.Clients, client)
		}

		b, err := yaml.Marshal(&c)
		if err != nil {
			log.Fatal(err)
		}
		err = os.WriteFile(*config, b, 0644)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("init shadowsocks config file [%s] success.", *config)
		return
	}

	b, err := os.ReadFile(*config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	compose := &DockerCompose{
		Version:  "3.5",
		Services: map[string]*Service{},
		Networks: map[string]*Network{},
	}

	if len(c.Servers) > 0 {
		for _, server := range c.Servers {
			service := &Service{
				Image:         c.Image,
				ContainerName: server.Name,
				Ports:         []string{fmt.Sprintf("%s:9000", server.Port), fmt.Sprintf("%s:9000/udp", server.Port)},
				Restart:       "always",
				Networks:      []string{"shadowsocks-server"},
				Command:       fmt.Sprintf(`-m "ss-server" -s "-s 0.0.0.0 -p 9000 -m aes-256-cfb -k %s --fast-open" -x -e "kcpserver" -k "-t 127.0.0.1:9000 -l :9000 --mode fast2 --key %s --crypt aes"`, server.Key, server.Key),
			}
			compose.Services[service.ContainerName] = service
		}
		compose.Networks["shadowsocks-server"] = &Network{
			Name:   "shadowsocks-server-network",
			Driver: "bridge",
		}
	}

	if len(c.Clients) > 0 {
		for _, client := range c.Clients {
			service := &Service{
				Image:         c.Image,
				ContainerName: client.Name,
				Ports:         []string{fmt.Sprintf("%s:1080", client.Port)},
				Restart:       "always",
				Networks:      []string{"shadowsocks-client"},
			}
			if client.KCP {
				service.Environment = map[string]string{
					"SS_MODULE":  "ss-local",
					"SS_CONFIG":  fmt.Sprintf("-s 127.0.0.1 -p 2080 -b 0.0.0.0 -l 1080 -m aes-256-cfb -k %s", client.Key),
					"KCP_FLAG":   "true",
					"KCP_MODULE": "kcpclient",
					"KCP_CONFIG": fmt.Sprintf("-r %s:%s -l :2080 --mode fast2 --key %s --crypt aes", client.RemoteIP, client.RemotePort, client.Key),
				}
			} else {
				service.Environment = map[string]string{
					"SS_MODULE": "ss-local",
					"SS_CONFIG": fmt.Sprintf("-s %s -p %s -b 0.0.0.0 -l 1080 -m aes-256-cfb -k %s", client.RemoteIP, client.RemotePort, client.Key),
				}
			}
			compose.Services[service.ContainerName] = service
		}
		compose.Networks["shadowsocks-client"] = &Network{
			Name:   "shadowsocks-client-network",
			Driver: "bridge",
		}
	}

	b, err = yaml.Marshal(compose)
	if err != nil {
		log.Fatal(err)
	}

	name := "docker-compose.yml"
	err = os.WriteFile(name, b, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("generate shadowsocks docker compose file [%s] success.", name)
}
