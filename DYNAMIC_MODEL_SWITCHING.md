# Dynamic Model Switching - Design Document

## Problem

**Current State:**
- Model configuration is static (read from config.yaml on startup)
- Changing models requires server restart
- No way to switch models when rate limits hit
- Not suitable for production

**Real-World Scenarios:**
1. Groq hits rate limit → need to switch to Ollama immediately
2. Testing different models for quality → need to A/B test
3. Cost optimization → use cheaper model for simple queries
4. User preference → allow users to choose their model

---

## Solution: Dynamic Model Switching API

### New Endpoints

#### 1. Switch Model (Runtime)
```bash
POST /api/v1/admin/switch-model
{
  "provider": "ollama",           # ollama | groq | openai
  "model": "llama3.1:latest",
  "base_url": "http://ollama:11434/v1",
  "api_key": "",                  # optional
  "temperature": 0.3,             # optional
  "max_tokens": 1000,             # optional
  "persist": false                # if true, update config.yaml
}

Response:
{
  "status": "success",
  "message": "Switched to ollama/llama3.1:latest",
  "previous_model": "groq/llama-3.3-70b-versatile",
  "current_model": "ollama/llama3.1:latest"
}
```

#### 2. Get Current Model
```bash
GET /api/v1/admin/model-status

Response:
{
  "provider": "groq",
  "model": "llama-3.3-70b-versatile",
  "base_url": "https://api.groq.com/openai/v1",
  "status": "rate_limited",       # active | rate_limited | error
  "tokens_used": 99665,
  "tokens_limit": 100000,
  "fallback_available": true,
  "fallback_provider": "ollama"
}
```

#### 3. List Available Models
```bash
GET /api/v1/admin/models

Response:
{
  "models": [
    {
      "name": "groq-llama-70b",
      "provider": "groq",
      "model": "llama-3.3-70b-versatile",
      "base_url": "https://api.groq.com/openai/v1",
      "status": "rate_limited",
      "description": "Ultra-fast 70B model",
      "cost": "free-tier-limited"
    },
    {
      "name": "ollama-llama-8b",
      "provider": "ollama",
      "model": "llama3.1:latest",
      "base_url": "http://ollama:11434/v1",
      "status": "available",
      "description": "Local 8B model",
      "cost": "free-unlimited"
    },
    {
      "name": "ollama-mixtral",
      "provider": "ollama",
      "model": "mixtral:8x7b",
      "base_url": "http://ollama:11434/v1",
      "status": "unavailable",
      "description": "Requires 26GB RAM",
      "cost": "free-unlimited"
    }
  ]
}
```

#### 4. Auto-Fallback Configuration
```bash
POST /api/v1/admin/configure-fallback
{
  "enabled": true,
  "primary": {
    "provider": "groq",
    "model": "llama-3.3-70b-versatile"
  },
  "fallback": {
    "provider": "ollama",
    "model": "llama3.1:latest"
  },
  "fallback_triggers": ["rate_limit", "timeout", "error"]
}

Response:
{
  "status": "success",
  "message": "Auto-fallback configured",
  "will_fallback_on": ["rate_limit", "timeout", "error"]
}
```

---

## Implementation Architecture

### 1. LLM Manager (New Component)

```go
type LLMManager struct {
    currentClient  LLMClient
    fallbackClient LLMClient
    config         *LLMConfig
    autoFallback   bool
    mutex          sync.RWMutex
}

func (m *LLMManager) SwitchModel(config ModelConfig) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    // Create new client
    newClient := createLLMClient(config)
    
    // Test connection
    if err := newClient.Test(); err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }
    
    // Swap
    m.currentClient = newClient
    return nil
}

func (m *LLMManager) Generate(ctx context.Context, prompt string) (string, error) {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    // Try primary
    result, err := m.currentClient.Generate(ctx, prompt)
    
    // Auto-fallback on rate limit
    if m.autoFallback && isRateLimitError(err) {
        log.Warn("Rate limit hit, falling back to secondary")
        result, err = m.fallbackClient.Generate(ctx, prompt)
    }
    
    return result, err
}
```

### 2. Model Registry

```go
type ModelRegistry struct {
    models map[string]ModelConfig
}

var DefaultModels = map[string]ModelConfig{
    "groq-llama-70b": {
        Provider: "groq",
        Model: "llama-3.3-70b-versatile",
        BaseURL: "https://api.groq.com/openai/v1",
        Description: "Ultra-fast 70B model",
    },
    "ollama-llama-8b": {
        Provider: "ollama",
        Model: "llama3.1:latest",
        BaseURL: "http://ollama:11434/v1",
        Description: "Local 8B model",
    },
    "ollama-mixtral": {
        Provider: "ollama",
        Model: "mixtral:8x7b",
        BaseURL: "http://ollama:11434/v1",
        Description: "Local 47B MoE model",
    },
}
```

---

## Usage Examples

### Example 1: Manual Switch When Rate Limited

```bash
# User gets rate limited
curl -X POST http://localhost:8080/api/v1/chat \
  -d '{"message": "list users"}'
# Response: "⚠️ AI service temporarily unavailable..."

# Admin switches to Ollama
curl -X POST http://localhost:8080/api/v1/admin/switch-model \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "ollama",
    "model": "llama3.1:latest"
  }'
# Response: {"status": "success", "current_model": "ollama/llama3.1:latest"}

# User retries - now works!
curl -X POST http://localhost:8080/api/v1/chat \
  -d '{"message": "list users"}'
# Response: Users list ✅
```

### Example 2: Auto-Fallback (Recommended)

```bash
# Configure auto-fallback once
curl -X POST http://localhost:8080/api/v1/admin/configure-fallback \
  -d '{
    "enabled": true,
    "fallback_triggers": ["rate_limit"]
  }'

# Now when Groq hits rate limit, automatically uses Ollama
# User never sees rate limit error!
```

### Example 3: Query-Level Model Selection

```bash
# Use specific model for this query
curl -X POST http://localhost:8080/api/v1/chat \
  -d '{
    "message": "list users",
    "model_preference": "ollama-llama-8b"  # NEW
  }'
```

---

## Configuration File Updates

### New config.yaml structure:

```yaml
llm:
  # Primary model
  primary:
    provider: "groq"
    model: "llama-3.3-70b-versatile"
    base_url: "https://api.groq.com/openai/v1"
    api_key: "gsk_..."
    temperature: 0.3
    max_tokens: 1000
  
  # Fallback model (optional)
  fallback:
    enabled: true
    provider: "ollama"
    model: "llama3.1:latest"
    base_url: "http://ollama:11434/v1"
    api_key: ""
    temperature: 0.3
    max_tokens: 1000
  
  # Auto-fallback configuration
  auto_fallback:
    enabled: true
    triggers:
      - rate_limit      # Fallback on 429
      - timeout         # Fallback on timeout
      - error           # Fallback on 500+
    revert_after: "5m"  # Try primary again after 5 minutes
```

---

## Security Considerations

### 1. Admin Authentication
```go
// Only authenticated admins can switch models
func (s *Server) handleSwitchModel(w http.ResponseWriter, r *http.Request) {
    // Check admin token
    token := r.Header.Get("X-Admin-Token")
    if !s.isValidAdminToken(token) {
        http.Error(w, "Unauthorized", 401)
        return
    }
    // ... switch logic
}
```

### 2. Rate Limiting
- Limit model switches to prevent abuse
- Log all model switches for audit

### 3. Validation
- Validate model names against registry
- Test connection before switching
- Don't allow arbitrary base_url (security risk)

---

## Implementation Plan

### Phase 1: Core Infrastructure ✅ (Current PR)
- [x] Rate limit detection
- [x] User-friendly messaging
- [x] Test suite

### Phase 2: Dynamic Switching (Next PR)
- [ ] LLMManager component
- [ ] Switch model endpoint
- [ ] Model status endpoint
- [ ] List models endpoint
- [ ] Update config structure

### Phase 3: Auto-Fallback (Next PR)
- [ ] Fallback configuration
- [ ] Auto-fallback logic
- [ ] Revert to primary after cooldown
- [ ] Fallback metrics

### Phase 4: Query-Level Selection (Future)
- [ ] Per-query model preference
- [ ] Smart routing (simple queries → small model)
- [ ] Cost optimization

---

## Benefits

### For Production ✅
- **Zero downtime** model switching
- **Automatic failover** on rate limits
- **Cost optimization** - use cheaper models when possible
- **Flexibility** - A/B test models

### For Development ✅
- **Rapid iteration** - test different models
- **No restarts** - faster development
- **Easy debugging** - switch to verbose model

### For Users ✅
- **Better experience** - no "try again later"
- **Faster responses** - auto-switch to fastest available
- **Reliability** - always have a working model

---

## Metrics to Track

```go
type ModelMetrics struct {
    Provider         string
    Model            string
    RequestCount     int64
    SuccessCount     int64
    ErrorCount       int64
    RateLimitCount   int64
    AvgResponseTime  time.Duration
    TokensUsed       int64
    FallbackCount    int64
}
```

Dashboard:
- Requests per model
- Success rate per model
- Rate limit frequency
- Fallback triggers
- Cost per model

---

## Status

**Current:** Phase 1 complete (rate limit detection)  
**Next:** Implement Phase 2 (dynamic switching)  
**Timeline:** ~2-3 days development + testing

**Would you like me to implement this now?**

---

**Created:** November 15, 2025  
**Purpose:** Enable production-ready dynamic model switching  
**Priority:** High (required for production deployment)

