package docker

import (
	"context"
	"fmt"
	"time"

	nativeDockerClient "github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/docker/docker/api/types/network"

	"go.keploy.io/server/pkg/clients"
)

const (
	kDefaultTimeoutForDockerQuery = 1 * time.Minute
)

type internalDockerClient struct {
	nativeDockerClient.APIClient
	timeoutForDockerQuery time.Duration
	logger                *zap.Logger
}

func NewInternalDockerClient(logger *zap.Logger) (clients.InternalDockerClient, error) {
	dockerClient, err := nativeDockerClient.NewClientWithOpts(nativeDockerClient.FromEnv,
		nativeDockerClient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &internalDockerClient{
		dockerClient,
		kDefaultTimeoutForDockerQuery,
		logger,
	}, nil
}

// ExtractNetworksForContainer returns the list of all the networks that the container is a part of.
// Note that if a user did not explicitly attach the container to a network, the Docker daemon attaches it
// to a network called "bridge".
func (idc *internalDockerClient) ExtractNetworksForContainer(containerName string) (map[string]*network.EndpointSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), idc.timeoutForDockerQuery)
	defer cancel()

	containerJSON, err := idc.ContainerInspect(ctx, containerName)
	if err != nil {
		idc.logger.Error("Could not inspect container via the Docker API", zap.Error(err),
			zap.Any("container_name", containerName))
		return nil, err
	}

	if settings := containerJSON.NetworkSettings; settings != nil {
		return settings.Networks, nil
	} else {
		// Docker attaches the container to "bridge" network by default.
		// If the network list is empty, the docker daemon is possibly misbehaving,
		// or the container is in a bad state.
		idc.logger.Error("The network list for the given container is empty. This is unexpected.",
			zap.Any("container_name", containerName))
		return nil, fmt.Errorf("the container is not attached to any network")
	}
}

func (idc *internalDockerClient) ConnectContainerToNetworks(containerName string, settings map[string]*network.EndpointSettings) error {
	if settings == nil {
		return fmt.Errorf("provided network settings is empty")
	}

	existingNetworks, err := idc.ExtractNetworksForContainer(containerName)
	if err != nil {
		return fmt.Errorf("could not get existing networks for container %s", containerName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), idc.timeoutForDockerQuery)
	defer cancel()

	for networkName, setting := range settings {
		// If the container is already part of this network, skip it.
		_, ok := existingNetworks[networkName]
		if ok {
			continue
		}

		err := idc.NetworkConnect(ctx, networkName, containerName, setting)
		if err != nil {
			return err
		}
	}

	return nil
}
