# Manually rotating the Lokomotive control plane certificates

- `ca.key`
  - Kubernetes CA key
  - Generated by Bootkube: https://github.com/kinvolk/lokomotive/blob/3ba5cf3c94b460901d8dcbb4eb047ee938eb5bd8/assets/terraform-modules/bootkube/tls-k8s.tf#L27-L30
  - Usages:
    - On controller node at `/opt/bootkube/assets/tls/ca.key`
    - In `kube-controller-manager` pods at `/etc/kubernetes/secrets/ca.key` via secret `kube-controller-manager` in `kube-system` (secret created by `kubernetes` chart)
    - In `bootstrap-controller-manager` pod at `/etc/kubernetes/secrets/ca.key` via secret `secrets`
    - Used to sign:
      - `tls_self_signed_cert.kube-ca`
      - `tls_locally_signed_cert.apiserver`
      - `tls_locally_signed_cert.admin`
      - `tls_locally_signed_cert.kubelet`
      - `tls_locally_signed_cert.admission-webhook-server`
      - Service accounts
- `ca.crt` - Kubernetes CA certificate
  - Usages:
    - On all nodes at `/etc/kubernetes/ca.crt`
      - Extracted from /etc/kubernetes/kubeconfig: https://github.com/kinvolk/lokomotive/blob/6b1e7cbdb84a3d91aa84cd494cc420951c4838fb/assets/terraform-modules/packet/flatcar-linux/kubernetes/cl/controller.yaml.tmpl#L113
    - On controller at `/opt/bootkube/assets/tls/ca.crt`
    - In `kube-controller-manager` pods at `/etc/kubernetes/secrets/ca.crt` via secret `kube-controller-manager` in `kube-system` (secret created by `kubernetes` chart)
    - In `kube-apiserver` pods at `/etc/kubernetes/secrets/ca.crt` via secret `kube-apiserver` in `kube-system` (secret created by `kube-apiserver` chart)
    - In `kubelet` daemonset at `/etc/kubernetes/ca.crt` mounted from host
    - In `bootstrap-controller-manager` pod at `/etc/kubernetes/secrets/ca.crt` via secret `secrets`
    - In `bootstrap-apiserver` pod at `/etc/kubernetes/secrets/ca.crt` via secret `secrets`

- `etcd-ca.key` - etcd CA key
  - Generated by Bootkube: https://github.com/kinvolk/lokomotive/blob/3ba5cf3c94b460901d8dcbb4eb047ee938eb5bd8/assets/terraform-modules/bootkube/tls-etcd.tf#L8-L11
  Usages:
    - `tls_self_signed_cert.etcd-ca`
    - `tls_locally_signed_cert.client`
    - `tls_locally_signed_cert.server`
    - `tls_locally_signed_cert.peer`
- `etcd-ca.crt` - etcd CA certificate
  - Generated by Bootkube: https://github.com/kinvolk/lokomotive/blob/3ba5cf3c94b460901d8dcbb4eb047ee938eb5bd8/assets/terraform-modules/bootkube/tls-etcd.tf#L2-L5
  - Usages:
    - `tls_locally_signed_cert.client`
    - `tls_locally_signed_cert.server`
    - `tls_locally_signed_cert.peer`

- `etcd-client.key` - etcd client key
  - Usages:
    - In `kube-apiserver` pods at `/etc/kubernetes/secrets/etcd-client.key` via secret `kube-apiserver` in `kube-system` (secret created by `kube-apiserver` chart)
- `etcd-client.crt` - etcd client certificate
  - Usages:
    - In `kube-apiserver` pods at `/etc/kubernetes/secrets/etcd-client.crt` via secret `kube-apiserver` in `kube-system` (secret created by `kube-apiserver` chart)
- `etcd-client-ca.crt` - etcd client CA certificate
  - Usages:
    - In `kube-apiserver` pods at `/etc/kubernetes/secrets/etcd-client-ca.crt` via secret `kube-apiserver` in `kube-system` (secret created by `kube-apiserver` chart)

- `server-ca.crt`
- `server.crt`
- `server.key`

- `peer-ca.crt`
- `peer.crt`
- `peer.key`

- `apiserver.key`
- `apiserver.crt`

- `admin.key`
- `admin.crt`

- `service-account.key`
- `service-account.pub`

- `kubelet.key`
- `kubelet.crt`

- `aggregation-ca.key`
- `aggregation-ca.crt`
- `aggregation-client.key`
- `aggregation-client.crt`

Not written to disk:

- `tls_private_key.admission-webhook-server`
- `tls_locally_signed_cert.admission-webhook-server`

Check expiration date of a certificate:

```
openssl x509 -enddate -noout -in ca.crt
```

## Generate new CA keys and certificates

### Back up old certificates and keys

```
mkdir cert-rotation && cd $_
mkdir backup
kubectl -n kube-system get secrets kube-controller-manager -ojson | jq -r '.data["ca.key"]' | base64 -d > backup/ca.key
kubectl -n kube-system get secrets kube-controller-manager -ojson | jq -r '.data["ca.crt"]' | base64 -d > backup/ca.crt
kubectl -n kube-system get secrets kube-apiserver -ojson | jq -r '.data["apiserver.crt"]' | base64 -d > backup/apiserver.crt
cp ../assets/cluster-assets/auth/kubeconfig backup/kubeconfig
```

### Generate self-signed certificate and key for CA

```
openssl req -newkey rsa:2048 -x509 -nodes -days 3650 \
    -subj '/O=bootkube/CN=kubernetes-ca' \
    -addext 'keyUsage=critical, digitalSignature, keyEncipherment, keyCertSign' \
    -keyout ca.key -out ca.crt
```

### Create a combined certificate

In order to complete the certificate rotation with minimal impact, control plane elements need to
be configured to temporarily trust both the old and the new CA. This is done by configuring them
with a file containing the TLS certificates of both CAs.

```
cat ca.crt backup/ca.crt > combined.crt
```

### Generate a certificate for admin kubeconfig

```
cat <<EOF >admin.ext
keyUsage=critical, digitalSignature, keyEncipherment
extendedKeyUsage=clientAuth
basicConstraints=critical, CA:FALSE
authorityKeyIdentifier=keyid
EOF

openssl req -newkey rsa:2048 -nodes \
    -subj '/O=system:masters/CN=kubernetes-admin' \
    -keyout admin.key \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial \
    -days 3650 -extfile admin.ext -out admin.crt
```

### Generate a certificate for kube-apiserver

```
# Get subject alt names from existing certificate
altnames=$(openssl x509 -in backup/apiserver.crt -noout -text \
    | grep 'DNS:' | sed 's/^\s*//' | sed 's/IP\sAddress/IP/')

cat <<EOF >apiserver.ext
keyUsage=critical, digitalSignature, keyEncipherment
extendedKeyUsage=serverAuth, clientAuth
basicConstraints=critical, CA:FALSE
authorityKeyIdentifier=keyid
subjectAltName=$altnames
EOF

openssl req -newkey rsa:2048 -nodes \
    -subj '/O=system:masters/CN=kube-apiserver' \
    -keyout apiserver.key \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial \
    -days 3650 -extfile apiserver.ext -out apiserver.crt
```

### Update kube-controller-manager secret

```
kubectl -n kube-system get secrets kube-controller-manager -ojson \
    | jq -r --arg key "$(cat ca.key | base64 -w0)" '.data["ca.key"]=$key' | kubectl apply -f -
kubectl -n kube-system get secrets kube-controller-manager -ojson \
    | jq -r --arg cert "$(cat ca.crt | base64 -w0)" '.data["ca.crt"]=$cert' | kubectl apply -f -
```

### Restart kube-controller-manager

>WARNING! Delete the kube-controller-manager pods one by one and allow each pod to become ready
>before moving on to the next one. At least one kube-controller-manager must be available at all
>times for the cluster to operate normally.

```
kubectl -n kube-system delete pods kube-controller-manager-5f6bc6b89b-xxxxx

# Wait for a new kube-controller-manager pod to become ready.

kubectl -n kube-system delete pods kube-controller-manager-5f6bc6b89b-yyyyy
```

TODO

### Update existing SA tokens

```
for namespace in $(kubectl get ns --no-headers | awk '{print $1}'); do
    for token in $(kubectl get secrets --namespace "$namespace" --field-selector type=kubernetes.io/service-account-token -o name); do
        kubectl get $token --namespace "$namespace" -ojson | \
        jq -r --arg cert "$(cat combined.crt | base64 -w0)" '.data["ca.crt"]=$cert' |
        kubectl apply -f -
    done
done

# Verify all tokens have the new certificate
for namespace in $(kubectl get ns --no-headers | awk '{print $1}'); do
    for token in $(kubectl get secrets --namespace "$namespace" --field-selector type=kubernetes.io/service-account-token -o name); do
        echo $namespace $token
        kubectl get $token --namespace "$namespace" -ojsonpath='{.data.ca\.crt}' | base64 -d | openssl x509 -noout -text | grep After
    done
done
```

### Update kube-apiserver secret

```
kubectl -n kube-system get secrets kube-apiserver -ojson \
    | jq -r --arg cert "$(cat combined.crt | base64 -w0)" '.data["ca.crt"]=$cert' | kubectl apply -f -
kubectl -n kube-system get secrets kube-apiserver -ojson \
    | jq -r --arg cert "$(cat apiserver.crt | base64 -w0)" '.data["apiserver.crt"]=$cert' | kubectl apply -f -
kubectl -n kube-system get secrets kube-apiserver -ojson \
    | jq -r --arg key "$(cat apiserver.key | base64 -w0)" '.data["apiserver.key"]=$key' | kubectl apply -f -
```

### Update kubeconfig file

```
sed -i "s/\(certificate-authority-data: \).*/\1$(cat combined.crt | base64 -w0)/" \
    ../assets/cluster-assets/auth/kubeconfig
sed -i "s/\(client-certificate-data: \).*/\1$(cat admin.crt | base64 -w0)/" \
    ../assets/cluster-assets/auth/kubeconfig
sed -i "s/\(client-key-data: \).*/\1$(cat admin.key | base64 -w0)/" \
    ../assets/cluster-assets/auth/kubeconfig
```

### Copy CA self-signed certificate to all node

```
nodes=( $(kubectl get nodes -ojsonpath='{$.items[*].metadata.labels.lokomotive\.alpha\.kinvolk\.io/public-ipv4}'; echo) )
for n in "${nodes[@]}"; do
  scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=quiet \
      combined.crt core@${n}:/tmp/combined.crt \
      && ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=quiet \
        core@${n} 'sudo mv /tmp/combined.crt /etc/kubernetes/ca.crt;
            sudo chown root:root /etc/kubernetes/ca.crt'
done
```