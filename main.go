package main

import (
	"io/ioutil"
	"log"
	"net/http"

	"gopkg.in/yaml.v2"

	"github.com/errm/alertdog/pkg/alertdog"
)

func main() {
	a := readConfig()
	a.Setup()
	go a.CheckLoop()
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		return
	})
	http.Handle("/webhook", a)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func readConfig() *alertdog.Alertdog {
	var a *alertdog.Alertdog
	configFile, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(configFile, &a)
	if err != nil {
		log.Fatal(err)
	}
	return a
}
