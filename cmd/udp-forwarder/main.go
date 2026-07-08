package main

import (
	"flag"
	"log"
	"os"

	"github.com/davidvhk/udp-forwarder/internal/forwarder"
	"github.com/davidvhk/udp-forwarder/internal/udp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	ListenAddress  string   `yaml:"listen_address"`
	DestinationIPs []string `yaml:"destinations"`
	Transparent    bool     `yaml:"transparent"`
	MTU            int      `yaml:"mtu"`
}

func loadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func main() {
	configPath := flag.String("config", "config/config.yaml", "Path to the configuration file")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if config.ListenAddress == "" {
		log.Fatal("Error: listen_address is empty or not specified in config")
	}
	if len(config.DestinationIPs) == 0 {
		log.Fatal("Error: no destinations are specified in config")
	}

	mtu := config.MTU
	if mtu <= 0 {
		mtu = 1500
	}

	log.Printf("Starting forwarder: transparent=%v, mtu=%d", config.Transparent, mtu)

	listener := udp.Listener{}
	forwarder := forwarder.Forwarder{
		Transparent: config.Transparent,
		MTU:         mtu,
	}
	defer forwarder.Close()

	for _, ip := range config.DestinationIPs {
		forwarder.AddDestination(ip)
	}

	err = listener.StartListening(config.ListenAddress, &forwarder)
	if err != nil {
		log.Fatalf("Error starting UDP listener: %v", err)
	}
}
