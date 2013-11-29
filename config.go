package main

import (
	"github.com/BurntSushi/toml"
)

var Configuration *Config

func init() {
	Configuration = NewConfig()
}

type Config struct {
	StatsdPort      int32
	LogPort         int32
	InfoPort        int32
	ListenHost      string
	ErrorStatistics bool

	MGOHost     string
	MGOPort     string
	MGOUsername string
	MGOPassword string
	MGODBName   string
}

func NewConfig() *Config {
	c := Config{
		StatsdPort:      8125,
		LogPort:         8126,
		InfoPort:        8127,
		ErrorStatistics: true,
		MGOHost:         "localhost",
		MGOPort:         "27017",
		MGOUsername:     "",
		MGOPassword:     "",
		MGODBName:       "appstatsd",
	}
	return &c
}

func (c *Config) Load(configfile string) error {
	_, err := toml.DecodeFile(configfile, Configuration)
	return err
}
