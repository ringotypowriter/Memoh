package server

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/memohai/memoh/internal/auth"
	"github.com/memohai/memoh/internal/handlers"
)

type Server struct {
	echo *echo.Echo
	addr string
}

func NewServer(addr string, jwtSecret string, pingHandler *handlers.PingHandler, authHandler *handlers.AuthHandler, memoryHandler *handlers.MemoryHandler, embeddingsHandler *handlers.EmbeddingsHandler, fsHandler *handlers.FSHandler, swaggerHandler *handlers.SwaggerHandler, chatHandler *handlers.ChatHandler, providersHandler *handlers.ProvidersHandler, modelsHandler *handlers.ModelsHandler) *Server {
	if addr == "" {
		addr = ":8080"
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLogger())
	e.Use(auth.JWTMiddleware(jwtSecret, func(c echo.Context) bool {
		path := c.Request().URL.Path
		if path == "/ping" || path == "/api/swagger.json" || path == "/auth/login" {
			return true
		}
		if strings.HasPrefix(path, "/api/docs") {
			return true
		}
		return false
	}))

	if pingHandler != nil {
		pingHandler.Register(e)
	}
	if authHandler != nil {
		authHandler.Register(e)
	}
	if memoryHandler != nil {
		memoryHandler.Register(e)
	}
	if embeddingsHandler != nil {
		embeddingsHandler.Register(e)
	}
	if fsHandler != nil {
		fsHandler.Register(e)
	}
	if swaggerHandler != nil {
		swaggerHandler.Register(e)
	}
	if chatHandler != nil {
		chatHandler.Register(e)
	}
	if providersHandler != nil {
		providersHandler.Register(e)
	}
	if modelsHandler != nil {
		modelsHandler.Register(e)
	}

	return &Server{
		echo: e,
		addr: addr,
	}
}

func (s *Server) Start() error {
	return s.echo.Start(s.addr)
}
