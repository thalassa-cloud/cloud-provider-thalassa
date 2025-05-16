package iaas

import (
	"context"
	"fmt"
	"time"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/pkg/base"
	"github.com/thalassa-cloud/client-go/pkg/client"
)

const (
	TargetGroupEndpoint = "/v1/loadbalancer-target-groups"
)

// ListTargetGroups lists all loadbalancer target groups for a given organisation.
func (c *Client) ListTargetGroups(ctx context.Context, listRequest *ListTargetGroupsRequest) ([]VpcLoadbalancerTargetGroup, error) {
	targetGroups := []VpcLoadbalancerTargetGroup{}
	req := c.R().SetResult(&targetGroups)

	if listRequest != nil {
		for _, filter := range listRequest.Filters {
			for k, v := range filter.ToParams() {
				req = req.SetQueryParam(k, v)
			}
		}
	}

	resp, err := c.Do(ctx, req, client.GET, TargetGroupEndpoint)
	if err != nil {
		return nil, err
	}

	if err := c.Check(resp); err != nil {
		return targetGroups, err
	}
	return targetGroups, nil
}

// GetTargetGroup retrieves a specific loadbalancer target group by its identity.
func (c *Client) GetTargetGroup(ctx context.Context, getRequest GetTargetGroupRequest) (*VpcLoadbalancerTargetGroup, error) {
	if getRequest.Identity == "" {
		return nil, fmt.Errorf("identity is required")
	}

	var targetGroup *VpcLoadbalancerTargetGroup
	req := c.R().SetResult(&targetGroup)
	resp, err := c.Do(ctx, req, client.GET, fmt.Sprintf("%s/%s", TargetGroupEndpoint, getRequest.Identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return targetGroup, err
	}
	return targetGroup, nil
}

// CreateTargetGroup creates a new loadbalancer target group.
func (c *Client) CreateTargetGroup(ctx context.Context, create CreateTargetGroup) (*VpcLoadbalancerTargetGroup, error) {
	if create.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if create.Vpc == "" {
		return nil, fmt.Errorf("vpc is required")
	}
	if create.TargetPort == 0 {
		return nil, fmt.Errorf("targetPort is required")
	}
	if create.Protocol == "" {
		return nil, fmt.Errorf("protocol is required")
	}

	var targetGroup *VpcLoadbalancerTargetGroup
	req := c.R().
		SetBody(create).SetResult(&targetGroup)

	resp, err := c.Do(ctx, req, client.POST, TargetGroupEndpoint)
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return targetGroup, err
	}
	return targetGroup, nil
}

// UpdateTargetGroup updates an existing loadbalancer target group.
func (c *Client) UpdateTargetGroup(ctx context.Context, update UpdateTargetGroupRequest) (*VpcLoadbalancerTargetGroup, error) {
	if update.Identity == "" {
		return nil, fmt.Errorf("identity is required")
	}
	if update.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	var targetGroup *VpcLoadbalancerTargetGroup
	req := c.R().
		SetBody(update.UpdateTargetGroup).SetResult(&targetGroup)

	resp, err := c.Do(ctx, req, client.PUT, fmt.Sprintf("%s/%s", TargetGroupEndpoint, update.Identity))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return targetGroup, err
	}
	return targetGroup, nil
}

// DeleteTargetGroup deletes a specific loadbalancer target group by its identity.
func (c *Client) DeleteTargetGroup(ctx context.Context, deleteRequest DeleteTargetGroupRequest) error {
	if deleteRequest.Identity == "" {
		return fmt.Errorf("identity is required")
	}

	req := c.R()
	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s", TargetGroupEndpoint, deleteRequest.Identity))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

// SetTargetGroupServerAttachments sets the server attachments for a target group.
// This will replace the existing attachments with the ones provided in the request.
// Note: Any existing attachments not present in the request will be detached.
func (c *Client) SetTargetGroupServerAttachments(ctx context.Context, setRequest TargetGroupAttachmentsBatch) error {
	if setRequest.TargetGroupID == "" {
		return fmt.Errorf("targetGroupID is required")
	}
	req := c.R().SetBody(setRequest)
	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/attachments", TargetGroupEndpoint, setRequest.TargetGroupID))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

// AttachServerToTargetGroup attaches a server to a target group.
func (c *Client) AttachServerToTargetGroup(ctx context.Context, attachRequest AttachTargetGroupRequest) (*LoadbalancerTargetGroupAttachment, error) {
	if attachRequest.ServerIdentity == "" {
		return nil, fmt.Errorf("serverIdentity is required")
	}
	if attachRequest.TargetGroupID == "" {
		return nil, fmt.Errorf("targetGroupID is required")
	}

	var result *LoadbalancerTargetGroupAttachment
	req := c.R().
		SetBody(attachRequest.AttachTarget).SetResult(&result)

	resp, err := c.Do(ctx, req, client.POST, fmt.Sprintf("%s/%s/attachments", TargetGroupEndpoint, attachRequest.TargetGroupID))
	if err != nil {
		return nil, err
	}
	if err := c.Check(resp); err != nil {
		return result, err
	}
	return result, nil
}

// DetachServerFromTargetGroup detaches a server from a target group.
func (c *Client) DetachServerFromTargetGroup(ctx context.Context, detachRequest DetachTargetRequest) error {
	if detachRequest.TargetGroupID == "" {
		return fmt.Errorf("targetGroupID is required")
	}
	if detachRequest.AttachmentID == "" {
		return fmt.Errorf("attachmentID is required")
	}

	req := c.R()

	resp, err := c.Do(ctx, req, client.DELETE, fmt.Sprintf("%s/%s/attachments/%s", TargetGroupEndpoint, detachRequest.TargetGroupID, detachRequest.AttachmentID))
	if err != nil {
		return err
	}
	if err := c.Check(resp); err != nil {
		return err
	}
	return nil
}

type VpcLoadbalancerTargetGroup struct {
	Identity      string      `json:"identity"`
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Description   string      `json:"description"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	ObjectVersion int         `json:"objectVersion"`
	Labels        Labels      `json:"labels,omitempty"`
	Annotations   Annotations `json:"annotations,omitempty"`

	Organisation   *base.Organisation   `json:"organisation"`
	Vpc            *Vpc                 `json:"vpc"`
	TargetPort     int                  `json:"targetPort"`
	Protocol       LoadbalancerProtocol `json:"protocol"`
	TargetSelector map[string]string    `json:"targetSelector"`

	LoadbalancerListeners              []VpcLoadbalancerListener           `json:"loadbalancerListeners"`
	LoadbalancerTargetGroupAttachments []LoadbalancerTargetGroupAttachment `json:"loadbalancerTargetGroupAttachments"`
}

type LoadbalancerTargetGroupAttachment struct {
	Identity                string                      `json:"identity"`
	CreatedAt               time.Time                   `json:"createdAt"`
	LoadbalancerTargetGroup *VpcLoadbalancerTargetGroup `json:"loadbalancerTargetGroup"`
	VirtualMachineInstance  *Machine                    `json:"virtualMachineInstance"`
}

type DetachTargetRequest struct {
	TargetGroupID string `json:"targetGroupID"`
	AttachmentID  string `json:"attachmentID"`
}

type GetTargetGroupRequest struct {
	Identity string
}

type ListTargetGroupsRequest struct {
	Filters []filters.Filter
}

type CreateTargetGroup struct {
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	Labels         Labels               `json:"labels,omitempty"`
	Annotations    Annotations          `json:"annotations,omitempty"`
	Vpc            string               `json:"vpc"`
	TargetPort     int                  `json:"targetPort"`
	Protocol       LoadbalancerProtocol `json:"protocol"`
	TargetSelector map[string]string    `json:"targetSelector,omitempty"`
}

type UpdateTargetGroupRequest struct {
	Identity string
	UpdateTargetGroup
}

type UpdateTargetGroup struct {
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	Labels         Labels               `json:"labels,omitempty"`
	Annotations    Annotations          `json:"annotations,omitempty"`
	TargetPort     int                  `json:"targetPort"`
	Protocol       LoadbalancerProtocol `json:"protocol"`
	TargetSelector map[string]string    `json:"targetSelector,omitempty"`
}

type AttachTarget struct {
	ServerIdentity string `json:"serverIdentity"`
}

type AttachTargetGroupRequest struct {
	TargetGroupID string
	AttachTarget
}

type DeleteTargetGroupRequest struct {
	Identity string
}

type TargetGroupAttachmentsBatch struct {
	TargetGroupID string
	Attachments   []AttachTarget `json:"attachments"`
}
