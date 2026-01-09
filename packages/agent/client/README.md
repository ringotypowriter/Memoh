# Agent CLI

A command-line interface for the personal housekeeper assistant agent.

## Setup

1. Create a `.env` file in the project root (not in this directory) with the following variables:

```env
MODEL=gpt-4o
BASE_URL=https://api.openai.com/v1
API_KEY=your-api-key-here
EMBEDDING_MODEL=text-embedding-3-small
MODEL_CLIENT_TYPE=openai
```

2. Make sure the database is set up and running (required for memory storage).

## Usage

Run the CLI from the agent package:

```bash
pnpm start
```

Or with Bun directly:

```bash
bun run index.ts
```

## Features

- **Interactive Chat**: Type your messages and get responses from the AI agent
- **Long-term Memory**: Conversations are automatically saved and can be recalled
- **Context Loading**: Automatically loads recent conversations (last 60 minutes)
- **Memory Search**: The agent can search through past conversations using natural language
- **Tool Calling**: Supports automatic tool execution with multi-step reasoning
- **Multi-Provider Support**: Works with OpenAI, Anthropic, and Google AI (via Vercel AI SDK)

## Commands

- Type your message and press Enter to chat
- Type `exit` or `quit` to close the application

## Environment Variables

- `MODEL`: The LLM model ID (e.g., `gpt-4o`, `claude-3-5-sonnet-20241022`, `gemini-pro`)
- `BASE_URL`: The API base URL
- `API_KEY`: Your API key
- `EMBEDDING_MODEL`: The embedding model for memory search (e.g., `text-embedding-3-small`)
- `MODEL_CLIENT_TYPE`: The model provider type (default: `openai`, options: `openai`, `anthropic`, `google`)

