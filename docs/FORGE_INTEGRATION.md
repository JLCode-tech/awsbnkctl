# Forge Integration

`awsbnkctl` can optionally register the clusters it provisions with **forge**, a GUI for operating BNK deployments. 

This integration is a **write-only handoff**: `awsbnkctl` operates AWS infrastructure, reports the cluster connection details to forge, and then forge connects directly to the cluster.

---

## 1. The Peer-Read Model

Both `awsbnkctl` and forge treat AWS as the single source of truth. They operate as peers:

```
        [AWS — Single Source of Truth]
              ▲                  ▲
              │ reads            │ reads (forge's own credentials)
              │                  │
         [awsbnkctl] ── registers pointers ──► [forge] ──► [user GUI]
```

- **Independent Operations:** `awsbnkctl` never asks forge for cluster health. It queries AWS directly.
- **Bootstrap Credentials:** `awsbnkctl` gives forge a short-lived presigned bootstrap kubeconfig. Forge handles refreshing credentials on its own identity after that.
- **No IaC Sync:** `awsbnkctl` simply points forge to the new cluster. It does not sync infrastructure state or "tfstate" equivalents.

---

## 2. Enabling Forge Integration

Forge integration is **opt-in** via the `forge:` block in `cluster.yaml`:

```yaml
forge:
  enabled: true
  url: http://localhost:8000
  mcpUrl: http://localhost:8081/mcp/
  username: admin
  credentialTemplateId: 1
```

If `forge.enabled` is missing or `false`, the integration is skipped.

---

## 3. MCP-Preferred, REST-Fallback

Forge provides both an MCP (Model Context Protocol) endpoint and a REST API.

- `awsbnkctl` will always attempt registration via **MCP first**.
- If a specific action isn't available in MCP (a catalog gap), it falls back gracefully to the **REST API**.
- On failures, it uses a soft-fail strategy so the actual cluster provisioning isn't interrupted.

---

## 4. Soft-Fail & Retry Strategy

Because AWS provisioning is time-consuming, a forge outage shouldn't break the whole pipeline.

- If forge registration fails on `up`, `awsbnkctl` writes `forge_link.json` with `status: pending` and returns success.
- The operator can retry later with `awsbnkctl forge register`.
- A successful link is saved to `.awsbnkctl/<cluster-name>/forge_link.json`.

---

## 5. Configuration Overrides

It is unsafe to store passwords in `cluster.yaml`. You can override forge settings using environment variables:

| Setting | Priority 1 (Env Var) | Priority 2 (YAML) | Default |
|---|---|---|---|
| **URL** | `AWSBNKCTL_FORGE_URL` | `forge.url` | `http://localhost:8000` |
| **Password** | `AWSBNKCTL_FORGE_PASSWORD` | `forge.password` | *built-in dev default* |

> [!WARNING]
> Always use `AWSBNKCTL_FORGE_PASSWORD` in real environments!

---

## 6. What Forge Does NOT Do

- **Manage Infrastructure:** Forge does not spin up or tear down EKS clusters. That is `awsbnkctl`'s job.
- **Act as Source of Truth:** `awsbnkctl doctor` and `awsbnkctl status` rely on AWS, not forge.
