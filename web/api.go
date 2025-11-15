package web

import (
	"context"
	"errors"
	"grout/models"
	"grout/utils"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var e *echo.Echo

func start() {
	e = echo.New()
	e.HideBanner = true

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{"*"},
	}))

	e.Static("/", "resources/web")

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "Pong!")
	})

	e.GET("/config", func(c echo.Context) error {
		config, err := utils.LoadConfig()
		if err != nil && strings.Contains(err.Error(), "no config file found") {
			return c.JSON(http.StatusNoContent, err)
		} else if err != nil {
			return c.JSON(http.StatusInternalServerError, config)
		}
		return c.JSON(http.StatusOK, config)
	})

	e.POST("/config", func(c echo.Context) error {
		var config *models.Config
		if err := c.Bind(&config); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}

		utils.SaveConfig(config)

		return c.JSON(http.StatusOK, config)
	})

	go func() {
		if err := e.Start(":1337"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			e.Logger.Fatal("shutting down the server")
		}
	}()
}

func stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.Shutdown(ctx)
}
