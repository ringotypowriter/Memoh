package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/memohai/memoh/internal/chat"
)

type ChatHandler struct {
	resolver *chat.Resolver
}

func NewChatHandler(resolver *chat.Resolver) *ChatHandler {
	return &ChatHandler{resolver: resolver}
}

func (h *ChatHandler) Register(e *echo.Echo) {
	group := e.Group("/chat")
	group.POST("", h.Chat)
	group.POST("/stream", h.StreamChat)
}

// Chat godoc
// @Summary Chat with AI
// @Description Send a chat message and get a response. The system will automatically select an appropriate chat model from the database.
// @Tags chat
// @Accept json
// @Produce json
// @Param request body chat.ChatRequest true "Chat request"
// @Success 200 {object} chat.ChatResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /chat [post]
func (h *ChatHandler) Chat(c echo.Context) error {
	var req chat.ChatRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if len(req.Messages) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "messages are required")
	}

	resp, err := h.resolver.Chat(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, resp)
}

// StreamChat godoc
// @Summary Stream chat with AI
// @Description Send a chat message and get a streaming response. The system will automatically select an appropriate chat model from the database.
// @Tags chat
// @Accept json
// @Produce text/event-stream
// @Param request body chat.ChatRequest true "Chat request"
// @Success 200 {object} chat.StreamChunk
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /chat/stream [post]
func (h *ChatHandler) StreamChat(c echo.Context) error {
	var req chat.ChatRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if len(req.Messages) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "messages are required")
	}

	// Set headers for SSE
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// Get streaming channels
	chunkChan, errChan := h.resolver.StreamChat(c.Request().Context(), req)

	// Create a flusher
	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	writer := bufio.NewWriter(c.Response().Writer)

	// Stream chunks
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				// Channel closed, send done message
				writer.WriteString("data: [DONE]\n\n")
				writer.Flush()
				flusher.Flush()
				return nil
			}

			// Marshal chunk to JSON
			data, err := json.Marshal(chunk)
			if err != nil {
				continue
			}

			// Write SSE format
			writer.WriteString(fmt.Sprintf("data: %s\n\n", string(data)))
			writer.Flush()
			flusher.Flush()

		case err := <-errChan:
			if err != nil {
				// Send error as SSE event
				errData := map[string]string{"error": err.Error()}
				data, _ := json.Marshal(errData)
				writer.WriteString(fmt.Sprintf("data: %s\n\n", string(data)))
				writer.Flush()
				flusher.Flush()
				return nil
			}
		}
	}
}
