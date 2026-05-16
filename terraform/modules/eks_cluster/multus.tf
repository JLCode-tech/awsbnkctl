# ============================================================
# multus.tf — Multus CNI DaemonSet (PRD 07 § "Implementation outline"
# § "Multus + SR-IOV stack").
#
# Upstream: k8snetworkplumbingwg/multus-cni v4.x thick plugin.
# https://github.com/k8snetworkplumbingwg/multus-cni
#
# Gated on var.enable_multus (default true). When the spike surfaces
# a hypothesis mismatch and the v0.x fallback is "no Multus" the user
# flips the bool and the module no-ops the DaemonSet without leaving
# stale CRDs behind.
#
# Chains AFTER AWS VPC CNI completes — Multus reads the pod annotation
# `k8s.v1.cni.cncf.io/networks` and invokes the SR-IOV CNI (or any
# other configured CNI) for the per-VF attachment, while VPC CNI keeps
# providing the pod's primary network identity for normal
# pod-to-pod traffic.
# ============================================================

locals {
  # multus_image pins the thick-plugin v4 release. Spike confirms this
  # tag is the right one for the EKS 1.30 cluster.
  multus_image = "ghcr.io/k8snetworkplumbingwg/multus-cni:v4.0.2-thick"
}

# ----------------------------------------------------------------
# NetworkAttachmentDefinition CRD — Multus depends on this CRD being
# registered before its DaemonSet starts. We install it as a YAML
# manifest. The upstream `deployments/multus-daemonset.yml` bundles
# the CRD + the DaemonSet in one document; we split them here so
# terraform's dependency graph is explicit.
# ----------------------------------------------------------------
resource "kubernetes_manifest" "multus_crd" {
  count = var.enable_multus ? 1 : 0

  manifest = {
    apiVersion = "apiextensions.k8s.io/v1"
    kind       = "CustomResourceDefinition"
    metadata = {
      name = "network-attachment-definitions.k8s.cni.cncf.io"
    }
    spec = {
      group = "k8s.cni.cncf.io"
      scope = "Namespaced"
      names = {
        plural   = "network-attachment-definitions"
        singular = "network-attachment-definition"
        kind     = "NetworkAttachmentDefinition"
        shortNames = [
          "net-attach-def",
        ]
      }
      versions = [
        {
          name    = "v1"
          served  = true
          storage = true
          schema = {
            openAPIV3Schema = {
              type        = "object"
              description = "NetworkAttachmentDefinition is a CRD schema specified by the Network Plumbing Working Group to express the intent for attaching pods to one or more logical or physical networks."
              properties = {
                spec = {
                  type        = "object"
                  description = "NetworkAttachmentDefinition spec"
                  properties = {
                    config = {
                      type        = "string"
                      description = "NetworkAttachmentDefinition config CNI JSON"
                    }
                  }
                }
              }
            }
          }
        },
      ]
    }
  }

  depends_on = [
    module.eks,
  ]
}

# ----------------------------------------------------------------
# Multus ServiceAccount + ClusterRole + ClusterRoleBinding +
# ConfigMap + DaemonSet. Lifted from the upstream thick-plugin
# manifest at v4.0.2.
#
# We avoid the upstream `kubectl apply -f` shape because the
# kubernetes_manifest resource gives terraform a per-object lifecycle
# (drift detection, destroy ordering). The trade-off: each manifest
# requires a kubernetes provider with the cluster's kubeconfig in
# place — the root module wires that via the post-apply provider
# config block in terraform/providers.tf.
# ----------------------------------------------------------------
resource "kubernetes_manifest" "multus_serviceaccount" {
  count = var.enable_multus ? 1 : 0

  manifest = {
    apiVersion = "v1"
    kind       = "ServiceAccount"
    metadata = {
      name      = "multus"
      namespace = "kube-system"
      labels = {
        "app.kubernetes.io/managed-by" = "awsbnkctl"
      }
    }
  }

  depends_on = [
    kubernetes_manifest.multus_crd,
  ]
}

resource "kubernetes_manifest" "multus_clusterrole" {
  count = var.enable_multus ? 1 : 0

  manifest = {
    apiVersion = "rbac.authorization.k8s.io/v1"
    kind       = "ClusterRole"
    metadata = {
      name = "multus"
      labels = {
        "app.kubernetes.io/managed-by" = "awsbnkctl"
      }
    }
    rules = [
      {
        apiGroups = ["k8s.cni.cncf.io"]
        resources = ["*"]
        verbs     = ["get", "list", "watch"]
      },
      {
        apiGroups = [""]
        resources = ["pods", "pods/status"]
        verbs     = ["get", "list", "watch", "update", "patch"]
      },
      {
        apiGroups = [""]
        resources = ["events"]
        verbs     = ["create", "patch", "update"]
      },
    ]
  }
}

resource "kubernetes_manifest" "multus_clusterrolebinding" {
  count = var.enable_multus ? 1 : 0

  manifest = {
    apiVersion = "rbac.authorization.k8s.io/v1"
    kind       = "ClusterRoleBinding"
    metadata = {
      name = "multus"
    }
    roleRef = {
      apiGroup = "rbac.authorization.k8s.io"
      kind     = "ClusterRole"
      name     = "multus"
    }
    subjects = [
      {
        kind      = "ServiceAccount"
        name      = "multus"
        namespace = "kube-system"
      },
    ]
  }

  depends_on = [
    kubernetes_manifest.multus_clusterrole,
    kubernetes_manifest.multus_serviceaccount,
  ]
}

# multus thick-plugin DaemonSet. Each kubelet drops the multus binary
# onto /opt/cni/bin and writes /etc/cni/net.d/00-multus.conf marking
# the AWS VPC CNI as the primary network.
resource "kubernetes_manifest" "multus_daemonset" {
  count = var.enable_multus ? 1 : 0

  manifest = {
    apiVersion = "apps/v1"
    kind       = "DaemonSet"
    metadata = {
      name      = "kube-multus-ds"
      namespace = "kube-system"
      labels = {
        "tier"                         = "node"
        "app"                          = "multus"
        "app.kubernetes.io/managed-by" = "awsbnkctl"
      }
    }
    spec = {
      selector = {
        matchLabels = {
          name = "multus"
        }
      }
      updateStrategy = {
        type = "RollingUpdate"
      }
      template = {
        metadata = {
          labels = {
            "tier" = "node"
            "app"  = "multus"
            "name" = "multus"
          }
        }
        spec = {
          hostNetwork        = true
          tolerations        = [{ operator = "Exists", effect = "NoSchedule" }]
          serviceAccountName = "multus"
          containers = [
            {
              name            = "kube-multus"
              image           = local.multus_image
              command         = ["/thin_entrypoint"]
              args            = ["--multus-conf-file=auto", "--cni-version=0.3.1"]
              imagePullPolicy = "IfNotPresent"
              resources = {
                requests = { cpu = "100m", memory = "50Mi" }
                limits   = { cpu = "100m", memory = "50Mi" }
              }
              securityContext = {
                privileged = true
              }
              volumeMounts = [
                { name = "cni", mountPath = "/host/etc/cni/net.d" },
                { name = "cnibin", mountPath = "/host/opt/cni/bin" },
              ]
            },
          ]
          volumes = [
            { name = "cni", hostPath = { path = "/etc/cni/net.d" } },
            { name = "cnibin", hostPath = { path = "/opt/cni/bin" } },
          ]
        }
      }
    }
  }

  depends_on = [
    kubernetes_manifest.multus_clusterrolebinding,
  ]
}
