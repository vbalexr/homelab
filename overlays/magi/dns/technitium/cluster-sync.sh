#!/bin/bash
# Keeps the Technitium cluster registered on stable addresses. The cluster identifies peers by IP address, and pod IPs
# change on every restart, so each node is registered with the ClusterIP of its technitium-node-<n> Service instead, which
# does not change while the Service exists. Each pod checks the IP it is registered with against that address and
# re-registers when they differ; a secondary whose primary is not connected is pointed at the primary's Service address.
api=http://127.0.0.1:5380
svc_suffix=dns.svc.cluster.local

log() {
  printf '{"level":"%s","service":"technitium-cluster-sync","pod":"%s","msg":"%s"}\n' "$1" "$POD_NAME" "$2"
}

node_ip() {
  getent ahostsv4 "technitium-node-$1.$svc_suffix" | awk 'NR==1{print $1}'
}

call() {
  curl -sf -m 20 "$@" | tr -d ' \t\r\n'
}

while true; do
  sleep 15

  token=$(call --data-urlencode user=admin --data-urlencode "pass=$DNS_SERVER_ADMIN_PASSWORD" "$api/api/user/login" \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
  [ -z "$token" ] && continue

  state=$(call "$api/api/admin/cluster/state?token=$token")
  case "$state" in *'"clusterInitialized":true'*) ;; *) continue ;; esac

  nodes=$(echo "$state" | grep -o '{"id":[^{}]*}')

  node_self_ip=$(node_ip "${POD_NAME##*-}")
  [ -z "$node_self_ip" ] && continue

  self_ips=$(echo "$nodes" | grep '"state":"Self"' | sed -n 's/.*"ipAddresses":\["\([^]]*\)\].*/\1/p' | tr -d '"')
  if [ "$self_ips" != "$node_self_ip" ]; then
    if call --data-urlencode "token=$token" --data-urlencode "ipAddresses=$node_self_ip" "$api/api/admin/cluster/updateIpAddress" >/dev/null; then
      log info "registered service IP $node_self_ip (was ${self_ips:-none})"
    else
      log error "failed to register service IP $node_self_ip"
    fi
  fi

  # only a secondary points at the primary: the primary is the node whose type is Primary and which is not Self
  primary=$(echo "$nodes" | grep '"type":"Primary"' | grep -v '"state":"Self"')
  [ -z "$primary" ] && continue
  echo "$primary" | grep -q '"state":"Connected"' && continue

  primary_url=$(echo "$primary" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
  primary_pod=$(echo "$primary" | sed -n 's/.*"name":"\([^."]*\)\..*/\1/p')
  primary_ip=$(node_ip "${primary_pod##*-}")
  [ -z "$primary_ip" ] && continue

  if call --data-urlencode "token=$token" --data-urlencode "primaryNodeUrl=$primary_url" \
    --data-urlencode "primaryNodeIpAddresses=$primary_ip" "$api/api/admin/cluster/secondary/updatePrimary" >/dev/null; then
    log info "pointed at primary $primary_pod on $primary_ip"
  else
    log error "failed to point at primary $primary_pod on $primary_ip"
  fi
done
