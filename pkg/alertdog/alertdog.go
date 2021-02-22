package alertdog

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/alertmanager/template"

	"github.com/errm/alertdog/pkg/alertmanager"
)

type Alertmanager interface {
	Alert(alertmanager.Alert) error
	Resolve(alertmanager.Alert) error
}

type Alertdog struct {
	AlertmanagerEndpoints []string `yaml:"alertmanager_endpoints"`
	Expected              []*Prometheus
	CheckInterval         time.Duration
	Expiry                time.Duration

	mu           sync.Mutex
	checkedIn    time.Time
	alertmanager Alertmanager
}

func (a *Alertdog) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaultInterval, _ := time.ParseDuration("2m")
	a.CheckInterval = defaultInterval
	defaultExpiry, _ := time.ParseDuration("5m")
	a.Expiry = defaultExpiry
	type plain Alertdog
	return unmarshal((*plain)(a))
}

func (a *Alertdog) Setup() {
	a.alertmanager = alertmanager.Alertmanager{Endpoints: a.AlertmanagerEndpoints, Expiry: a.CheckInterval * 2}
}
func (a *Alertdog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var data template.Data
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("Webhook body invalid, skipping request: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, alert := range data.Alerts {
		a.processWatchdog(alert)
	}
	w.WriteHeader(http.StatusOK)
	return
}

func (a *Alertdog) processWatchdog(alert template.Alert) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checkedIn = time.Now()

	for _, prometheus := range a.Expected {
		switch action := prometheus.CheckIn(alert); action {
		case ActionAlert:
			if err := a.alertmanager.Alert(prometheus.Alert); err != nil {
				log.Println("could not alert to alertmanager", err)
				//TODO: pagerduty
			}
		case ActionResolve:
			if err := a.alertmanager.Resolve(prometheus.Alert); err != nil {
				log.Println("could not resolve alert on alertmanager", err)
				//TODO: pagerduty
			}
		}
	}
}

func (a *Alertdog) CheckLoop() {
	checkExpiryTicker := time.NewTicker(a.CheckInterval)
	for {
		<-checkExpiryTicker.C
		a.Check()
	}
}

func (a *Alertdog) Check() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, prometheus := range a.Expected {
		if action := prometheus.Check(); action == ActionAlert {
			if err := a.alertmanager.Alert(prometheus.Alert); err != nil {
				log.Println("could not alert to alertmanager", err)
				//TODO: pagerduty
			}
		}
	}
	if a.Expired() {
		log.Println("Didn't get any webhook for over 5m")
		// TODO: alert directly to pagerduty
	} else {
		// TODO: resolve pagerduty
	}
}

func (a *Alertdog) Expired() bool {
	return time.Now().After(a.checkedIn.Add(a.Expiry))
}
