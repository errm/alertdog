package alertmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sync/atomic"

	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/stretchr/testify/require"
)

func TestAlert(t *testing.T) {
	var (
		errc             = make(chan error, 1)
		expected         = make(models.PostableAlerts, 0, 1)
		status1, status2 atomic.Int32
		slow1, slow2     atomic.Bool
	)


	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))

	newHTTPServer := func(status *atomic.Int32, slow *atomic.Bool, checkAlerts func(models.PostableAlerts, models.PostableAlerts) error) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			defer func() {
				if err == nil {
					return
				}
				select {
				case errc <- err:
				default:
				}
			}()
			var alerts models.PostableAlerts
			err = json.NewDecoder(r.Body).Decode(&alerts)
			if err == nil {
				err = checkAlerts(expected, alerts)
			}
			s := int(status.Load())
			w.WriteHeader(s)
			if slow.Load() {
				time.Sleep(500 * time.Millisecond)
			}
			if s == http.StatusOK {
				if _, err := w.Write([]byte("{\"status\":\"success\"}")); err != nil {
					panic(err)
				}
			} else {
				if _, err := w.Write([]byte("{\"status\":\"error\"}")); err != nil {
					panic(err)
				}
			}
		}))
	}

	server1 := newHTTPServer(&status1, &slow1, alertsOK)
	server2 := newHTTPServer(&status2, &slow2, alertsOK)

	defer server1.Close()
	defer server2.Close()

	alertManager := Alertmanager{
		Endpoints:      []string{server1.URL, server2.URL},
		Expiry:         time.Minute,
		RequestTimeout: 100 * time.Millisecond,
	}

	checkNoErr := func() {
		t.Helper()
		select {
		case err := <-errc:
			require.NoError(t, err)
		default:
		}
	}

	expected = append(expected, &models.PostableAlert{
		Alert: models.Alert{
			Labels: models.LabelSet{
				"alertname": "PrometheusAlertFailure",
				"foo":       "bar",
			},
		},
	})

	// Both servers OK
	require.NoError(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting failed unexpectedly")
	checkNoErr()

	// Only one server erring
	status2.Store(int32(http.StatusInternalServerError))
	require.NoError(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting failed unexpectedly")
	checkNoErr()

	// Both servers error
	status1.Store(int32(http.StatusNotFound))
	require.Error(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")
	checkNoErr()

	// Timeout
	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))
	slow1.Store(true)
	slow2.Store(true)
	require.Error(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")
	checkNoErr()

	// Dead server
	server1.Close()
	server2.Close()
	require.Error(t, alertManager.Alert(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Alerting succeeded unexpectedly")

	// Resolve
	server1 = newHTTPServer(&status1, &slow1, resolveOK)
	server2 = newHTTPServer(&status2, &slow2, resolveOK)
	defer server1.Close()
	defer server2.Close()

	alertManager = Alertmanager{
		Endpoints:      []string{server1.URL, server2.URL},
		RequestTimeout: 100 * time.Millisecond,
	}

	status1.Store(int32(http.StatusOK))
	status2.Store(int32(http.StatusOK))
	slow1.Store(false)
	slow2.Store(false)

	require.NoError(t, alertManager.Resolve(Alert{
		Name: "PrometheusAlertFailure",
		Labels: map[string]string{
			"foo": "bar",
		},
	}), "Resolve failed unexpectedly")
}

func alertsOK(expected, actual models.PostableAlerts) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("length mismatch: %v != %v", expected, actual)
	}
	for i, alert := range expected {
		if !labelSetsEqual(alert.Labels, actual[i].Labels) {
			return fmt.Errorf("label mismatch at index %d: %s != %s", i, alert.Labels, actual[i].Labels)
		}
	}
	for _, alert := range actual {
		expectedEndsAt := time.Time(alert.StartsAt).Add(time.Minute)
		if !time.Time(alert.EndsAt).Equal(expectedEndsAt) {
			return fmt.Errorf("expected EndsAt to be %s, was %s", expectedEndsAt, time.Time(alert.EndsAt))
		}
	}
	return nil
}

func resolveOK(expected, actual models.PostableAlerts) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("length mismatch: %v != %v", expected, actual)
	}
	for i, alert := range expected {
		if !labelSetsEqual(alert.Labels, actual[i].Labels) {
			return fmt.Errorf("label mismatch at index %d: %s != %s", i, alert.Labels, actual[i].Labels)
		}
	}
	for _, alert := range actual {
		if !time.Time(alert.EndsAt).Equal(time.Time(alert.StartsAt)) {
			return fmt.Errorf("expected EndsAt to equal StartsAt: %s vs %s", time.Time(alert.EndsAt), time.Time(alert.StartsAt))
		}
	}
	return nil
}

func labelSetsEqual(a, b models.LabelSet) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
