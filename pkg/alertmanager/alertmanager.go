package alertmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/alertmanager/api/v2/models"
)

type Alertmanager struct {
	Endpoints      []string
	Expiry         time.Duration
	RequestTimeout time.Duration
}


func (a Alertmanager) Alert(alert Alert) error {
	now := time.Now()
	return a.push(models.PostableAlerts{alert.postableAlert(now, now.Add(a.Expiry))})
}

func (a Alertmanager) Resolve(alert Alert) error {
	now := time.Now()
	return a.push(models.PostableAlerts{alert.postableAlert(now, now)})
}

func (a Alertmanager) push(alerts models.PostableAlerts) error {
	b, err := json.Marshal(alerts)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.RequestTimeout)
	defer cancel()

	var (
		pushes atomic.Int64
		wg     sync.WaitGroup
	)

	for _, endpoint := range a.Endpoints {
		wg.Add(1)

		go func(url string) {
			defer wg.Done()
			if err := pushToAlertmanager(ctx, url, b); err != nil {
				log.Printf("Error pushing alert to %s - %s", url, err)
				return
			}
			pushes.Add(1)
		}(endpoint)
	}

	wg.Wait()

	if pushes.Load() < 1 {
		return errors.New("failed to push alert to any alertmanager")
	}
	return nil
}

func pushToAlertmanager(ctx context.Context, url string, b []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+"/api/v2/alerts", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad response status %s", resp.Status)
	}
	return nil
}
