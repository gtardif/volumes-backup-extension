package backend

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/felipecruz91/vackup-docker-extension/internal/log"
	"os"
)

func Load(ctx context.Context, client *client.Client, volumeName, image string) error {
	resp, err := client.ContainerCreate(ctx, &container.Config{
		Image:        image,
		AttachStdout: true,
		AttachStderr: true,
		// remove hidden and not-hidden files and folders:
		// ..?* matches all dot-dot files except '..'
		// .[!.]* matches all dot files except '.' and files whose name begins with '..'
		Cmd: []string{"/bin/sh", "-c", "rm -rf /mount-volume/..?* /mount-volume/.[!.]* /mount-volume/* && cp -Rp /volume-data/. /mount-volume/;"},
	}, &container.HostConfig{
		Binds: []string{
			volumeName + ":" + "/mount-volume",
		},
	}, nil, nil, "")
	if err != nil {
		return err
	}

	if err := client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	var exitCode int64
	statusCh, errCh := client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		log.Infof("status: %#+v\n", status)
		exitCode = status.StatusCode
	}

	out, err := client.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return err
	}

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return fmt.Errorf("container exited with status code %d\n", exitCode)
	}

	err = client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
	if err != nil {
		return err
	}

	return nil
}
