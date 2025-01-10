package base

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type HiLogConf struct {
	Level      string `yaml:"level"`
	File       string `yaml:"log_file"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Env        string `yaml:"env"`
}

type HiAgentConf struct {
	S5Port int       `yaml:"s5Port"`
	HPort  int       `yaml:"httpPort"`
	Url    string    `yaml:"url"`
	Log    HiLogConf `yaml:"logger"`
}

type HiServerConf struct {
	Log    HiLogConf `yaml:"logger"`
	Port   int       `yaml:"port"`
	WsPort int       `yaml:"wsPort"`
	Crt    string    `yaml:"crt"`
	Key    string    `yaml:"key"`
}

func (hconf *HiAgentConf) ReadConf(path string) {
	ymlFile, err := os.ReadFile(path)
	if err != nil {
		log.Printf("ReadFile failed:%s", err.Error())
		panic(err)
	}
	err = yaml.Unmarshal(ymlFile, hconf)
	if err != nil {
		log.Printf("Unmarshal failed:%s", err.Error())
		panic(err)
	}
}

func (hconf *HiServerConf) ReadConf(path string) {
	ymlFile, err := os.ReadFile(path)
	if err != nil {
		log.Printf("ReadFile failed:%s", err.Error())
		panic(err)
	}
	err = yaml.Unmarshal(ymlFile, hconf)
	if err != nil {
		log.Printf("Unmarshal failed:%s", err.Error())
		panic(err)
	}
}
