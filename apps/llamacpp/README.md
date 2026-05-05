# llama.cpp Server

OpenAI-compatible inference server for GGUF models, serving Qwen2.5-7B-Instruct (Q6_K) by default.

Base application definition (environment-agnostic Kubernetes manifests).

## Endpoint

- In-cluster: `http://llamacpp.<namespace>.svc.cluster.local:8080/v1`
- OpenAI-compatible: `/v1/chat/completions`, `/v1/models`, etc.

## Notes

- `--jinja` is required for Qwen2.5 tool calling (OpenAI `tools` / `tool_calls`).
- `-c 131072 --parallel 2` gives 64k context per slot (2 warm slots).
- KV cache quantized to `q8_0` to halve KV RAM with negligible quality impact for 7B.
- `--no-mmap` keeps weights fully resident (Talos has no swap).
- `--no-context-shift` returns an error instead of silently dropping oldest tokens when KV is full.
- First start downloads ~6.3 GB GGUF from HuggingFace into the model cache volume.

## Customization

Customize per-cluster using overlays in `overlays/<cluster>/.../llamacpp/`.
