package alertmanager

import (
	"time"

	strfmt "github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/api/v2/models"
)

type Alert struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
}

func (a Alert) postableAlert(startsAt, endsAt time.Time) *models.PostableAlert {
	labels := toLabelSet(a.Labels)
	labels["alertname"] = a.Name
	return &models.PostableAlert{
		StartsAt:    strfmt.DateTime(startsAt),
		EndsAt:      strfmt.DateTime(endsAt),
		Annotations: toLabelSet(a.Annotations),
		Alert: models.Alert{
			Labels: labels,
		},
	}
}

func toLabelSet(labels map[string]string) models.LabelSet {
	labelSet := make(models.LabelSet, len(labels))
	for name, value := range labels {
		labelSet[name] = value
	}
	return labelSet
}
