package alertdog

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/template"
	"github.com/stretchr/testify/mock"

	"github.com/errm/alertdog/pkg/alertmanager"
)

type AlertmanagerMock struct {
	mock.Mock
}

func (a *AlertmanagerMock) Alert(alert alertmanager.Alert) error {
	args := a.Called(alert)
	return args.Error(0)
}

func (a *AlertmanagerMock) Resolve(alert alertmanager.Alert) error {
	args := a.Called(alert)
	return args.Error(0)
}

type PagerdutyMock struct {
	alertCalls   []string
	resolveCalls []string
}

func (p *PagerdutyMock) Alert(dedupKey, summary string) {
	p.alertCalls = append(p.alertCalls, dedupKey)
}

func (p *PagerdutyMock) Resolve(dedupKey string) {
	p.resolveCalls = append(p.resolveCalls, dedupKey)
}

type alertExpectation struct {
	alert alertmanager.Alert
	err   error
}

func assertPagerduty(t *testing.T, pd *PagerdutyMock, wantAlerted, wantResolved bool) {
	t.Helper()
	if wantAlerted && len(pd.alertCalls) == 0 {
		t.Error("expected pagerduty Alert to be called, but it was not")
	}
	if !wantAlerted && len(pd.alertCalls) > 0 {
		t.Errorf("expected no pagerduty Alert calls, got %v", pd.alertCalls)
	}
	if wantResolved && len(pd.resolveCalls) == 0 {
		t.Error("expected pagerduty Resolve to be called, but it was not")
	}
	if !wantResolved && len(pd.resolveCalls) > 0 {
		t.Errorf("expected no pagerduty Resolve calls, got %v", pd.resolveCalls)
	}
}

func TestProcessWatchdog(t *testing.T) {
	alert1 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "one",
		},
	}

	prom1 := &Prometheus{
		MatchLabels: map[string]string{
			"alertname":  "Watchdog",
			"prometheus": "prom1",
		},
		Alert: alert1,
	}

	alert2 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "two",
		},
	}

	prom2 := &Prometheus{
		MatchLabels: map[string]string{
			"alertname":  "Watchdog",
			"prometheus": "prom2",
		},
		Alert: alert2,
	}

	alertmanagerError := errors.New("alertmanager is broken")

	var tests = []struct {
		description           string
		alertExpectations     []alertExpectation
		resolveExpectations   []alertExpectation
		watchdogs             []template.Alert
		wantPagerdutyAlerted  bool
		wantPagerdutyResolved bool
	}{
		{
			description:           "When we receive a resolved watchdog: alert with the correct alert",
			alertExpectations:     []alertExpectation{{alert: alert1}},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
		{
			description:           "Fire the correct alert based on labels",
			alertExpectations:     []alertExpectation{{alert: alert2}},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
			},
		},
		{
			description:           "Don't care about extra labels",
			alertExpectations:     []alertExpectation{{alert: alert2}},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
						"foo":        "bar",
					},
				},
			},
		},
		{
			description: "Don't do anything with watchdogs that don't match",
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom33",
					},
				},
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom33",
					},
				},
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom33",
					},
				},
			},
		},
		{
			description:           "When we receive a firing watchdog twice: resolve the correct alert",
			resolveExpectations:   []alertExpectation{{alert: alert1}},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
		{
			description:          "When alertmanager errors, raise a pagerduty event",
			alertExpectations:    []alertExpectation{{alert: alert1, err: alertmanagerError}},
			wantPagerdutyAlerted: true,
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
		{
			description: "Don't flap on a single watchdog",
			// This can happen when prometheus goes down
			// if the watchdog resolves slightly earlier
			// on one alertmanager in a ha pair.
			alertExpectations:     []alertExpectation{{alert: alert2}},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			alertmanagerMock := &AlertmanagerMock{}
			alertmanagerMock.Test(t)
			pagerdutyMock := &PagerdutyMock{}

			alertdog := Alertdog{
				Expected:            []*Prometheus{prom1, prom2},
				PagerDutyKey:        "pagerduty-key",
				PagerDutyRunbookURL: "https://example.org/runbook-url",
				alertmanager:        alertmanagerMock,
				pagerduty:           pagerdutyMock,
			}

			for _, e := range test.alertExpectations {
				alertmanagerMock.On("Alert", e.alert).Return(e.err)
			}
			for _, e := range test.resolveExpectations {
				alertmanagerMock.On("Resolve", e.alert).Return(e.err)
			}

			for _, watchdog := range test.watchdogs {
				alertdog.processWatchdog(watchdog)
			}

			alertmanagerMock.AssertExpectations(t)
			assertPagerduty(t, pagerdutyMock, test.wantPagerdutyAlerted, test.wantPagerdutyResolved)
		})
	}
}

func TestCheck(t *testing.T) {
	alert1 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "one",
		},
	}

	alert2 := alertmanager.Alert{
		Labels: map[string]string{
			"alert": "two",
		},
	}

	var tests = []struct {
		description           string
		alertExpectations     []alertExpectation
		wantPagerdutyAlerted  bool
		wantPagerdutyResolved bool
		watchdogs             []template.Alert
	}{
		{
			description: "If no watchdogs are received, then fire all alerts, and raise a pagerduty incident",
			alertExpectations: []alertExpectation{
				{alert: alert1},
				{alert: alert2},
			},
			wantPagerdutyAlerted:  true,
			wantPagerdutyResolved: true,
		},
		{
			description:           "Fire the alert if the watchdog was missing",
			alertExpectations:     []alertExpectation{{alert: alert1}},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
			},
		},
		{
			description:           "Don't fire if watchdogs were received",
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
				{
					Status: "firing",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
		{
			description: "Fire if only resolves where received",
			alertExpectations: []alertExpectation{
				{alert: alert1},
				{alert: alert2},
			},
			wantPagerdutyResolved: true,
			watchdogs: []template.Alert{
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom2",
					},
				},
				{
					Status: "resolved",
					Labels: template.KV{
						"alertname":  "Watchdog",
						"prometheus": "prom1",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			alertmanagerMock := &AlertmanagerMock{}
			alertmanagerMock.Test(t)
			pagerdutyMock := &PagerdutyMock{}

			alertdog := Alertdog{
				Expected: []*Prometheus{
					{
						MatchLabels: map[string]string{
							"alertname":  "Watchdog",
							"prometheus": "prom1",
						},
						Alert:  alert1,
						Expiry: time.Minute,
					},
					{
						MatchLabels: map[string]string{
							"alertname":  "Watchdog",
							"prometheus": "prom2",
						},
						Alert:  alert2,
						Expiry: time.Minute,
					},
				},
				PagerDutyKey: "this-is-a-key",
				Expiry:       time.Minute * 2,
				pagerduty:    pagerdutyMock,
				alertmanager: alertmanagerMock,
			}

			for _, e := range test.alertExpectations {
				alertmanagerMock.On("Alert", e.alert).Return(e.err)
			}

			for _, watchdog := range test.watchdogs {
				alertdog.processWatchdog(watchdog)
			}

			alertdog.Check()
			alertmanagerMock.AssertExpectations(t)
			assertPagerduty(t, pagerdutyMock, test.wantPagerdutyAlerted, test.wantPagerdutyResolved)
		})
	}
}
