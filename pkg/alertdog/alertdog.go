package alertdog

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/alertmanager/template"

	"github.com/errm/alertdog/pkg/alertmanager"
	"github.com/errm/alertdog/pkg/pagerduty"
)

type Alertmanager interface {
	Alert(alertmanager.Alert) error
	Resolve(alertmanager.Alert) error
}

type PagerDuty interface {
	Alert(dedupKey, summary string)
	Resolve(dedupKey string)
}

type Alertdog struct {
	AlertmanagerEndpoints []string `yaml:"alertmanager_endpoints"`
	Expected              []*Prometheus
	CheckInterval         time.Duration `yaml:"check_interval"`
	Expiry                time.Duration
	Port                  uint
	PagerDutyKey          string `yaml:"pager_duty_key"`
	PagerDutyRunbookURL   string `yaml:"pagerduty_runbook_url"`

	mu           sync.RWMutex
	checkedIn    time.Time
	alertmanager Alertmanager
	pagerduty    PagerDuty
}

const (
	dedupKeyAlertmanagerPush = "alertdog:alertmanager-push"
	dedupKeyWebhookExpiry    = "alertdog:webhook-expiry"
)

func (a *Alertdog) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// set default values
	a.CheckInterval = 2 * time.Minute
	a.Expiry = 5 * time.Minute
	// https://github.com/prometheus/prometheus/wiki/Default-port-allocations
	a.Port = 9796
	a.PagerDutyKey = os.Getenv("PAGER_DUTY_KEY")
	type plain Alertdog
	return unmarshal((*plain)(a))
}

func (a *Alertdog) Setup() {
	a.alertmanager = alertmanager.Alertmanager{Endpoints: a.AlertmanagerEndpoints, Expiry: a.CheckInterval * 2}
	a.pagerduty = pagerduty.New(a.PagerDutyKey, a.PagerDutyRunbookURL, nil)
	a.PagerDutyKey = ""
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
		switch action := prometheus.CheckIn(alert); action {
		case ActionAlert:
			a.reportPushResult(a.alertmanager.Alert(prometheus.Alert))
		case ActionResolve:
			a.reportPushResult(a.alertmanager.Resolve(prometheus.Alert))
		}
	}
}

func (a *Alertdog) reportPushResult(err error) {
	if err != nil {
		a.pagerduty.Alert(dedupKeyAlertmanagerPush, "Alertdog cannot push alerts to alertmanager")
		return
	}
	a.pagerduty.Resolve(dedupKeyAlertmanagerPush)
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
			a.reportPushResult(a.alertmanager.Alert(prometheus.Alert))
		}
	}
	if a.Expired() {
		a.pagerduty.Alert(
			dedupKeyWebhookExpiry,
			fmt.Sprintf("Alertdog: didn't receive webhook from alert manager for over %v", a.Expiry),
		)
	} else {
		a.pagerduty.Resolve(dedupKeyWebhookExpiry)
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
