package hooks

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"go.keploy.io/server/pkg"
	"go.keploy.io/server/pkg/models"
)

var (
	dockerClient     *client.Client
	appDockerNetwork string
	appContainerName string
)

const (
	// TODO : Remove hard-coded container name.
	KeployContainerName = "keploy-v2"
)

func (h *Hook) LaunchUserApplication(appCmd, appContainer, appNetwork string, Delay uint64) error {
	// Supports Linux, Windows

	if appCmd == "" && len(appContainer) == 0 {
		return fmt.Errorf(Emoji + "please provide containerName when trying to run using IDE")
	}

	if appCmd == "" && len(appContainer) != 0 {

		if len(appNetwork) == 0 {
			h.logger.Error("please provide docker network name when running with IDE")
			return fmt.Errorf(Emoji + "docker network name not found")
		}

		h.logger.Debug("User Application is running inside docker using IDE")
		//search for the container and process it
		err := h.processDockerEnv("", appContainer, appNetwork)
		if err != nil {
			return err
		}
		h.logger.Info("User application container fetced successfully")
		return nil
	}

	ok, cmd := h.IsDockerRelatedCmd(appCmd)
	if ok {

		h.logger.Debug("Running user application on Docker", zap.Any("Docker env", cmd))

		if cmd == "docker-compose" {
			if len(appContainer) == 0 {
				h.logger.Error("please provide container name in case of docker-compose file", zap.Any("AppCmd", appCmd))
				return fmt.Errorf(Emoji + "container name not found")
			}

			if len(appNetwork) == 0 {
				h.logger.Error("please provide docker network name in case of docker-compose file", zap.Any("AppCmd", appCmd))
				return fmt.Errorf(Emoji + "docker network name not found")
			}
			h.logger.Debug("", zap.Any("appContainer", appContainer), zap.Any("appNetwork", appNetwork))
		} else if cmd == "docker" {
			var err error
			appContainerName, appDockerNetwork, err = parseDockerCommand(appCmd)
			h.logger.Debug("", zap.String("Parsed container name", appContainerName))
			h.logger.Debug("", zap.String("Parsed docker network", appDockerNetwork))

			if err != nil {
				h.logger.Error("failed to parse container or network name from given docker command", zap.Error(err), zap.Any("AppCmd", appCmd))
				return err
			}

			if len(appContainer) == 0 {

				appContainer = appContainerName
			}

			if len(appNetwork) == 0 {

				appNetwork = appDockerNetwork
			}
		}

		err := h.processDockerEnv(appCmd, appContainer, appNetwork)
		if err != nil {
			return err
		}
	} else { //Supports only linux
		h.logger.Debug("Running user application on Linux", zap.Any("pid of keploy", os.Getpid()))

		// to notify the kernel hooks that the user application command is running in native linux.
		key := 0
		value := false
		h.objects.DockerCmdMap.Update(uint32(key), &value, ebpf.UpdateAny)

		errCh := make(chan error, 1)
		go func() {
			// Recover from panic and gracefully shutdown
			defer h.Recover(pkg.GenerateRandomID())

			err := h.runApp(appCmd, false)
			errCh <- err
		}()

		h.logger.Debug("Waiting for any error from user application")

		// Check if there is an error in the channel without blocking
		select {
		case err := <-errCh:
			if err != nil {
				h.logger.Error("failed to launch the user application", zap.Any("err", err.Error()))
				return err
			}
		default:
			h.logger.Debug("no error found while running user application")
			// No error received yet, continue with further flow
		}
	}

	h.logger.Info("User application started successfully")
	return nil
}

func (h *Hook) processDockerEnv(appCmd, appContainer, appNetwork string) error {

	// to notify the kernel hooks that the user application is related to Docker.
	key := 0
	value := true
	h.objects.DockerCmdMap.Update(uint32(key), &value, ebpf.UpdateAny)

	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, os.Kill, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	dockerClient := makeDockerClient()
	appErrCh := make(chan error, 1)

	//User is using keploy with IDE when appCmd is empty
	if len(appCmd) != 0 {
		go func() {
			// Recover from panic and gracefully shutdown
			defer h.Recover(pkg.GenerateRandomID())

			err := h.runApp(appCmd, true)
			appErrCh <- err
		}()

		select {
		case err := <-appErrCh:
			if err != nil {
				h.logger.Error("failed to launch the user application container", zap.Any("err", err.Error()))
				return err
			}
		default:
			h.logger.Info("no error found while running user application container")
			// No error received yet, continue with further flow
		}
	}

	dockerErrCh := make(chan error, 1)
	done := make(chan bool, 1)

	// listen for the "create container" event in order to send the inode of the container to the kernel
	go func() {
		// Recover from panic and gracefully shutdown
		defer h.Recover(pkg.GenerateRandomID())

		// listen for the docker daemon events
		defer func() {
			h.logger.Debug("exiting from goroutine of docker daemon event listener")
		}()

		endTime := time.Now().Add(30 * time.Second)
		logTicker := time.NewTicker(1 * time.Second)
		defer logTicker.Stop()

		eventFilter := filters.NewArgs()
		eventFilter.Add("type", "container")
		eventFilter.Add("event", "create")

		messages, errs := dockerClient.Events(context.Background(), types.EventsOptions{
			Filters: eventFilter,
		})

		for {
			if time.Now().After(endTime) {
				dockerErrCh <- fmt.Errorf("no container found for :%v", appContainer)
				return
			}

			select {
			case err := <-errs:
				if err != nil && err != context.Canceled {
					h.logger.Error("failed to listen for the docker events", zap.Error(err))
				}
				return
			case <-stopper:
				dockerErrCh <- fmt.Errorf("found sudden interrupt")
				return
			case <-logTicker.C:
				h.logger.Info("still waiting for the container to start.", zap.String("containerName", appContainer))
			case e := <-messages:
				if e.Type == events.ContainerEventType && e.Action == "create" {
					// Fetch container details
					containerDetails, err := dockerClient.ContainerInspect(context.Background(), e.ID)
					if err != nil {
						h.logger.Debug("failed to inspect container", zap.Error(err))
						continue
					}

					// Check if the container's name matches the desired name
					if containerDetails.Name != "/"+appContainer {
						h.logger.Debug("ignoring container creation for unrelated container", zap.String("containerName", containerDetails.Name))
						continue
					}

					h.logger.Debug("container created for desired app", zap.Any("ID", e.ID))

					containerPid := 0
					containerIp := ""
					containerFound := false
					for {
						if time.Now().After(endTime) {
							h.logger.Error("failed to find the user application container", zap.Any("appContainer", appContainer))
							break
						}
						inspect, err := dockerClient.ContainerInspect(context.Background(), appContainer)
						if err != nil {
							// h.logger.Debug(fmt.Sprintf("failed to get inspect:%v", inspect), zap.Error(err))
							continue
						}

						h.logger.Debug("checking for container pid", zap.Any("inspect.State.Pid", inspect.State.Pid))
						if inspect.State.Pid != 0 {
							h.logger.Debug("", zap.Any("inspect.State.Pid", inspect.State.Pid))

							if inspect.NetworkSettings != nil && inspect.NetworkSettings.Networks != nil {
								networkDetails, ok := inspect.NetworkSettings.Networks[appNetwork]
								if ok && networkDetails != nil {
									h.logger.Debug(fmt.Sprintf("the ip of the docker container: %v", networkDetails.IPAddress))
									if models.GetMode() == models.MODE_TEST {
										h.logger.Debug("setting container ip address")
										containerIp = networkDetails.IPAddress
										h.logger.Debug("receiver channel received the ip address", zap.Any("containerIp found", containerIp))
									}
								} else {
									h.logger.Debug("Network details for given network not found", zap.Any("network", appNetwork))
								}
							} else {
								h.logger.Debug("Network settings or networks not available in inspect data")
							}
							containerPid = inspect.State.Pid
							containerFound = true
							h.logger.Debug("container found...")
							break
						}
					}
					if containerFound {
						h.logger.Debug(fmt.Sprintf("the user application container pid: %v", containerPid))
						inode := getInodeNumber([15]int32{int32(containerPid)})
						h.logger.Debug("", zap.Any("user inode", inode))
						// send the inode of the container to ebpf hooks to filter the network traffic
						err := h.SendNameSpaceId(0, inode)
						if err == nil {
							h.logger.Debug("application inode sent to kernel successfully", zap.Any("user inode", inode), zap.Any("time", time.Now().UnixNano()))
						}
						done <- true
						if models.GetMode() == models.MODE_TEST {
							h.userIpAddress <- containerIp
						}
						return
					}
				}
			}
		}
	}()

	select {
	case err := <-dockerErrCh:
		if err != nil {
			h.logger.Error("failed to find the user application container", zap.Any("err", err.Error()))
			return err
		}
	case <-done:
		h.logger.Info("container found and processed successfully", zap.Any("time", time.Now().UnixNano()))
		// No error received yet, continue with further flow
	}

	// Now that the application has started, inject the Keploy container into the application's network.
	networks, err := h.idc.ExtractNetworksForContainer(appContainer)
	if err != nil {
		return fmt.Errorf("could not extract network names for the application container: %s with error [%v]",
			appContainer, err)
	}

	err = h.idc.ConnectContainerToNetworks(KeployContainerName, networks)
	if err != nil {
		return fmt.Errorf("could not inject keploy container to the application's network with error [%v]", err)
	}

	h.logger.Debug("processDockerEnv executed successfully")
	return nil
}

// It runs the application using the given command
func (h *Hook) runApp(appCmd string, isDocker bool) error {
	// Create a new command with your appCmd
	parts := strings.Fields(appCmd)
	cmd := exec.Command(parts[0], parts[1:]...)

	// Set the output of the command
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	h.userAppCmd = cmd

	h.logger.Debug("", zap.Any("executing cmd", cmd.String()))

	// Run the command, this handles non-zero exit code get from application.
	err := cmd.Run()
	if err != nil {
		return err
	}

	//make it debug statement
	return fmt.Errorf(Emoji, "user application exited with zero error code")
}

// It checks if the cmd is related to docker or not, it also returns if its a docker compose file
func (h *Hook) IsDockerRelatedCmd(cmd string) (bool, string) {
	// Check for Docker command patterns
	dockerCommandPatterns := []string{
		"docker-compose ",
		"sudo docker-compose ",
		"docker compose ",
		"sudo docker compose ",
		"docker ",
		"sudo docker ",
	}

	for _, pattern := range dockerCommandPatterns {
		if strings.HasPrefix(strings.ToLower(cmd), pattern) {
			if strings.Contains(pattern, "compose") {
				return true, "docker-compose"
			}
			return true, "docker"
		}
	}

	// Check for Docker Compose file extension
	dockerComposeFileExtensions := []string{".yaml", ".yml"}
	for _, extension := range dockerComposeFileExtensions {
		if strings.HasSuffix(strings.ToLower(cmd), extension) {
			return true, "docker-compose"
		}
	}

	return false, ""
}

func parseDockerCommand(dockerCmd string) (string, string, error) {
	// Regular expression patterns
	containerNamePattern := `--name\s+([^\s]+)`
	networkNamePattern := `(--network|--net)\s+([^\s]+)`

	// Extract container name
	containerNameRegex := regexp.MustCompile(containerNamePattern)
	containerNameMatches := containerNameRegex.FindStringSubmatch(dockerCmd)
	if len(containerNameMatches) < 2 {
		return "", "", fmt.Errorf("failed to parse container name")
	}
	containerName := containerNameMatches[1]

	// Extract network name
	networkNameRegex := regexp.MustCompile(networkNamePattern)
	networkNameMatches := networkNameRegex.FindStringSubmatch(dockerCmd)
	if len(networkNameMatches) < 3 {
		return "", "", fmt.Errorf("failed to parse network name")
	}
	networkName := networkNameMatches[2]

	return containerName, networkName, nil
}

func makeDockerClient() *client.Client {
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf(Emoji, "failed to make a docker client:", err)
	}
	return cli
}

func getInodeNumber(pids [15]int32) uint64 {

	for _, pid := range pids {
		filepath := filepath.Join("/proc", strconv.Itoa(int(pid)), "ns", "pid")

		f, err := os.Stat(filepath)
		if err != nil {
			fmt.Errorf("%v failed to get the inode number or namespace Id:", Emoji, err)
			continue
		}
		// Dev := (f.Sys().(*syscall.Stat_t)).Dev
		Ino := (f.Sys().(*syscall.Stat_t)).Ino
		if Ino != 0 {
			return Ino
		}
	}
	return 0
}

func getSelfInodeNumber() uint64 {
	filepath := filepath.Join("/proc", "self", "ns", "pid")

	f, err := os.Stat(filepath)
	if err != nil {
		log.Fatal(Emoji, "failed to get the self inode number or namespace Id:", err)
	}
	// Dev := (f.Sys().(*syscall.Stat_t)).Dev
	Ino := (f.Sys().(*syscall.Stat_t)).Ino
	if Ino != 0 {
		return Ino
	}
	return 0
}
