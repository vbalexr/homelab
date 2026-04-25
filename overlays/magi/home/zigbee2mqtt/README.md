# Zigbee2MQTT - Home Lab Overlay

Home namespace-specific Zigbee2MQTT config on magi cluster.

- Namespace: home
- Storage: 1Gi PVC (Rook Ceph)
- MQTT: Connects to mosquitto:1883 on base topic `zigbee2mqtt`

```bash
kubectl apply -k overlays/magi/home/zigbee2mqtt
```
