package backend

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/felipecruz91/vackup-docker-extension/internal/log"
	"golang.org/x/sync/errgroup"
	"strings"
	"time"
)

func GetContainersForVolume(ctx context.Context, cli *client.Client, volumeName string) []string {
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("volume", volumeName)),
	})
	if err != nil {
		log.Error(err)
	}

	containerNames := make([]string, 0, len(containers))
	for _, c := range containers {
		containerNames = append(containerNames, strings.TrimPrefix(c.Names[0], "/"))
	}

	return containerNames
}

func StopContainersAttachedToVolume(ctx context.Context, cli *client.Client, volumeName string) ([]string, error) {
	var stoppedContainersByExtension []string
	var timeout = 10 * time.Second

	containerNames := GetContainersForVolume(ctx, cli, volumeName)

	g, gCtx := errgroup.WithContext(ctx)
	for _, containerName := range containerNames {
		containerName := containerName
		g.Go(func() error {
			// if the container linked to this volume is running then it must be stopped to ensure data integrity
			containers, err := cli.ContainerList(gCtx, types.ContainerListOptions{
				Filters: filters.NewArgs(filters.Arg("name", containerName)),
			})
			if err != nil {
				return err
			}

			if len(containers) != 1 {
				log.Infof("container %s is not running, no need to stop it", containerName)
				return nil
			}

			log.Infof("stopping container %s...", containerName)
			err = cli.ContainerStop(gCtx, containerName, &timeout)
			if err != nil {
				return err
			}

			log.Infof("container %s stopped", containerName)
			stoppedContainersByExtension = append(stoppedContainersByExtension, containerName)
			return nil
		})
	}

	return containerNames, g.Wait()
}

func StartContainersAttachedToVolume(ctx context.Context, cli *client.Client, containers []string) error {
	g, gCtx := errgroup.WithContext(ctx)

	for _, containerName := range containers {
		containerName := containerName
		g.Go(func() error {
			log.Infof("starting container %s...", containerName)
			err := cli.ContainerStart(gCtx, containerName, types.ContainerStartOptions{})
			if err != nil {
				return err
			}

			log.Infof("container %s started", containerName)
			return nil
		})
	}

	return g.Wait()
}
