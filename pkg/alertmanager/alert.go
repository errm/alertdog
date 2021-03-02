package alertmanager

import (
	"time"

	"github.com/prometheus/alertmanager/client"
)

type Alert struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
}

func (a Alert) clientAlert() client.Alert {
	labels := toLabelSet(a.Labels)
	labels[client.LabelName("alertname")] = client.LabelValue(a.Name)
	return client.Alert{
		Labels:      labels,
		Annotations: toLabelSet(a.Annotations),
		StartsAt:    time.Now(),
	}
}

func toLabelSet(labels map[string]string) client.LabelSet {
	labelSet := make(client.LabelSet, len(labels))
	for name, value := range labels {
		labelSet[client.LabelName(name)] = client.LabelValue(value)
	}
	return labelSet
}
