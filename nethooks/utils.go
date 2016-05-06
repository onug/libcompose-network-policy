package nethooks

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/go-connections/nat"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
)

type imageInfo struct {
	portID    int
	protoName string
}

var dockerCl *client.Client

func initDockerClient() error {
	var err error

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = client.DefaultDockerHost
	}

	defaultHeaders := map[string]string{"User-Agent": "deploy-client"}

	dockerCl, err = client.NewClient(dockerHost, "v1.21", nil, defaultHeaders)
	if err != nil {
		return err
	}

	return nil
}

func getImageInfo(imageName string) ([]nat.Port, error) {
	imageInfoList := []nat.Port{}

	if err := initDockerClient(); err !=nil {
		log.Errorf("Unable to connect to docker: %s", err)
		return imageInfoList, err
	}

	imageInfo, _, err := dockerCl.ImageInspectWithRaw(context.Background(), imageName, false)
	log.Debugf("Got the following container info %#v", imageInfo.ContainerConfig)

	if err != nil {
		log.Errorf("Unable to inspect image '%s'. Error %v", imageName, err)
		return imageInfoList, err
	}

	for port := range imageInfo.ContainerConfig.ExposedPorts {
		log.Infof("  Fetched port/protocol) = %s/%s from image", port.Proto(), port.Port())
		imageInfoList = append(imageInfoList, port)
	}

	return imageInfoList, nil
}

func getSelfId() (string, error) {
	output, err := exec.Command("/usr/bin/id", "-u", "-n").CombinedOutput()
	if err != nil {
		log.Errorf("Unable to fetch the user id. Error %v", err)
		return "", err
	}
	userId := strings.TrimSpace(string(output))
	return userId, nil
}

func getDnsInfo(targetNetwork, tenant string) (string, error) {
	dnsContName := tenant + "dns"
	if tenant != TENANT_DEFAULT {
		targetNetwork = targetNetwork + "/" + tenant
	}
	if err := initDockerClient(); err !=nil {
		log.Errorf("Unable to connect to docker: %s", err)
		return "", err
	}

	containerInfo, err := dockerCl.ContainerInspect(context.Background(), dnsContName)

	if err != nil {
		log.Errorf("Unable to inspect container '%s': %s", dnsContName, err)
		return "", err
	}

	if len (containerInfo.NetworkSettings.Networks) == 0 {
		return "", errors.New("No endpoints found; Are Networks Configured?")
	}

	for networkName, endPointInfo := range containerInfo.NetworkSettings.Networks {
		if networkName == targetNetwork {
			return endPointInfo.IPAddress, nil
		}
	}

	return "", errors.New("DNS Server IP not found")
}
