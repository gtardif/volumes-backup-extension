package handler

import (
	"bytes"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/felipecruz91/vackup-docker-extension/internal/backend"
	"github.com/felipecruz91/vackup-docker-extension/internal/log"
	"github.com/labstack/echo"
	"net/http"
	"path/filepath"
)

func (h *Handler) ImportTarGzFile(ctx echo.Context) error {
	volumeName := ctx.Param("volume")
	path := ctx.QueryParam("path")

	if volumeName == "" {
		return ctx.String(http.StatusBadRequest, "volume is required")
	}
	if path == "" {
		return ctx.String(http.StatusBadRequest, "path is required")
	}

	filePathDir := filepath.Dir(path)
	fileName := filepath.Base(path)

	log.Infof("volumeName: %s", volumeName)
	log.Infof("path: %s", path)
	log.Infof("filePathDir: %s", filePathDir)
	log.Infof("fileName: %s", fileName)

	// Stop container(s)
	stoppedContainers, err := backend.StopContainersAttachedToVolume(ctx.Request().Context(), h.DockerClient, volumeName)
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Import
	binds := []string{
		volumeName + ":" + "/vackup-volume",
		path + ":" + "/vackup",
	}
	log.Infof("binds: %+v", binds)
	resp, err := h.DockerClient.ContainerCreate(ctx.Request().Context(), &container.Config{
		Image:        "docker.io/library/busybox",
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"/bin/sh", "-c", "tar -xvzf /vackup"},
	}, &container.HostConfig{
		Binds: binds,
	}, nil, nil, "")
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := h.DockerClient.ContainerStart(ctx.Request().Context(), resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	statusCh, errCh := h.DockerClient.ContainerWait(ctx.Request().Context(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	case <-statusCh:
	}

	out, err := h.DockerClient.ContainerLogs(ctx.Request().Context(), resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(out)
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	output := buf.String()

	log.Info(output)

	err = h.DockerClient.ContainerRemove(ctx.Request().Context(), resp.ID, types.ContainerRemoveOptions{})
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Start container(s)
	err = backend.StartContainersAttachedToVolume(ctx.Request().Context(), h.DockerClient, stoppedContainers)
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.String(http.StatusOK, "")
}
