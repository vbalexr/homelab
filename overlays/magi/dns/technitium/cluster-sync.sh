#!/bin/bash
# Keeps the Technitium cluster pointed at the current pod IPs. The cluster identifies peers by IP address, and pod IPs
# change on every restart, so each pod checks the IP it is registered with against its real one and re-registers when they
# differ. A secondary whose primary is unreachable also takes the primary's current pod IP from the headless service.
# The primary does the reverse for every secondary: the primary cannot reach a secondary at a stale address, and the
# secondary cannot always tell the primary its new one, so the primary sets the secondary's current pod IP. A secondary
# can look connected through an old address while zone transfers to it are refused, so the state is not checked.
api=http://127.0.0.1:5380
peer_suffix=technitium-headless.dns.svc.cluster.local

log() {
  printf '{"level":"%s","service":"technitium-cluster-sync","pod":"%s","msg":"%s"}\n' "$1" "$POD_NAME" "$2"
}

call() {
  curl -sf -m 20 "$@" | tr -d ' \t\r\n'
}

# the API answers errors with HTTP 200 and "status":"error", so success is read from the body
call_ok() {
  case "$(call "$@")" in *'"status":"ok"'*) return 0 ;; esac
  return 1
}

while true; do
  sleep 15

  token=$(call --data-urlencode user=admin --data-urlencode "pass=$DNS_SERVER_ADMIN_PASSWORD" "$api/api/user/login" \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
  [ -z "$token" ] && continue

  state=$(call "$api/api/admin/cluster/state?token=$token")
  case "$state" in *'"clusterInitialized":true'*) ;; *) continue ;; esac

  nodes=$(echo "$state" | grep -o '{"id":[^{}]*}')

  self_ips=$(echo "$nodes" | grep '"state":"Self"' | sed -n 's/.*"ipAddresses":\["\([^]]*\)\].*/\1/p' | tr -d '"')
  if [ "$self_ips" != "$POD_IP" ]; then
    if call_ok --data-urlencode "token=$token" --data-urlencode "ipAddresses=$POD_IP" "$api/api/admin/cluster/updateIpAddress" >/dev/null; then
      log info "registered pod IP $POD_IP (was ${self_ips:-none})"
    else
      log error "failed to register pod IP $POD_IP"
    fi
  fi

  # on the primary: a secondary registered with an old pod IP is refused zone transfers, so set the current one
  case "$(echo "$nodes" | grep '"state":"Self"')" in
    *'"type":"Primary"'*)
      echo "$nodes" | grep '"type":"Secondary"' | while read -r secondary; do
        secondary_id=$(echo "$secondary" | sed -n 's/.*"id":\([0-9-]*\).*/\1/p')
        secondary_url=$(echo "$secondary" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
        secondary_pod=$(echo "$secondary" | sed -n 's/.*"name":"\([^."]*\)\..*/\1/p')
        secondary_ips=$(echo "$secondary" | sed -n 's/.*"ipAddresses":\["\([^]]*\)\].*/\1/p' | tr -d '"')
        secondary_ip=$(getent ahostsv4 "$secondary_pod.$peer_suffix" | awk 'NR==1{print $1}')
        [ -z "$secondary_id" ] || [ -z "$secondary_url" ] || [ -z "$secondary_ip" ] && continue
        [ "$secondary_ips" = "$secondary_ip" ] && continue

        # the update call also takes the secondary's own (self-signed) certificate, read from its API port in base64url form
        secondary_cert=$(curl -ks -m 8 -o /dev/null --write-out '%{certs}' "https://$secondary_ip:53443/" \
          | sed -n '/BEGIN CERTIFICATE/,/END CERTIFICATE/p' | grep -v -- '-----' | tr -d '\n' | tr '+/' '-_' | tr -d '=')
        [ -z "$secondary_cert" ] && continue

        if call_ok --data-urlencode "token=$token" --data-urlencode "secondaryNodeId=$secondary_id" \
          --data-urlencode "secondaryNodeUrl=$secondary_url" --data-urlencode "secondaryNodeIpAddresses=$secondary_ip" \
          --data-urlencode "secondaryNodeCertificate=$secondary_cert" "$api/api/admin/cluster/primary/updateSecondary" >/dev/null; then
          log info "set secondary $secondary_pod to pod IP $secondary_ip (was ${secondary_ips:-none})"
        else
          log error "failed to set secondary $secondary_pod to pod IP $secondary_ip"
        fi
      done
      ;;
  esac

  # only a secondary points at the primary: the primary is the node whose type is Primary and which is not Self
  primary=$(echo "$nodes" | grep '"type":"Primary"' | grep -v '"state":"Self"')
  [ -z "$primary" ] && continue
  echo "$primary" | grep -q '"state":"Connected"' && continue

  primary_url=$(echo "$primary" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
  primary_pod=$(echo "$primary" | sed -n 's/.*"name":"\([^."]*\)\..*/\1/p')
  primary_ip=$(getent ahostsv4 "$primary_pod.$peer_suffix" | awk 'NR==1{print $1}')
  [ -z "$primary_ip" ] && continue

  if call_ok --data-urlencode "token=$token" --data-urlencode "primaryNodeUrl=$primary_url" \
    --data-urlencode "primaryNodeIpAddresses=$primary_ip" "$api/api/admin/cluster/secondary/updatePrimary" >/dev/null; then
    log info "pointed at primary $primary_pod on $primary_ip"
  else
    log error "failed to point at primary $primary_pod on $primary_ip"
  fi
done
