# LLM Provider Configuration

ERPNext MCP Server supports multiple LLM providers for AI-powered query processing. Choose the provider that best fits your needs.

## Supported Providers

The integration is **truly generic** - it works with any OpenAI-compatible API!

### Native Providers

| Provider | Privacy | Cost | Speed | Setup Complexity |
|----------|---------|------|-------|------------------|
| **Ollama** | ✅ Local | Free | Fast | Easy |
| **OpenAI** | ☁️ Cloud | $$ | Fast | Easy |
| **Anthropic** | ☁️ Cloud | $$ | Medium | Easy |
| **Azure OpenAI** | ☁️ Enterprise | $$ | Fast | Medium |

### OpenAI-Compatible (via `base_url`)

| Provider | Type | Models | Cost |
|----------|------|--------|------|
| **[Together.ai](https://together.ai)** | Cloud | 150+ open-source | $ |
| **[Groq](https://groq.com)** | Cloud | Llama, Mixtral, Gemma | Free tier |
| **[OpenRouter](https://openrouter.ai)** | Cloud | 200+ models | $-$$$ |
| **[Replicate](https://replicate.com)** | Cloud | Research models | $ |
| **[LocalAI](https://localai.io)** | Self-hosted | Any GGUF | Free |
| **[LM Studio](https://lmstudio.ai)** | Desktop | Local models | Free |

*Plus any other service implementing OpenAI's API!*

## 1. Ollama (Default - Local & Private)

**Best for**: Privacy, offline use, no API costs

### Setup

```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull a model
ollama pull llama3.2:1b  # Fast, efficient (1.2GB)
ollama pull llama3.1     # More capable (4.9GB)
```

### Configuration

**config.yaml:**
```yaml
llm:
  provider: "ollama"
  timeout: "60s"
  ollama:
    url: "http://localhost:11434"
    model: "llama3.2:1b"
```

**Environment Variables:**
```bash
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=llama3.2:1b
```

### Pros & Cons

✅ **Pros:**
- Complete privacy - all data stays local
- No API costs
- Works offline
- Fast response times

❌ **Cons:**
- Requires local resources (CPU/RAM)
- Smaller models less capable than cloud LLMs

---

## 2. OpenAI

**Best for**: Best-in-class AI quality, wide model selection

### Setup

1. Get API key from https://platform.openai.com/api-keys
2. Add to environment

### Configuration

**config.yaml:**
```yaml
llm:
  provider: "openai"
  timeout: "60s"
  openai:
    api_key: "${OPENAI_API_KEY}"  # From environment
    model: "gpt-4o-mini"
    max_tokens: 500
    temperature: 0.7
```

**Environment Variables:**
```bash
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-proj-...your-key...
OPENAI_MODEL=gpt-4o-mini
```

### Available Models

| Model | Speed | Quality | Cost | Use Case |
|-------|-------|---------|------|----------|
| `gpt-4o-mini` | ⚡ Fast | Good | $ | Recommended for this use case |
| `gpt-4o` | Fast | Excellent | $$ | Best quality |
| `gpt-4-turbo` | Medium | Excellent | $$$ | Complex queries |
| `gpt-3.5-turbo` | ⚡ Fastest | Good | $ | Budget option |

### Pros & Cons

✅ **Pros:**
- Highest quality AI responses
- No local resources needed
- Fast and reliable

❌ **Cons:**
- Costs per API call
- Requires internet
- Data sent to OpenAI servers

### Cost Estimation

Approximate costs for ERPNext MCP queries:
- **gpt-4o-mini**: ~$0.0001-0.0002 per query
- **gpt-4o**: ~$0.001-0.002 per query

*Typical usage: 100 queries/day = $0.01-0.20/day with gpt-4o-mini*

---

## 3. Anthropic Claude

**Best for**: Detailed analysis, coding tasks, safety

### Setup

1. Get API key from https://console.anthropic.com/
2. Add to environment

### Configuration

**config.yaml:**
```yaml
llm:
  provider: "anthropic"
  timeout: "60s"
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-3-5-sonnet-20241022"
    max_tokens: 1024
    temperature: 0.7
```

**Environment Variables:**
```bash
LLM_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-...your-key...
ANTHROPIC_MODEL=claude-3-5-sonnet-20241022
```

### Available Models

| Model | Speed | Quality | Cost | Use Case |
|-------|-------|---------|------|----------|
| `claude-3-5-sonnet-20241022` | Fast | Excellent | $$ | Recommended |
| `claude-3-opus-20240229` | Medium | Best | $$$ | Complex analysis |
| `claude-3-haiku-20240307` | ⚡ Fastest | Good | $ | Simple queries |

### Pros & Cons

✅ **Pros:**
- Excellent reasoning and analysis
- Strong coding capabilities
- Safe and helpful responses

❌ **Cons:**
- Costs per API call
- Requires internet
- Data sent to Anthropic servers

---

## 4. Azure OpenAI

**Best for**: Enterprise deployments, compliance, regional data residency

### Setup

1. Create Azure OpenAI resource
2. Deploy a model (e.g., gpt-4)
3. Get endpoint and API key

### Configuration

**config.yaml:**
```yaml
llm:
  provider: "azure"
  timeout: "60s"
  azure:
    api_key: "${AZURE_OPENAI_API_KEY}"
    endpoint: "https://your-resource.openai.azure.com"
    deployment: "gpt-4"
    api_version: "2024-02-01"
    max_tokens: 500
    temperature: 0.7
```

**Environment Variables:**
```bash
LLM_PROVIDER=azure
AZURE_OPENAI_API_KEY=your-azure-key
AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
AZURE_OPENAI_DEPLOYMENT=gpt-4
```

### Pros & Cons

✅ **Pros:**
- Enterprise SLA and support
- Data residency control
- Compliance certifications (SOC 2, HIPAA, etc.)
- Same models as OpenAI

❌ **Cons:**
- More complex setup
- Requires Azure subscription
- Costs per API call

---

## Switching Providers

Switching is easy - just update configuration:

### Method 1: Environment Variable

```bash
# Switch to OpenAI
export LLM_PROVIDER=openai
export OPENAI_API_KEY=sk-...

# Restart server
./bin/frappe-mcp-server
```

### Method 2: Config File

Edit `config.yaml`:

```yaml
llm:
  provider: "openai"  # Changed from "ollama"
  # ... provider-specific config
```

Restart the server.

---

## OpenAI-Compatible Providers

The `openai` provider works with **ANY service that implements the OpenAI API**. This makes the integration truly generic!

### 5. Together.ai

**Best for**: Open-source models at scale, cost-effective inference

[Together.ai](https://www.together.ai/) offers 150+ open-source models with OpenAI-compatible API.

**Setup:**
1. Get API key from https://api.together.xyz/settings/api-keys
2. Configure:

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "https://api.together.xyz/v1"
    api_key: "${TOGETHER_API_KEY}"
    model: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
    max_tokens: 500
    temperature: 0.7
```

**Popular Models:**
- `meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo` - Flagship model
- `meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo` - Great balance
- `Qwen/Qwen2.5-72B-Instruct-Turbo` - Excellent for reasoning
- `deepseek-ai/DeepSeek-V3` - Code and reasoning
- `mistralai/Mixtral-8x22B-Instruct-v0.1` - Fast and capable

**Pros:**
- 150+ open-source models
- Very competitive pricing
- Fast inference with custom optimizations
- No vendor lock-in

### 6. Groq

**Best for**: Ultra-fast inference with LPU technology

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "https://api.groq.com/openai/v1"
    api_key: "${GROQ_API_KEY}"
    model: "llama-3.1-70b-versatile"
```

**Why Groq:**
- Extremely fast (500+ tokens/sec)
- Free tier available
- Llama 3, Mixtral, Gemma models

Get API key: https://console.groq.com/keys

### 7. OpenRouter

**Best for**: Access to 200+ models through one API

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "https://openrouter.ai/api/v1"
    api_key: "${OPENROUTER_API_KEY}"
    model: "anthropic/claude-3.5-sonnet"
```

**Why OpenRouter:**
- Single API for OpenAI, Anthropic, Google, Meta, etc.
- Pay-as-you-go pricing
- Automatic fallbacks
- Model routing

Get API key: https://openrouter.ai/keys

### 8. Replicate

**Best for**: Research models and custom deployments

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "https://openai-proxy.replicate.com/v1"
    api_key: "${REPLICATE_API_KEY}"
    model: "meta/meta-llama-3-70b-instruct"
```

Get API key: https://replicate.com/account/api-tokens

### 9. LocalAI (Self-Hosted)

**Best for**: Complete privacy and control

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "http://localhost:8080/v1"
    api_key: "not-needed"  # LocalAI doesn't require key
    model: "llama-3"
```

Setup: https://localai.io/

### 10. LM Studio (Desktop)

**Best for**: Local development with GUI

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "http://localhost:1234/v1"
    api_key: "lm-studio"
    model: "llama-3-8b"
```

Download: https://lmstudio.ai/

### 11. Ollama (OpenAI API Mode)

**Alternative**: Use Ollama through OpenAI-compatible endpoint

```yaml
llm:
  provider: "openai"
  openai:
    base_url: "http://localhost:11434/v1"
    api_key: "ollama"
    model: "llama3.2:1b"
```

*Note: Using native Ollama provider is recommended for better performance.*

---

## Recommendations

### For Development

Use **Ollama**:
- Free
- Fast
- No API limits
- Privacy

### For Production (Small Scale)

Use **OpenAI gpt-4o-mini**:
- Best quality/cost ratio
- Reliable
- Fast

### For Production (Enterprise)

Use **Azure OpenAI**:
- Enterprise support
- Compliance
- Data control

### For Maximum Privacy

Use **Ollama** with larger models:
- All data stays local
- No external API calls
- Full control

---

## Performance Tuning

### Timeout Configuration

Adjust based on provider and model:

```yaml
llm:
  timeout: "60s"  # Ollama: 30-60s, Cloud: 60-120s
```

### Temperature & Tokens

For query parsing (our use case):

```yaml
llm:
  openai:
    temperature: 0.7  # Balance creativity and consistency
    max_tokens: 500   # Enough for our JSON responses
```

Lower temperature (0.3-0.5) = More consistent
Higher temperature (0.7-1.0) = More creative

---

## Troubleshooting

### Provider Not Available

```
Error: LLM client not available - AI features disabled
```

**Solution**: Check provider configuration and credentials

### OpenAI Rate Limits

```
Error: OpenAI API returned status 429
```

**Solution**: Upgrade OpenAI plan or switch to Ollama

### Azure Endpoint Error

```
Error: Azure OpenAI API returned status 404
```

**Solution**: Verify endpoint URL and deployment name

### Ollama Connection Failed

```
Error: failed to call LLM: connection refused
```

**Solution**: Ensure Ollama is running:
```bash
ollama serve
```

---

## Cost Management

### Track Usage

Monitor API costs:
- OpenAI: https://platform.openai.com/usage
- Anthropic: https://console.anthropic.com/settings/cost
- Azure: Azure Portal > Cost Management

### Set Budgets

Configure alerts in your provider dashboard to avoid surprises.

### Optimize Costs

1. Use **cheaper models** for development
2. Use **Ollama** for testing
3. Use **cloud LLMs** only in production
4. **Cache** frequent queries (future feature)

---

Back to [Documentation Home](index.md)

