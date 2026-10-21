# LLM module

The LLM gateway loads model configuration from `config/llm.yaml` independently from the server's main YAML. Set `CS_IM_LLM_CONFIG` to use another file. Every enabled model must set `api_key_env` to an environment variable name; API key values must not be written to this file or committed.

Configure an endpoint base URL, the upstream model ID, a provider kind, and an API key environment variable for each public selector. The client sends only the selector as `model`; the server replaces it with the configured upstream model ID. Disabled entries can stay in the file without credentials. The sample entries are disabled until their model IDs and environment variables are supplied.

Supported provider kinds:

- `openai` and `openai-compatible` for Chat Completions compatible endpoints.
- `volcengine` / `volc`, `minimax`, `aliyun` / `qwen` use their configured OpenAI compatible endpoints.
- `opencode` uses Chat Completions. When the endpoint host is `opencode.ai`, the gateway sets its User-Agent and may set `X-OpenCode-Session` when `opencode_session_header` is enabled. The value comes from the caller's `X-OpenCode-Session` or `X-Session-ID` header, validated as an opaque safe identifier.
- `anthropic` uses the native Messages protocol with `X-Api-Key` and `Anthropic-Version` headers.

Routes are mounted below `/api/v1`:

- `POST /api/v1/chat/completions` for OpenAI-compatible requests.
- `POST /api/v1/messages` for Anthropic Messages requests.
- `GET /api/v1/models` returns enabled public selectors only.

All routes require a human account token with `gateway:admin` scope. Upstream URLs must use HTTPS. The gateway clamps caller token limits to the configured maximum and bounds request, response, and provider time. It sanitizes upstream error details before returning them.

For MiniMax responses the gateway removes `<think>`, `<thinking>`, `<analysis>`, and `<reasoning>` blocks, including streaming output. Text inside those blocks is dropped from the client response.

The existing gateway translation endpoint uses `default_translation_model` when set. If it is unset or the LLM request fails, it falls back to the existing Google translation path.
