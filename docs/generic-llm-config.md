# Generic LLM Configuration

## Overview

The ERPNext MCP Server now uses a **truly generic LLM configuration** that works with ANY provider. No more provider-specific configs!

## Key Changes

### Before (Provider-Specific)

```yaml
llm:
  provider: "ollama"
  ollama:
    url: "..."
    model: "..."
  openai:
    api_key: "..."
    model: "..."
    base_url: "..."
  anthropic:
    api_key: "..."
    model: "..."
```

**Problems**:
- ❌ Redundant configuration for each provider
- ❌ Not truly generic
- ❌ Hard to add new providers
- ❌ Confusing structure

### After (Generic)

```yaml
llm:
  provider_type: "openai-compatible"  # One of: openai-compatible, anthropic, azure
  base_url: "..."         # API endpoint
  api_key: "..."          # API key
  model: "..."            # Model name
  timeout: "60s"
  max_tokens: 500
  temperature: 0.7
```

**Benefits**:
- ✅ **ONE configuration** for ALL providers
- ✅ **Truly generic** - works with any OpenAI-compatible API
- ✅ **Simple** - just 3-4 fields to change
- ✅ **Flexible** - easy to switch providers

## Configuration

### Generic Fields (All Providers)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider_type` | string | No | `openai-compatible` (default), `anthropic`, or `azure` |
| `base_url` | string | Yes | API endpoint URL |
| `api_key` | string | Optional | API key (if required) |
| `model` | string | Yes | Model name/ID |
| `timeout` | duration | No | Request timeout (default: 60s) |
| `max_tokens` | int | No | Max tokens (default: 500) |
| `temperature` | float | No | Temperature 0.0-2.0 (default: 0.7) |

### Azure-Specific Fields

Only needed if `provider_type: "azure"`:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `azure_deployment` | string | Yes | Azure deployment name |
| `azure_api_version` | string | No | API version (default: 2024-02-01) |

## Examples

### Ollama (Local)

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "http://localhost:11434/v1"
  api_key: ""
  model: "llama3.2:1b"
```

### OpenAI

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "https://api.openai.com/v1"
  api_key: "${LLM_API_KEY}"
  model: "gpt-4o-mini"
```

### Together.ai (150+ Models)

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "https://api.together.xyz/v1"
  api_key: "${LLM_API_KEY}"
  model: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
```

### Groq (Ultra-Fast)

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "https://api.groq.com/openai/v1"
  api_key: "${LLM_API_KEY}"
  model: "llama-3.1-70b-versatile"
```

### OpenRouter (200+ Models)

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "https://openrouter.ai/api/v1"
  api_key: "${LLM_API_KEY}"
  model: "anthropic/claude-3.5-sonnet"
```

### Replicate

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "https://openai-proxy.replicate.com/v1"
  api_key: "${LLM_API_KEY}"
  model: "meta/meta-llama-3-70b-instruct"
```

### LocalAI (Self-Hosted)

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "http://localhost:8080/v1"
  api_key: ""
  model: "llama-3"
```

### LM Studio (Desktop)

```yaml
llm:
  provider_type: "openai-compatible"
  base_url: "http://localhost:1234/v1"
  api_key: "lm-studio"
  model: "llama-3-8b"
```

### Anthropic Claude (Different API)

```yaml
llm:
  provider_type: "anthropic"
  base_url: "https://api.anthropic.com/v1"
  api_key: "${LLM_API_KEY}"
  model: "claude-3-5-sonnet-20241022"
```

### Azure OpenAI

```yaml
llm:
  provider_type: "azure"
  base_url: "https://your-resource.openai.azure.com"
  api_key: "${LLM_API_KEY}"
  model: "gpt-4"
  azure_deployment: "gpt-4"
  azure_api_version: "2024-02-01"
```

## Environment Variables

Even simpler with environment variables:

```bash
# Just set these 4 variables:
LLM_PROVIDER_TYPE=openai-compatible
LLM_BASE_URL=https://api.together.xyz/v1
LLM_API_KEY=your-key-here
LLM_MODEL=meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo
```

## Switching Providers

### In 3 Steps:

1. **Change `base_url`** to the provider's endpoint
2. **Set `api_key`** (if required)
3. **Update `model`** to the model name
4. Done! 🎉

### Example: Ollama → Together.ai

```yaml
# Before (Ollama)
base_url: "http://localhost:11434/v1"
api_key: ""
model: "llama3.2:1b"

# After (Together.ai)
base_url: "https://api.together.xyz/v1"
api_key: "your-together-key"
model: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
```

That's it! No other changes needed.

## Provider Detection

The server automatically detects which provider you're using based on the `base_url`:

- Contains `together` → logs as "together.ai"
- Contains `groq` → logs as "groq"
- Contains `openrouter` → logs as "openrouter"
- Contains `replicate` → logs as "replicate"
- Contains `openai.com` → logs as "openai"
- Contains `ollama` or `:11434` → logs as "ollama"
- Contains `localhost` → logs as "local"
- Otherwise → logs as "openai-compatible"

This is just for better logging - **all OpenAI-compatible providers work the same way!**

## Implementation Details

### Architecture

```
┌─────────────────────────────────────────────┐
│         LLMConfig (Generic)                 │
│  - provider_type                            │
│  - base_url, api_key, model                 │
│  - timeout, max_tokens, temperature         │
└────────────────┬────────────────────────────┘
                 │
                 ↓
┌────────────────┴────────────────────────────┐
│     llm.NewClient(cfg)                      │
│     (Factory based on provider_type)        │
└────────────────┬────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
        ↓                 ↓
┌───────────────┐  ┌──────────────┐
│ OpenAI-       │  │ Anthropic    │
│ Compatible    │  │ Client       │
│ Client        │  │              │
└───────────────┘  └──────────────┘
        │
        ├─→ OpenAI
        ├─→ Together.ai
        ├─→ Groq
        ├─→ OpenRouter
        ├─→ Ollama
        ├─→ LocalAI
        └─→ Any OpenAI-compatible API
```

### Code Structure

**Before**: 5 provider-specific files
- `ollama.go`
- `openai.go`
- `anthropic.go`
- `azure.go`
- `client.go`

**After**: 4 files (more generic)
- `openai.go` → `OpenAICompatibleClient` (handles ALL OpenAI-compatible APIs)
- `anthropic.go` → `AnthropicClient` (Anthropic-specific API)
- `azure.go` → `AzureClient` (Azure-specific URL format)
- `client.go` → Factory function

### Benefits

1. **Simplicity**: One config structure for everything
2. **Flexibility**: Works with ANY OpenAI-compatible API
3. **Maintainability**: Less code, easier to maintain
4. **Extensibility**: Easy to add new OpenAI-compatible providers (no code changes!)
5. **User-Friendly**: Simple 3-field configuration

## Migration from Old Config

**Old config format is NOT supported**. Update your `config.yaml`:

```yaml
# OLD (not supported)
llm:
  provider: "ollama"
  ollama:
    url: "http://localhost:11434"
    model: "llama3.2:1b"

# NEW (required)
llm:
  provider_type: "openai-compatible"
  base_url: "http://localhost:11434/v1"
  api_key: ""
  model: "llama3.2:1b"
  timeout: "60s"
  max_tokens: 500
  temperature: 0.7
```

**Note the `/v1` suffix** for Ollama's OpenAI-compatible endpoint!

## Testing

```bash
# Test with Ollama
LLM_BASE_URL=http://localhost:11434/v1 \
LLM_MODEL=llama3.2:1b \
./bin/frappe-mcp-server

# Test with Together.ai
LLM_BASE_URL=https://api.together.xyz/v1 \
LLM_API_KEY=your-key \
LLM_MODEL=meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo \
./bin/frappe-mcp-server

# Test with OpenAI
LLM_BASE_URL=https://api.openai.com/v1 \
LLM_API_KEY=sk-... \
LLM_MODEL=gpt-4o-mini \
./bin/frappe-mcp-server
```

## Summary

**Before**: Provider-specific configs with nested structures
**After**: Flat, generic configuration that works with ANY provider

Just set:
1. `base_url` - Where is the API?
2. `api_key` - What's the key? (if needed)
3. `model` - Which model?

That's it! 🚀

