package pagerduty

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/mock"
)

type ClientMock struct {
	mock.Mock
}

func (c *ClientMock) ManageEvent(event pagerduty.V2Event) (*pagerduty.V2EventResponse, error) {
	args := c.Called(event)
	return &pagerduty.V2EventResponse{}, args.Error(0)
}

func isEvent(action, dedupKey string) interface{} {
	return mock.MatchedBy(func(event pagerduty.V2Event) bool {
		return event.Action == action && event.DedupKey == dedupKey
	})
}

func newDeduper(t *testing.T) (*Deduper, *ClientMock) {
	client := &ClientMock{}
	client.Test(t)
	return New("test-key", "https://example.org/runbook", client), client
}

func TestAlertOnlyTriggersOnce(t *testing.T) {
	d, client := newDeduper(t)
	client.On("ManageEvent", isEvent("trigger", "key1")).Return(nil).Once()

	d.Alert("key1", "something broke")
	d.Alert("key1", "something broke")
	d.Alert("key1", "something broke")

	client.AssertNumberOfCalls(t, "ManageEvent", 1)
	client.AssertExpectations(t)
}

func TestResolveOnlyCalledAfterTrigger(t *testing.T) {
	d, client := newDeduper(t)

	d.Resolve("key1")
	d.Resolve("key1")

	client.AssertNotCalled(t, "ManageEvent", mock.Anything)
}

func TestAlertThenResolve(t *testing.T) {
	d, client := newDeduper(t)
	client.On("ManageEvent", isEvent("trigger", "key1")).Return(nil).Once()
	client.On("ManageEvent", isEvent("resolve", "key1")).Return(nil).Once()

	d.Alert("key1", "something broke")
	d.Resolve("key1")
	d.Resolve("key1")

	client.AssertNumberOfCalls(t, "ManageEvent", 2)
	client.AssertExpectations(t)
}

func TestAlertRetryOnError(t *testing.T) {
	d, client := newDeduper(t)
	client.On("ManageEvent", isEvent("trigger", "key1")).Return(errors.New("pagerduty is down")).Once()
	client.On("ManageEvent", isEvent("trigger", "key1")).Return(nil).Once()

	d.Alert("key1", "something broke")
	d.Alert("key1", "something broke")
	d.Alert("key1", "something broke")

	client.AssertNumberOfCalls(t, "ManageEvent", 2)
	client.AssertExpectations(t)
}

func TestResolveRetryOnError(t *testing.T) {
	d, client := newDeduper(t)
	client.On("ManageEvent", isEvent("trigger", "key1")).Return(nil).Once()
	client.On("ManageEvent", isEvent("resolve", "key1")).Return(errors.New("pagerduty is down")).Once()
	client.On("ManageEvent", isEvent("resolve", "key1")).Return(nil).Once()

	d.Alert("key1", "something broke")
	d.Resolve("key1")
	d.Resolve("key1")

	client.AssertNumberOfCalls(t, "ManageEvent", 3)
	client.AssertExpectations(t)
}

func TestIndependentKeys(t *testing.T) {
	d, client := newDeduper(t)
	client.On("ManageEvent", isEvent("trigger", "key1")).Return(nil).Times(2)
	client.On("ManageEvent", isEvent("trigger", "key2")).Return(nil).Once()
	client.On("ManageEvent", isEvent("resolve", "key1")).Return(nil).Once()
	client.On("ManageEvent", isEvent("resolve", "key2")).Return(nil).Once()

	d.Alert("key1", "problem one") // triggers key1
	d.Alert("key2", "problem two") // triggers key2
	d.Resolve("key1")              // resolves key1
	d.Resolve("key2")              // resolves key2
	d.Alert("key1", "problem one") // re-triggers key1

	client.AssertNumberOfCalls(t, "ManageEvent", 5)
	client.AssertExpectations(t)
}
