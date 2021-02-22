package alertdog

import (
	"time"

	"github.com/errm/alertdog/pkg/alertmanager"
	"github.com/prometheus/alertmanager/template"
)

type AlertAction int

const (
	ActionNone AlertAction = iota
	ActionAlert
	ActionResolve
)

type Prometheus struct {
	MatchLabels  map[string]string `yaml:"match_labels"`
	Expiry       time.Duration
	PagerDutyKey string `yaml:"pager_duty_key"`
	Alert        alertmanager.Alert
	checkedIn    time.Time
	count        uint
}

func (p *Prometheus) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaultExpiry, _ := time.ParseDuration("4m")
	p.Expiry = defaultExpiry
	type plain Prometheus
	return unmarshal((*plain)(p))
}

func (p *Prometheus) CheckIn(alert template.Alert) AlertAction {
	if p.match(alert.Labels) {
		if alert.Status == "firing" {
			p.checkedIn = time.Now()
			p.count += 1
			// Debounce during state change, wait for 2 alerts before resolving
			if p.count == 2 {
				return ActionResolve
			}
		} else {
			p.count = 0
			return ActionAlert
		}
	}
	return ActionNone
}

func (p *Prometheus) Check() AlertAction {
	if p.Expired() {
		p.count = 0
		return ActionAlert
	}
	return ActionNone
}

func (p *Prometheus) match(labels map[string]string) bool {
	for key, value := range p.MatchLabels {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func (p *Prometheus) Expired() bool {
	return time.Now().After(p.checkedIn.Add(p.Expiry))
}
