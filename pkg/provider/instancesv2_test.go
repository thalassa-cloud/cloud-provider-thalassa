package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iaas"
)

func TestMachineIsShutdown(t *testing.T) {
	tests := []struct {
		name      string
		machine   *iaas.Machine
		shutdown  bool
		expectErr bool
	}{
		{
			name:     "nil machine",
			machine:  nil,
			shutdown: true,
		},
		{
			name:     "running",
			machine:  &iaas.Machine{State: iaas.MachineStateRunning},
			shutdown: false,
		},
		{
			name:     "stopped",
			machine:  &iaas.Machine{Name: "worker-1", State: iaas.MachineStateStopped},
			shutdown: true,
		},
		{
			name:     "deleting",
			machine:  &iaas.Machine{Name: "worker-1", State: iaas.MachineStateDeleting},
			shutdown: true,
		},
		{
			name:     "deleted state",
			machine:  &iaas.Machine{Name: "worker-1", State: iaas.MachineStateDeleted},
			shutdown: true,
		},
		{
			name:     "deleted status",
			machine:  &iaas.Machine{Name: "worker-1", Status: iaas.ResourceStatus{Status: "deleted"}},
			shutdown: true,
		},
		{
			name:      "unknown status",
			machine:   &iaas.Machine{Name: "worker-1", Status: iaas.ResourceStatus{Status: "unknown"}},
			shutdown:  true,
			expectErr: true,
		},
		{
			name:     "ready status without state",
			machine:  &iaas.Machine{Status: iaas.ResourceStatus{Status: "ready"}},
			shutdown: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdown, err := machineIsShutdown(tt.machine)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.shutdown, shutdown)
		})
	}
}
