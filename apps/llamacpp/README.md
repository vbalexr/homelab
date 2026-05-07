# llama.cpp Server

OpenAI-compatible inference server for GGUF models, serving `unsloth/Qwen3.6-27B-GGUF` (Q4_K_M) with the matching vision projector (`mmproj-F16.gguf`) by default.

Base application definition (environment-agnostic Kubernetes manifests).

## Endpoint

- In-cluster: `http://llamacpp.<namespace>.svc.cluster.local:8080/v1`
- OpenAI-compatible: `/v1/chat/completions`, `/v1/models`, etc.

## Notes

- `--jinja` is required for Qwen tool calling (OpenAI `tools` / `tool_calls`).
- Context size follows the GGUF unless you set `-c` / `--ctx-size` in args; keep total memory in mind for KV plus vision (image) preprocessing.
- Vision: `--mmproj-url` loads the Unsloth projector; `--no-mmproj-offload` matches CPU-only pods (no GPU in this manifest). Use OpenAI-style `/v1/chat/completions` with image parts (e.g. base64) per [llama.cpp multimodal docs](https://github.com/ggml-org/llama.cpp/blob/master/docs/multimodal.md).
- KV cache quantized to `q8_0` to reduce KV RAM.
- `--no-mmap` keeps weights fully resident (Talos has no swap).
- `--no-context-shift` returns an error instead of silently dropping oldest tokens when KV is full.
- First start downloads the main GGUF plus the mmproj file into the model cache volume (~16 GiB + ~0.9 GiB for Q4_K_M + F16 projector).

## Customization

Customize per-cluster using overlays in `overlays/<cluster>/.../llamacpp/`.
