package main

import (
	"io/ioutil"
	"github.com/BurntSushi/toml"
)

const DefaultConfigPath = "/etc/yig-ftp.toml"

var globalConfig Config

func ParseConfig() error {
	data, err := ioutil.ReadFile(DefaultConfigPath)
	if err != nil {
		if err != nil {
			panic("Cannot open yig-ftp.toml")
		}
	}
	var c Config
	_, err = toml.Decode(string(data), &c)
	if err != nil {
		panic("load yig-ftp.toml error: " + err.Error())
	}
	globalConfig.Host = c.Host
	globalConfig.Port = c.Port
	globalConfig.Endpoint = c.Endpoint
	return nil
}

type Config struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`

	Endpoint  string `toml:"endpoint"` // Domain name of YIG
}
