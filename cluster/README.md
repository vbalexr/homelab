# cluster

This guide covers only SOPS setup after the cluster is already up and running.

## Prerequisite

Set your cluster name once for the commands below:


## Bootstrap FluxCD

```bash
flux bootstrap github \
  --owner=<owner> \
  --repository=homelab \
  --branch=main \
  --path=cluster/<cluster-name> \
  --components-extra=image-reflector-controller,image-automation-controller \
  --read-write-key
```

```bash
export CLUSTER_NAME=<cluster-name>
```

## Install SOPS

Install SOPS and confirm it is available:

```bash
sudo pacman -S sops age
which sops
which age-keygen
```

## Generate Age key

Create an Age keypair for the cluster and extract the public key:

```bash
age-keygen -o "cluster/${CLUSTER_NAME}/.sops.age"
chmod 600 "cluster/${CLUSTER_NAME}/.sops.age"
age_public_key="$(grep '^# public key:' "cluster/${CLUSTER_NAME}/.sops.age" | awk '{print $4}')"
echo "$age_public_key"
```

## Configure cluster decryption secret (flux-system)

Create or update the Flux decryption secret from the Age private key:

```bash
kubectl create secret generic sops-age \
  --namespace flux-system \
  --from-file=age.agekey="cluster/${CLUSTER_NAME}/.sops.age" \
  --dry-run=client -o yaml | kubectl apply -f -
```

## Verify setup

Confirm the secret exists and SOPS can decrypt with this key:

```bash
kubectl get secret sops-age -n flux-system
SOPS_AGE_KEY_FILE="cluster/${CLUSTER_NAME}/.sops.age" sops --decrypt <encrypted-file>.enc.yaml >/dev/null
```
