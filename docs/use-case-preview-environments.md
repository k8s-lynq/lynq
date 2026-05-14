---
description: "Per-PR preview environments provisioned from CI with a single SQL INSERT. The row exists while the PR is open; deleting it cleans up every resource."
---

# Per-PR Preview Environments

Every team that has tried per-PR preview environments has hit the same wall: cleanup. The `helm install` on PR open is easy. The teardown CI job fails silently, gets skipped when someone force-merges, or simply doesn't run when a branch is deleted. Stale environments pile up. Someone has to clean them manually.

With Lynq, the cleanup problem doesn't exist. Deleting the row is the teardown. The CI job that runs `DELETE FROM preview_environments` is the only cleanup you need — and if it fails, the environment stays provisioned rather than getting orphaned.

::: tip Time to working
~5 minutes to configure. Each PR gets a namespace, Deployment, Service, and TLS Ingress within ~60 seconds of the INSERT.
:::

## How It Works

- CI inserts one row per open PR. Lynq provisions a full isolated environment — Namespace, Deployment, Service, and Ingress with TLS.
- Each environment is reachable at `pr-<number>.<base_domain>` within ~60 seconds of the INSERT.
- When the PR closes, CI runs `DELETE`. Lynq removes the namespace and all resources inside it on the next sync.

## Database Schema

```sql
CREATE TABLE preview_environments (
  env_id       VARCHAR(63)   PRIMARY KEY,    -- e.g. 'pr-1234'
  repo         VARCHAR(255)  NOT NULL,
  branch       VARCHAR(255)  NOT NULL,
  commit_sha   VARCHAR(40)   NOT NULL,
  image_tag    VARCHAR(255)  NOT NULL,        -- fully-qualified Docker image
  base_domain  VARCHAR(255)  NOT NULL,        -- e.g. 'preview.company.com'
  ttl_hours    INT           DEFAULT 48,
  opened_by    VARCHAR(100),
  is_active    BOOLEAN       DEFAULT TRUE,
  created_at   TIMESTAMP     DEFAULT CURRENT_TIMESTAMP,
  updated_at   TIMESTAMP     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

## CI Integration

```bash
# On PR open / commit push — upsert so pushes update the running environment
mysql -e "
  INSERT INTO preview_environments
    (env_id, repo, branch, commit_sha, image_tag, base_domain, opened_by)
  VALUES
    ('pr-$PR_NUMBER', '$REPO', '$BRANCH', '$SHA', '$IMAGE', 'preview.company.com', '$ACTOR')
  ON DUPLICATE KEY UPDATE
    commit_sha = VALUES(commit_sha),
    image_tag  = VALUES(image_tag),
    updated_at = NOW();
"

# On PR close / merge
mysql -e "DELETE FROM preview_environments WHERE env_id = 'pr-$PR_NUMBER';"
```

## LynqHub

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: preview-hub
  namespace: lynq-system
spec:
  source:
    type: mysql
    syncInterval: 30s    # fast polling — new PRs should get environments quickly
    mysql:
      host: mysql.internal.svc.cluster.local
      port: 3306
      database: ci_db
      table: preview_environments
      username: lynq_reader
      passwordRef:
        name: mysql-credentials
        key: password
  valueMappings:
    uid: env_id
    activate: is_active
  extraValueMappings:
    imageTag: image_tag
    branch: branch
    commitSha: commit_sha
    baseDomain: base_domain
    openedBy: opened_by
```

## LynqForm

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqForm
metadata:
  name: preview-stack
  namespace: lynq-system
spec:
  hubId: preview-hub

  namespaces:
    - id: ns
      nameTemplate: "preview-{{ .uid }}"
      spec:
        apiVersion: v1
        kind: Namespace
        metadata:
          labels:
            preview-env: "{{ .uid }}"
            opened-by: "{{ .openedBy }}"

  deployments:
    - id: app
      nameTemplate: "{{ .uid }}-app"
      targetNamespace: "preview-{{ .uid }}"
      dependIds: ["ns"]
      deletionPolicy: Delete
      waitForReady: true
      timeoutSeconds: 300
      spec:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          labels:
            app: "{{ .uid }}"
            commit: "{{ .commitSha }}"
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: "{{ .uid }}"
          template:
            metadata:
              labels:
                app: "{{ .uid }}"
                commit: "{{ .commitSha }}"
            spec:
              containers:
                - name: app
                  image: "{{ .imageTag }}"
                  ports:
                    - containerPort: 8080
                  env:
                    - name: PREVIEW_ENV_ID
                      value: "{{ .uid }}"
                    - name: BRANCH
                      value: "{{ .branch }}"
                  resources:
                    requests:
                      cpu: 200m
                      memory: 256Mi
                    limits:
                      cpu: 500m
                      memory: 512Mi

  services:
    - id: svc
      nameTemplate: "{{ .uid }}-svc"
      targetNamespace: "preview-{{ .uid }}"
      dependIds: ["app"]
      deletionPolicy: Delete
      spec:
        apiVersion: v1
        kind: Service
        spec:
          selector:
            app: "{{ .uid }}"
          ports:
            - port: 80
              targetPort: 8080

  ingresses:
    - id: ingress
      nameTemplate: "{{ .uid }}-ingress"
      targetNamespace: "preview-{{ .uid }}"
      dependIds: ["svc"]
      deletionPolicy: Delete
      spec:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          annotations:
            cert-manager.io/cluster-issuer: letsencrypt-prod
            nginx.ingress.kubernetes.io/proxy-read-timeout: "60"
        spec:
          ingressClassName: nginx
          tls:
            - hosts:
                - "{{ .uid }}.{{ .baseDomain }}"
              secretName: "{{ .uid }}-tls"
          rules:
            - host: "{{ .uid }}.{{ .baseDomain }}"
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: "{{ .uid }}-svc"
                        port:
                          number: 80
```

## Automatic TTL Cleanup

CI deletes the row when a PR closes, but PRs can stay open for days. Add a scheduled job to expire environments that have outlived their TTL:

```sql
-- Deactivate (Lynq removes resources but preserves the row)
UPDATE preview_environments
SET is_active = FALSE
WHERE is_active = TRUE
  AND created_at < NOW() - INTERVAL ttl_hours HOUR;

-- Or hard-delete (Lynq removes resources and the row is gone)
DELETE FROM preview_environments
WHERE created_at < NOW() - INTERVAL ttl_hours HOUR;
```

## Verify It Works

```bash
# List all active preview environments
kubectl get lynqnodes -n lynq-system -l lynq.sh/hub=preview-hub
# NAME                    READY   DESIRED   AGE
# pr-1234-preview-stack   True    3/3       2m
# pr-1235-preview-stack   True    3/3       5m

# Get the URL for PR 1234
kubectl get ingress -n preview-pr-1234
# NAME              CLASS   HOSTS                          ADDRESS      PORTS
# pr-1234-ingress   nginx   pr-1234.preview.company.com    10.0.0.50    80, 443

# Watch a new environment appear after CI inserts a row
kubectl get lynqnodes -n lynq-system -w
```

## Caveats

- **cert-manager must be installed** for TLS to work. Without it, the Ingress will be created but the certificate will never issue. For non-TLS previews, remove the `tls:` block and the cert-manager annotation.
- **Image must be pushed before the INSERT** — Lynq will attempt to start the pod immediately. If the image doesn't exist yet, the pod will fail with `ImagePullBackOff` until it appears.
- **`syncInterval: 30s` increases database query frequency** compared to the default 1 minute. For 100+ open PRs this is negligible, but factor it in if your database connection pool is small.

## See Also

- [Developer Sandbox Environments](./use-case-sandbox-environments.md) — longer-lived isolated environments per developer account
- [Custom Domains](./use-case-custom-domains.md) — add per-PR subdomains via ExternalDNS
- [Policies](./policies.md) — `deletionPolicy`, `creationPolicy`, `maxSkew`
