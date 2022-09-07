package handler

import (
	"net/http"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/docker/volumes-backup-extension/internal/backend"
	"github.com/docker/volumes-backup-extension/internal/log"
	"github.com/labstack/echo"
)

func (h *Handler) LoadImage(ctx echo.Context) error {
	ctxReq := ctx.Request().Context()
	volumeName := ctx.Param("volume")
	image := ctx.QueryParam("image")

	if volumeName == "" {
		return ctx.String(http.StatusBadRequest, "volume is required")
	}
	if image == "" {
		return ctx.String(http.StatusBadRequest, "image is required")
	}

	log.Infof("volumeName: %s", volumeName)
	log.Infof("image: %s", image)

	cli, err := h.DockerClient()
	if err != nil {
		log.Error(err)
		return ctx.String(http.StatusInternalServerError, err.Error())
	}
	defer func() {
		h.ProgressCache.Lock()
		delete(h.ProgressCache.m, volumeName)
		h.ProgressCache.Unlock()
		_ = backend.TriggerUIRefresh(ctxReq, cli)
	}()

	h.ProgressCache.Lock()
	h.ProgressCache.m[volumeName] = "load"
	h.ProgressCache.Unlock()

	if err := backend.TriggerUIRefresh(ctxReq, cli); err != nil {
		log.Error(err)
		_ = bugsnag.Notify(err, ctxReq)
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	stoppedContainers, err := backend.StopContainersAttachedToVolume(ctxReq, cli, volumeName)
	if err != nil {
		log.Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Load
	err = backend.Load(ctxReq, cli, volumeName, image)
	if err != nil {
		log.Error(err)
		_ = bugsnag.Notify(err, ctxReq)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Start container(s)
	err = backend.StartContainersAttachedToVolume(ctxReq, cli, stoppedContainers)
	if err != nil {
		log.Error(err)
		_ = bugsnag.Notify(err, ctxReq)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.String(http.StatusOK, "")
}
