package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Configuration contains config options for service
type Configuration struct {
	Address       string
	SpecialBranch string
	MongoURL      string
	SecretKey     string
}

func loadConfig() *Configuration {
	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	configuration := &Configuration{}
	err := decoder.Decode(configuration)
	if err != nil {
		fmt.Println("error:", err)
		return nil
	}
	fmt.Println(configuration)
	return configuration
}

func (c *Configuration) print() {
	fmt.Println("addr:", c.Address, " speacial branch:", c.SpecialBranch,
		" mongo:", c.MongoURL)
}
