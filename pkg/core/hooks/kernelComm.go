//go:build linux

package hooks

import (
	"context"
	"fmt"

	"github.com/cilium/ebpf"
	"go.keploy.io/server/v2/pkg/core"
	"go.keploy.io/server/v2/pkg/core/hooks/structs"
	"go.keploy.io/server/v2/utils"
	"go.uber.org/zap"
)

//TODO: rename this file.

// Get Used by proxy
func (h *Hooks) Get(_ context.Context, srcPort uint16) (*core.NetworkAddress, error) {
	d, err := h.GetDestinationInfo(srcPort)
	if err != nil {
		return nil, err
	}
	// TODO : need to implement eBPF code to differentiate between different apps
	s, ok := h.sess.Get(0)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	return &core.NetworkAddress{
		AppID:    s.ID,
		Version:  d.IPVersion,
		IPv4Addr: d.DestIP4,
		IPv6Addr: d.DestIP6,
		Port:     d.DestPort,
	}, nil
}

// GetDestinationInfo retrieves destination information associated with a source port.
func (h *Hooks) GetDestinationInfo(srcPort uint16) (*structs.DestInfo, error) {
	h.m.Lock()
	defer h.m.Unlock()
	destInfo := structs.DestInfo{}
	fmt.Println("srcPort", srcPort)
	if err := h.redirectProxyMap.Lookup(srcPort, &destInfo); err != nil {
		return nil, err
	}
	return &destInfo, nil
}

func (h *Hooks) Delete(_ context.Context, srcPort uint16) error {
	return h.CleanProxyEntry(srcPort)
}

func (h *Hooks) CleanProxyEntry(srcPort uint16) error {
	h.m.Lock()
	defer h.m.Unlock()
	err := h.redirectProxyMap.Delete(srcPort)
	if err != nil {
		utils.LogError(h.logger, err, "failed to remove entry from redirect proxy map")
		return err
	}
	h.logger.Debug("successfully removed entry from redirect proxy map", zap.Any("(Key)/SourcePort", srcPort))
	return nil
}

func (h *Hooks) SendAppInfo(appInfo structs.AppInfo) error {
	key := 1234
	err := h.appRegistrationMap.Update(uint64(key), appInfo, ebpf.UpdateAny)
	if err != nil {
		utils.LogError(h.logger, err, "failed to send the app info to the ebpf program")
		return err
	}
	return nil
}

func (h *Hooks) SendAgentInfo(agentInfo structs.AgentInfo) error {
	key := 0
	err := h.agentRegistartionMap.Update(uint32(key), agentInfo, ebpf.UpdateAny)
	if err != nil {
		utils.LogError(h.logger, err, "failed to send the agent info to the ebpf program")
		return err
	}
	return nil
}

func (h *Hooks) SendDockerAppInfo(dockerAppInfo structs.DockerAppInfo) error {
	key := 1234
	err := h.dockerAppRegistrationMap.Update(uint64(key), dockerAppInfo, ebpf.UpdateAny)
	if err != nil {
		utils.LogError(h.logger, err, "failed to send the dockerAppInfo info to the ebpf program")
		return err
	}
	return nil
}
