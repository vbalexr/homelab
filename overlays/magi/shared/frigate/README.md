# frigate (external backend)

Frigate NVR is not part of the cluster: it runs on the CCTV VLAN at `172.16.25.200`. `cameras.vbalex.com` goes through the
`shared-gateway` to its authenticated UI on port 8971 (plain HTTP, as Caddy did): a Service without a selector and an
`EndpointSlice` carry the backend address, and the route disables the request timeouts for live view and streams.
Cluster pods reach `172.16.25.200:8971` (checked from a pod); the Home VLAN does not need access to the CCTV VLAN.
Change the backend by editing the address in `service.yaml`.
