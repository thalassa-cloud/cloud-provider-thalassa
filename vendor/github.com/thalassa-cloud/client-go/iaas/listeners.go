package iaas

import (
	"context"
	"fmt"
	"time"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/pkg/client"
)

// ListListeners lists all listeners for a specific loadbalancer.
func (c *Client) ListListeners(ctx context.Context, listRequest *ListLoadbalancerListenersRequest) ([]VpcLoadbalancerListener, error) {
	if listRequest == nil {
		return nil, fmt.Errorf("listRequest is required")
	}
	if listRequest.Loadbalancer == "" {
		return nil, fmt.Errorf("loadbalancer is required")
	}

	listeners := []VpcLoadbalancerListener{}
	req := c.R().SetResult(&listeners)

	if listRequest != nil {
		for _, filter := range listRequest.Filters {
			for k, v := range filter.ToParams() {
				req = req.SetQueryParam(k, v)
			}
		}
	}

	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s/listeners", LoadbalancerEndpoint, listRequest.Loadbalancer))
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return listeners, err
	}
	return listeners, nil
}

// GetListener retrieves a specific loadbalancer listener by its identity.
func (c *Client) GetListener(ctx context.Context, getRequest GetLoadbalancerListenerRequest) (*VpcLoadbalancerListener, error) {
	if getRequest.Loadbalancer == "" {
		return nil, fmt.Errorf("loadbalancer is required")
	}
	if getRequest.Listener == "" {
		return nil, fmt.Errorf("listener is required")
	}

	var listener *VpcLoadbalancerListener
	req := c.R().SetResult(&listener)
	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s/listeners/%s", LoadbalancerEndpoint, getRequest.Loadbalancer, getRequest.Listener))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return listener, err
	}
	return listener, nil
}

// CreateListener creates a new loadbalancer listener.
func (c *Client) CreateListener(ctx context.Context, loadbalancerID string, create CreateListener) (*VpcLoadbalancerListener, error) {
	var listener *VpcLoadbalancerListener
	req := c.R().
		SetBody(create).SetResult(&listener)

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/listeners", LoadbalancerEndpoint, loadbalancerID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return listener, err
	}
	return listener, nil
}

// UpdateListener updates an existing loadbalancer listener.
func (c *Client) UpdateListener(ctx context.Context, loadbalancerID string, listenerID string, update UpdateListener) (*VpcLoadbalancerListener, error) {
	var listener *VpcLoadbalancerListener
	req := c.R().
		SetBody(update).SetResult(&listener)

	resp, err := c.Do(ctx, req, client.PUT, fmt.Sprintf("%s/%s/listeners/%s", LoadbalancerEndpoint, loadbalancerID, listenerID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return listener, err
	}
	return listener, nil
}

// DeleteListener deletes a specific loadbalancer listener by its identity.
func (c *Client) DeleteListener(ctx context.Context, loadbalancerID string, listenerID string) error {
	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s/listeners/%s", LoadbalancerEndpoint, loadbalancerID, listenerID))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

type VpcLoadbalancerListener struct {
	Identity      string      `json:"identity"`
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Description   string      `json:"description"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	ObjectVersion int         `json:"objectVersion"`
	Labels        Labels      `json:"labels,omitempty"`
	Annotations   Annotations `json:"annotations,omitempty"`

	Port           int                         `json:"port"`
	Protocol       LoadbalancerProtocol        `json:"protocol"`
	TargetGroup    *VpcLoadbalancerTargetGroup `json:"targetGroup"`
	TargetGroupId  int                         `json:"targetGroupId"`
	AllowedSources []string                    `json:"allowedSources"`
}

type CreateListener struct {
	Name           string               `json:"name"`
	Identity       string               `json:"identity"`
	Description    string               `json:"description"`
	Labels         Labels               `json:"labels,omitempty"`
	Annotations    Annotations          `json:"annotations,omitempty"`
	Port           int                  `json:"port"`
	Protocol       LoadbalancerProtocol `json:"protocol"`
	TargetGroup    string               `json:"targetGroup"`
	AllowedSources []string             `json:"allowedSources,omitempty"`
}

type UpdateListener struct {
	Name        string               `json:"name"`
	Identity    string               `json:"identity"`
	Description string               `json:"description"`
	Labels      Labels               `json:"labels,omitempty"`
	Annotations Annotations          `json:"annotations,omitempty"`
	Port        int                  `json:"port"`
	Protocol    LoadbalancerProtocol `json:"protocol"`
	TargetGroup string               `json:"targetGroup"`
}

type ListLoadbalancerListenersRequest struct {
	Loadbalancer string
	Filters      []filters.Filter
}

type GetLoadbalancerListenerRequest struct {
	Loadbalancer string
	Listener     string
}
