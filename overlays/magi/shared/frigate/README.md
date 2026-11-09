# frigate (external backend)

Frigate NVR runs outside the cluster. The `shared-gateway` route reaches its authenticated UI through a Service without a selector and
an `EndpointSlice` that carry the backend address, with the request timeouts disabled for live view and streams. Change the backend by
editing the address in `service.yaml`.
