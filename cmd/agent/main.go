package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/memohai/memoh/internal/chat"
	"github.com/memohai/memoh/internal/config"
	ctr "github.com/memohai/memoh/internal/containerd"
	"github.com/memohai/memoh/internal/db"
	dbsqlc "github.com/memohai/memoh/internal/db/sqlc"
	"github.com/memohai/memoh/internal/embeddings"
	"github.com/memohai/memoh/internal/handlers"
	"github.com/memohai/memoh/internal/mcp"
	"github.com/memohai/memoh/internal/memory"
	"github.com/memohai/memoh/internal/models"
	"github.com/memohai/memoh/internal/providers"
	"github.com/memohai/memoh/internal/server"
)

type resolverTextEmbedder struct {
	resolver *embeddings.Resolver
	modelID  string
	dims     int
}

func (e *resolverTextEmbedder) Embed(ctx context.Context, input string) ([]float32, error) {
	result, err := e.resolver.Embed(ctx, embeddings.Request{
		Type:  embeddings.TypeText,
		Model: e.modelID,
		Input: embeddings.Input{Text: input},
	})
	if err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

func (e *resolverTextEmbedder) Dimensions() int {
	return e.dims
}

func collectEmbeddingVectors(ctx context.Context, service *models.Service) (map[string]int, models.GetResponse, models.GetResponse, bool, error) {
	candidates, err := service.ListByType(ctx, models.ModelTypeEmbedding)
	if err != nil {
		return nil, models.GetResponse{}, models.GetResponse{}, false, err
	}
	vectors := map[string]int{}
	var textModel models.GetResponse
	var multimodalModel models.GetResponse
	for _, model := range candidates {
		if model.Dimensions > 0 && model.ModelID != "" {
			vectors[model.ModelID] = model.Dimensions
		}
		if model.IsMultimodal {
			if multimodalModel.ModelID == "" {
				multimodalModel = model
			}
			continue
		}
		if textModel.ModelID == "" {
			textModel = model
		}
	}
	
	hasTextModel := textModel.ModelID != ""
	hasMultimodalModel := multimodalModel.ModelID != ""
	hasAnyModel := hasTextModel || hasMultimodalModel
	
	return vectors, textModel, multimodalModel, hasAnyModel, nil
}

func main() {
	ctx := context.Background()
	cfgPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		log.Fatalf("jwt secret is required")
	}
	jwtExpiresIn, err := time.ParseDuration(cfg.Auth.JWTExpiresIn)
	if err != nil {
		log.Fatalf("invalid jwt expires in: %v", err)
	}

	addr := cfg.Server.Addr
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		addr = value
	}

	factory := ctr.DefaultClientFactory{SocketPath: cfg.Containerd.SocketPath}
	client, err := factory.New(ctx)
	if err != nil {
		log.Fatalf("connect containerd: %v", err)
	}
	defer client.Close()

	service := ctr.NewDefaultService(client, cfg.Containerd.Namespace)
	manager := mcp.NewManager(service, cfg.MCP)

	conn, err := db.Open(ctx, cfg.Postgres)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer conn.Close()
	manager.WithDB(conn)
	queries := dbsqlc.New(conn)
	modelsService := models.NewService(queries)

	pingHandler := handlers.NewPingHandler()
	authHandler := handlers.NewAuthHandler(conn, cfg.Auth.JWTSecret, jwtExpiresIn)
	
	// Initialize chat resolver for both chat and memory operations
	chatResolver := chat.NewResolver(modelsService, queries, 30*time.Second)
	
	// Create LLM client for memory operations using chat provider
	var llmClient memory.LLM
	memoryModel, memoryProvider, err := selectMemoryModel(ctx, modelsService, queries)
	if err != nil {
		log.Fatalf("select memory model: %v\nPlease configure at least one chat model in the database.", err)
	}
	
	log.Printf("Using memory model: %s (provider: %s)", memoryModel.ModelID, memoryProvider.ClientType)
	provider, err := createChatProvider(memoryProvider, 30*time.Second)
	if err != nil {
		log.Fatalf("create memory provider: %v", err)
	}
	llmClient = memory.NewProviderLLMClient(provider, memoryModel.ModelID)
	
	resolver := embeddings.NewResolver(modelsService, queries, 10*time.Second)
	vectors, textModel, multimodalModel, hasModels, err := collectEmbeddingVectors(ctx, modelsService)
	if err != nil {
		log.Fatalf("embedding models: %v", err)
	}
	
	var memoryService *memory.Service
	var memoryHandler *handlers.MemoryHandler
	
	if !hasModels {
		log.Println("WARNING: No embedding models configured. Memory service will not be available.")
		log.Println("You can add embedding models via the /models API endpoint.")
		memoryHandler = handlers.NewMemoryHandler(nil)
	} else {
		if textModel.ModelID == "" {
			log.Println("WARNING: No text embedding model configured. Text embedding features will be limited.")
		}
		if multimodalModel.ModelID == "" {
			log.Println("WARNING: No multimodal embedding model configured. Multimodal embedding features will be limited.")
		}
		
		var textEmbedder embeddings.Embedder
		var store *memory.QdrantStore
		
		if textModel.ModelID != "" && textModel.Dimensions > 0 {
			textEmbedder = &resolverTextEmbedder{
				resolver: resolver,
				modelID:  textModel.ModelID,
				dims:     textModel.Dimensions,
			}
			
			if len(vectors) > 0 {
				store, err = memory.NewQdrantStoreWithVectors(
					cfg.Qdrant.BaseURL,
					cfg.Qdrant.APIKey,
					cfg.Qdrant.Collection,
					vectors,
					time.Duration(cfg.Qdrant.TimeoutSeconds)*time.Second,
				)
				if err != nil {
					log.Fatalf("qdrant named vectors init: %v", err)
				}
			} else {
				store, err = memory.NewQdrantStore(
					cfg.Qdrant.BaseURL,
					cfg.Qdrant.APIKey,
					cfg.Qdrant.Collection,
					textModel.Dimensions,
					time.Duration(cfg.Qdrant.TimeoutSeconds)*time.Second,
				)
				if err != nil {
					log.Fatalf("qdrant init: %v", err)
				}
			}
		}
		
		memoryService = memory.NewService(llmClient, textEmbedder, store, resolver, textModel.ModelID, multimodalModel.ModelID)
		memoryHandler = handlers.NewMemoryHandler(memoryService)
	}
	embeddingsHandler := handlers.NewEmbeddingsHandler(modelsService, queries)
	fsHandler := handlers.NewFSHandler(service, manager, cfg.MCP, cfg.Containerd.Namespace)
	swaggerHandler := handlers.NewSwaggerHandler()
	chatHandler := handlers.NewChatHandler(chatResolver)
	
	// Initialize providers and models handlers
	providersService := providers.NewService(queries)
	providersHandler := handlers.NewProvidersHandler(providersService)
	modelsHandler := handlers.NewModelsHandler(modelsService)
	
	srv := server.NewServer(addr, cfg.Auth.JWTSecret, pingHandler, authHandler, memoryHandler, embeddingsHandler, fsHandler, swaggerHandler, chatHandler, providersHandler, modelsHandler)

	if err := srv.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// selectMemoryModel selects a chat model for memory operations
func selectMemoryModel(ctx context.Context, modelsService *models.Service, queries *dbsqlc.Queries) (models.GetResponse, dbsqlc.LlmProvider, error) {
	// First try to get the memory-enabled model
	memoryModel, err := modelsService.GetByEnableAs(ctx, models.EnableAsMemory)
	if err == nil {
		provider, err := fetchProviderByID(ctx, queries, memoryModel.LlmProviderID)
		if err != nil {
			return models.GetResponse{}, dbsqlc.LlmProvider{}, err
		}
		return memoryModel, provider, nil
	}

	// Fallback to chat model
	chatModel, err := modelsService.GetByEnableAs(ctx, models.EnableAsChat)
	if err == nil {
		provider, err := fetchProviderByID(ctx, queries, chatModel.LlmProviderID)
		if err != nil {
			return models.GetResponse{}, dbsqlc.LlmProvider{}, err
		}
		return chatModel, provider, nil
	}

	// If no enabled models, try to find any chat model
	candidates, err := modelsService.ListByType(ctx, models.ModelTypeChat)
	if err != nil || len(candidates) == 0 {
		return models.GetResponse{}, dbsqlc.LlmProvider{}, fmt.Errorf("no chat models available for memory operations")
	}

	selected := candidates[0]
	provider, err := fetchProviderByID(ctx, queries, selected.LlmProviderID)
	if err != nil {
		return models.GetResponse{}, dbsqlc.LlmProvider{}, err
	}
	return selected, provider, nil
}

// fetchProviderByID fetches a provider by ID
func fetchProviderByID(ctx context.Context, queries *dbsqlc.Queries, providerID string) (dbsqlc.LlmProvider, error) {
	if strings.TrimSpace(providerID) == "" {
		return dbsqlc.LlmProvider{}, fmt.Errorf("provider id missing")
	}
	parsed, err := uuid.Parse(providerID)
	if err != nil {
		return dbsqlc.LlmProvider{}, err
	}
	pgID := pgtype.UUID{Valid: true}
	copy(pgID.Bytes[:], parsed[:])
	return queries.GetLlmProviderByID(ctx, pgID)
}

// createChatProvider creates a chat provider instance
func createChatProvider(provider dbsqlc.LlmProvider, timeout time.Duration) (chat.Provider, error) {
	clientType := strings.ToLower(strings.TrimSpace(provider.ClientType))
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	switch clientType {
	case chat.ProviderOpenAI, chat.ProviderOpenAICompat:
		if strings.TrimSpace(provider.ApiKey) == "" {
			return nil, fmt.Errorf("openai api key is required")
		}
		return chat.NewOpenAIProvider(provider.ApiKey, provider.BaseUrl, timeout)
	case chat.ProviderAnthropic:
		if strings.TrimSpace(provider.ApiKey) == "" {
			return nil, fmt.Errorf("anthropic api key is required")
		}
		return chat.NewAnthropicProvider(provider.ApiKey, timeout)
	case chat.ProviderGoogle:
		if strings.TrimSpace(provider.ApiKey) == "" {
			return nil, fmt.Errorf("google api key is required")
		}
		return chat.NewGoogleProvider(provider.ApiKey, timeout)
	case chat.ProviderOllama:
		return chat.NewOllamaProvider(provider.BaseUrl, timeout)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", clientType)
	}
}
