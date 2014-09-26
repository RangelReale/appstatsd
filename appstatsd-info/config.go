package main

import (
	"github.com/BurntSushi/toml"
)

var Configuration *Config

func init() {
	Configuration = NewConfig()
}

type Config struct {
	InfoPort   int32
	ListenHost string

	MGOHost     string
	MGOPort     string
	MGOUsername string
	MGOPassword string
	MGODBName   string
}

func NewConfig() *Config {
	c := Config{
		InfoPort:    8127,
		MGOHost:     "localhost",
		MGOPort:     "27017",
		MGOUsername: "",
		MGOPassword: "",
		MGODBName:   "appstatsd",
	}
	return &c
}

func (c *Config) Load(configfile string) error {
	_, err := toml.DecodeFile(configfile, c)
	return err
}
