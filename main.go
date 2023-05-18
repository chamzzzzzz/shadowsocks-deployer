package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

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

type Config struct {
	Version  string              `yaml:"version"`
	Services map[string]*Service `yaml:"services"`
	Networks map[string]*Network `yaml:"networks"`
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	a := &cli.App{
		Usage: "deploy shadowsocks server and client.",
		Commands: []*cli.Command{
			{
				Name:  "server",
				Usage: "deploy shadowsocks server.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "workdir", Value: filepath.Join(home, ".shadowsocks-deploy", "server"), EnvVars: []string{"SHADOWSOCKS_SERVER_WORKDIR"}},
					&cli.StringFlag{Name: "file", Value: "docker-compose.yml", EnvVars: []string{"SHADOWSOCKS_SERVER_FILE"}},
				},
				Commands: []*cli.Command{
					{
						Name:  "config",
						Usage: "config shadowsocks server.",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "image", Value: "mritd/shadowsocks:3.2.5-20190616", EnvVars: []string{"SHADOWSOCKS_SERVER_IMAGE"}},
							&cli.StringFlag{Name: "container-name", Value: "shadowsocks-server", EnvVars: []string{"SHADOWSOCKS_SERVER_CONTAINER_NAME"}},
							&cli.StringFlag{Name: "port", Value: "9000", EnvVars: []string{"SHADOWSOCKS_SERVER_PORT"}},
							&cli.BoolFlag{Name: "enable-backup-ports", Value: false, EnvVars: []string{"SHADOWSOCKS_SERVER_ENABLE_BACKUP_PORTS"}},
							&cli.StringSliceFlag{Name: "backup-ports", Value: []string{"2047", "3047", "4047", "7527", "8527", "9527"}, EnvVars: []string{"SHADOWSOCKS_SERVER_BACKUP_PORTS"}},
							&cli.StringFlag{Name: "key", EnvVars: []string{"SHADOWSOCKS_SERVER_KEY"}, Required: true},
						},
						Action: configServer,
					},
					{
						Name:   "start",
						Usage:  "start shadowsocks server.",
						Action: startServer,
					},
					{
						Name:   "stop",
						Usage:  "stop shadowsocks server.",
						Action: stopServer,
					},
				},
			},

			{
				Name:  "client",
				Usage: "deploy shadowsocks client.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "workdir", Value: filepath.Join(home, ".shadowsocks-deploy", "client"), EnvVars: []string{"SHADOWSOCKS_CLIENT_WORKDIR"}},
					&cli.StringFlag{Name: "file", Value: "docker-compose.yml", EnvVars: []string{"SHADOWSOCKS_CLIENT_FILE"}},
				},
				Commands: []*cli.Command{
					{
						Name:  "config",
						Usage: "config shadowsocks client.",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "image", Value: "mritd/shadowsocks:3.3.4-20200409", EnvVars: []string{"SHADOWSOCKS_CLIENT_IMAGE"}},
							&cli.StringFlag{Name: "container-name", Value: "shadowsocks-client", EnvVars: []string{"SHADOWSOCKS_CLIENT_CONTAINER_NAME"}},
							&cli.StringFlag{Name: "ip", EnvVars: []string{"SHADOWSOCKS_CLIENT_IP"}},
							&cli.StringFlag{Name: "port", Value: "9000", EnvVars: []string{"SHADOWSOCKS_CLIENT_PORT"}},
							&cli.StringFlag{Name: "qingcloud-ip", EnvVars: []string{"SHADOWSOCKS_CLIENT_QINGCLOUD_IP"}},
							&cli.StringFlag{Name: "qingcloud-port", Value: "9000", EnvVars: []string{"SHADOWSOCKS_CLIENT_QINGCLOUD_PORT"}},
							&cli.StringFlag{Name: "linode-ip", EnvVars: []string{"SHADOWSOCKS_CLIENT_LINODE_IP"}},
							&cli.StringSliceFlag{Name: "linode-ports", Value: []string{"2047", "3047", "4047", "7527", "8527", "9527"}, EnvVars: []string{"SHADOWSOCKS_CLIENT_LINODE_PORTS"}},
							&cli.StringFlag{Name: "key", EnvVars: []string{"SHADOWSOCKS_CLIENT_KEY"}, Required: true},
						},
						Action: configClient,
					},
					{
						Name:   "start",
						Usage:  "start shadowsocks client.",
						Action: startClient,
					},
					{
						Name:   "stop",
						Usage:  "stop shadowsocks client.",
						Action: stopClient,
					},
				},
			},
		},
	}

	err = a.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func configServer(c *cli.Context) error {
	config := &Config{
		Version:  "3.5",
		Services: map[string]*Service{},
		Networks: map[string]*Network{},
	}
	name := c.String("container-name")
	key := c.String("key")
	service := &Service{
		Image:         c.String("image"),
		ContainerName: name,
		Restart:       "always",
		Networks:      []string{name},
		Command:       fmt.Sprintf(`-m "ss-server" -s "-s 0.0.0.0 -p 9000 -m aes-256-cfb -k %s --fast-open" -x -e "kcpserver" -k "-t 127.0.0.1:9000 -l :9000 --mode fast2 --key %s --crypt aes"`, key, key),
	}
	ports := []string{c.String("port")}
	if c.Bool("enable-backup-ports") {
		ports = append(ports, c.StringSlice("backup-ports")...)
	}
	for _, port := range ports {
		if port == "" {
			continue
		}
		service.Ports = append(service.Ports, fmt.Sprintf("%s:9000", port))
		service.Ports = append(service.Ports, fmt.Sprintf("%s:9000/udp", port))
	}
	config.Services[name] = service

	network := &Network{
		Name:   fmt.Sprintf("%s-network", name),
		Driver: "bridge",
	}
	config.Networks[name] = network
	b, err := yaml.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("generate server config success.\n%s", b)

	workdir := c.String("workdir")
	file := c.String("file")
	name = filepath.Join(workdir, file)
	os.MkdirAll(workdir, 0755)
	err = os.WriteFile(name, b, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("write server config file %s success.", name)
	return nil
}

func startServer(c *cli.Context) error {
	cmd := exec.Command("docker-compose", "-f", c.String("file"), "up", "-d")
	cmd.Dir = c.String("workdir")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("output:%s\n", out)
		log.Fatal(err)
	}
	log.Printf("start server success.\n")
	return nil
}

func stopServer(c *cli.Context) error {
	cmd := exec.Command("docker-compose", "-f", c.String("file"), "down")
	cmd.Dir = c.String("workdir")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("output:%s\n", out)
		log.Fatal(err)
	}
	log.Printf("stop server success.\n")
	return nil
}

func configClient(c *cli.Context) error {
	config := &Config{
		Version:  "3.5",
		Services: map[string]*Service{},
		Networks: map[string]*Network{},
	}

	image := c.String("image")
	name := c.String("container-name")
	ip := c.String("ip")
	port := c.String("port")
	kcp := c.Bool("kcp")
	key := c.String("key")
	if ip != "" {
		service := newClientService(image, name, "", "1080", ip, port, key, kcp)
		config.Services[service.ContainerName] = service
	}

	qingcloudIP := c.String("qingcloud-ip")
	qingcloudPort := c.String("qingcloud-port")
	if qingcloudIP != "" {
		service := newClientService(image, name, "qingcloud", "1081", qingcloudIP, qingcloudPort, key, false)
		config.Services[service.ContainerName] = service
	}

	linodeIP := c.String("linode-ip")
	linodePorts := c.StringSlice("linode-ports")
	if linodeIP != "" {
		for i, port := range linodePorts {
			service := newClientService(image, name, "linode", fmt.Sprintf("%d", 1082+i), linodeIP, port, key, true)
			config.Services[service.ContainerName] = service
		}
	}

	network := &Network{
		Name:   fmt.Sprintf("%s-network", name),
		Driver: "bridge",
	}
	config.Networks[name] = network
	b, err := yaml.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("generate client config success.\n%s", b)

	workdir := c.String("workdir")
	file := c.String("file")
	name = filepath.Join(workdir, file)
	os.MkdirAll(workdir, 0755)
	err = os.WriteFile(name, b, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("write client config file %s success.", name)
	return nil
}

func startClient(c *cli.Context) error {
	cmd := exec.Command("docker-compose", "-f", c.String("file"), "up", "-d")
	cmd.Dir = c.String("workdir")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("output:%s\n", out)
		log.Fatal(err)
	}
	log.Printf("start client success.\n")
	return nil
}

func stopClient(c *cli.Context) error {
	cmd := exec.Command("docker-compose", "-f", c.String("file"), "down")
	cmd.Dir = c.String("workdir")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("output:%s\n", out)
		log.Fatal(err)
	}
	log.Printf("stop client success.\n")
	return nil
}

func newClientService(image, name, suffix, port, remoteIP, remotePort, key string, kcp bool) *Service {
	containerName := name
	if suffix != "" {
		containerName = fmt.Sprintf("%s-%s", containerName, suffix)
	}
	containerName = fmt.Sprintf("%s-%s", containerName, port)
	service := &Service{
		Image:         image,
		ContainerName: containerName,
		Restart:       "always",
		Networks:      []string{name},
		Ports:         []string{fmt.Sprintf("%s:1080", port)},
	}
	if kcp {
		service.Environment = map[string]string{
			"SS_MODULE":  "ss-local",
			"SS_CONFIG":  fmt.Sprintf("-s 127.0.0.1 -p 2080 -b 0.0.0.0 -l 1080 -m aes-256-cfb -k %s", key),
			"KCP_FLAG":   "true",
			"KCP_MODULE": "kcpclient",
			"KCP_CONFIG": fmt.Sprintf("-r %s:%s -l :2080 --mode fast2 --key %s --crypt aes", remoteIP, remotePort, key),
		}
	} else {
		service.Environment = map[string]string{
			"SS_MODULE": "ss-local",
			"SS_CONFIG": fmt.Sprintf("-s %s -p %s -b 0.0.0.0 -l 1080 -m aes-256-cfb -k %s", remoteIP, remotePort, key),
		}
	}
	return service
}
