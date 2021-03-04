package alertdog

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/prometheus/alertmanager/template"

	"github.com/errm/alertdog/pkg/alertmanager"
)

type Alertmanager interface {
	Alert(alertmanager.Alert) error
	Resolve(alertmanager.Alert) error
}

type Pagerduty interface {
	ManageEvent(event pagerduty.V2Event) (*pagerduty.V2EventResponse, error)
}

type Alertdog struct {
	AlertmanagerEndpoints []string `yaml:"alertmanager_endpoints"`
	Expected              []*Prometheus
	CheckInterval         time.Duration `yaml:"check_interval"`
	Expiry                time.Duration
	Port                  uint
	PagerDutyKey          string `yaml:"pager_duty_key"`
	PagerDutyRunbookURL   string

	mu           sync.RWMutex
	checkedIn    time.Time
	alertmanager Alertmanager
	pagerduty    Pagerduty
}

func (a *Alertdog) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaultInterval, _ := time.ParseDuration("2m")
	a.CheckInterval = defaultInterval
	defaultExpiry, _ := time.ParseDuration("5m")
	a.Expiry = defaultExpiry
	// https://github.com/prometheus/prometheus/wiki/Default-port-allocations
	a.Port = 9796
	type plain Alertdog
	return unmarshal((*plain)(a))
}

func (a *Alertdog) Setup() {
	a.alertmanager = alertmanager.Alertmanager{Endpoints: a.AlertmanagerEndpoints, Expiry: a.CheckInterval * 2}
	a.pagerduty = PagerdutyClient{}
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
}

func (a *Alertdog) processWatchdog(alert template.Alert) {
	a.CheckIn()
	for _, prometheus := range a.Expected {
		var err error
		switch action := prometheus.CheckIn(alert); action {
		case ActionAlert:
			err = a.alertmanager.Alert(prometheus.Alert)
		case ActionResolve:
			err = a.alertmanager.Resolve(prometheus.Alert)
		}
		if err != nil {
			a.pagerDutyAlert("alertdog:alertmanager-push", "Alertdog cannot push alerts to alertmanager")
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
	for _, prometheus := range a.Expected {
		if action := prometheus.Check(); action == ActionAlert {
			if err := a.alertmanager.Alert(prometheus.Alert); err != nil {
				a.pagerDutyAlert("alertdog:alertmanager-push", "Alertdog cannot push alerts to alertmanager")
			}
		}
	}
	if a.Expired() {
		a.pagerDutyAlert(
			"alertdog:webhook-expiry",
			fmt.Sprintf("Alertdog: didn't receive webhook from alert manager for over %v", a.Expiry),
		)
	} else {
		a.pagerDutyResolve("alertdog:webhook-expiry")
	}
}

func (a *Alertdog) CheckIn() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checkedIn = time.Now()
}

func (a *Alertdog) Expired() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return time.Now().After(a.checkedIn.Add(a.Expiry))
}

func (a *Alertdog) pagerDutyAlert(dedupKey, summary string) {
	log.Println("PagerDuty: ", summary)
	event := pagerduty.V2Event{
		Action:     "trigger",
		RoutingKey: a.PagerDutyKey,
		DedupKey:   dedupKey,
		Payload: &pagerduty.V2Payload{
			Summary:  summary,
			Source:   dedupKey,
			Severity: "critical",
		},
		Images: []interface{}{
			map[string]string{
				"src": "https://github.com/errm/alertdog/raw/main/docs/dog.jpg",
			},
		},
	}
	if a.PagerDutyRunbookURL != "" {
		event.Links = []interface{}{
			map[string]string{
				"text": "Runbook 📕",
				"href": a.PagerDutyRunbookURL,
			},
		}
	}
	if response, err := a.pagerduty.ManageEvent(event); err != nil {
		log.Printf("Error raising alert on pagerduty: %s %+v", err, response)
	}
}

func (a *Alertdog) pagerDutyResolve(dedupKey string) {
	event := pagerduty.V2Event{
		Action:     "resolve",
		RoutingKey: a.PagerDutyKey,
		DedupKey:   dedupKey,
	}
	if response, err := a.pagerduty.ManageEvent(event); err != nil {
		log.Printf("Error resolving alert on pagerduty: %s %+v", err, response)
	}
}

type PagerdutyClient struct{}

func (p PagerdutyClient) ManageEvent(event pagerduty.V2Event) (*pagerduty.V2EventResponse, error) {
	return pagerduty.ManageEvent(event)
}
