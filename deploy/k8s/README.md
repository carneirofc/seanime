# Seanime on Kubernetes

Reference manifests for running Seanime as an internet-facing, OIDC-protected
server behind an ingress controller. They are examples, not a chart: read them,
change the hostnames and selectors, and apply what fits your cluster.

```
kubectl apply -f namespace.yaml
kubectl -n seanime create secret generic seanime-oidc \
  --from-literal=client-secret='<the OIDC client secret>'
kubectl apply -f .        # or: kubectl apply -k .
```

## What you must change

| File | What |
|---|---|
| `configmap.yaml` | `trustedproxies`, `externalurl`, `[server.oidc]` issuer / client id / allowlist |
| `ingress.yaml` | hostname, `ingressClassName`, TLS issuer |
| `networkpolicy.yaml` | ingress-controller namespace and pod selector, DNS selector |
| `pvc.yaml` | storage classes and sizes, or point at existing volumes |
| `deployment.yaml` | image reference, resource limits |

Two of these are load-bearing rather than cosmetic:

- **`trustedproxies` must list your ingress controller's pod addresses and
  nothing else.** With OIDC configured the server refuses to start without it.
  Every address listed is trusted to state the real client IP in
  `X-Forwarded-For`; widen it to the pod CIDR and any pod in the cluster can
  forge client IPs and evade the authentication rate limiter.
- **`externalurl` must equal the Ingress host.** The OIDC redirect URI, the
  secure cookie scope and the CORS origin all derive from it. A mismatch
  presents as a login loop, not as an error message.

## Design notes

**One replica, Recreate.** The database is SQLite on a `ReadWriteOnce` volume.
A second writer corrupts it. This is not something to scale around.

**Capabilities are empty and should stay that way.** `server.capabilities` is
where privileged actions are granted — spawning processes, browsing the host
filesystem, installing extensions, replacing the binary, serving the Nakama peer
endpoints. Nothing else can grant them: not a source address, not a header, not
an authenticated session, and not the API itself. On an internet-facing
deployment, granting one means an attacker who obtains a session obtains that
capability too. `selfupdate` in particular is pointless here, since the
replacement binary is discarded on the next restart.

**The pod is hardened because the image is not minimal.** Transcoding needs
`ffmpeg`, which means a full Debian userland with a shell. So: non-root
(uid 10001), `readOnlyRootFilesystem`, all capabilities dropped, no privilege
escalation, `seccompProfile: RuntimeDefault`, and the namespace enforces the
`restricted` Pod Security Standard so a future edit that loosens any of this is
rejected at admission. `/data` and `/tmp` are the only writable paths, and
Seanime refuses to spawn an executable that lives under any directory it writes
to — so a file staged in `/data` cannot become the transcoder.

**No service account token.** `automountServiceAccountToken: false`. Nothing in
Seanime talks to the Kubernetes API, so a compromised process should not find a
credential for it sitting in the filesystem.

**The media volume is mounted read-only.** Seanime scans and streams from it and
never needs to write. A read-only mount means neither a bug nor a deliberately
granted `filesystem` capability can damage the library.

**NetworkPolicy is the containment layer that matters.** Seanime fetches URLs
chosen by remote content, by extensions, and by its proxy endpoints. It refuses
non-public destinations at dial time, but that check runs *in the same process*
as the code it defends against — an extension executes in-process, on the wrong
side of it. The egress policy excludes RFC 1918, carrier-grade NAT, loopback and
`169.254.0.0/16` (the cloud metadata endpoint) at the CNI, which does not care
whether the process is compromised. The ingress policy admits only the ingress
controller, so a neighbouring pod cannot connect straight to `:43211` and skip
TLS termination, forwarded-header handling, and whatever runs at your edge.

This requires a CNI that implements NetworkPolicy — Calico, Cilium, Antrea,
Weave. **Flannel silently ignores these objects**: they will apply cleanly and
enforce nothing. Confirm before you rely on it.

**config.toml is copied in by an init container.** Seanime rewrites its config at
runtime, so it cannot be a read-only ConfigMap mount. The copy runs on every
start, which makes the ConfigMap authoritative and any edit made inside the pod
last only until the next restart. The `checksum/config` annotation on the pod
template has to be bumped by hand when the ConfigMap changes — see
`kustomization.yaml` for the generator alternative that does it automatically.

## Verifying the posture

```sh
# Capabilities: expect an empty list and a warning at startup.
kubectl -n seanime logs deploy/seanime | grep -i 'capabilities'

# Privileged routes: expect 403 regardless of how the request is dressed up.
kubectl -n seanime exec deploy/seanime -- \
  curl -s -o /dev/null -w '%{http_code}\n' \
  -X POST http://127.0.0.1:43211/api/v1/open-in-explorer \
  -H 'Host: 127.0.0.1:43211' -H 'Origin: http://127.0.0.1:43211' \
  -d '{"path":"/"}'

# Egress containment: the metadata endpoint must not be reachable.
kubectl -n seanime exec deploy/seanime -- \
  curl -s --max-time 5 http://169.254.169.254/ ; echo "exit=$?"

# Direct pod access from elsewhere in the cluster must be refused.
kubectl run probe --rm -it --image=curlimages/curl --restart=Never -- \
  curl -s --max-time 5 http://seanime.seanime.svc.cluster.local/api/v1/status
```

The last two should fail. If they succeed, your CNI is not enforcing
NetworkPolicy.

## What these manifests do not do

- No backups. The SQLite database and your AniList tokens live on
  `seanime-data`; snapshot it.
- No HA. See the single-replica note above.
- No monitoring or log shipping.
- Nothing about the identity provider's own configuration. The redirect URI it
  must allow is `https://<your host>/api/v1/auth/oidc/callback`.
