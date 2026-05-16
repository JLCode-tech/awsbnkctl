# ============================================================
# sriov.tf — SR-IOV CNI + SR-IOV device plugin (PRD 07 §
# "Implementation outline" § "Multus + SR-IOV stack").
#
# Two DaemonSets:
#
#   1. SR-IOV CNI — upstream k8snetworkplumbingwg/sriov-cni; drops the
#      sriov binary onto /opt/cni/bin so Multus can chain to it after
#      VPC CNI completes.
#   2. SR-IOV device plugin — upstream
#      k8snetworkplumbingwg/sriov-network-device-plugin; enumerates VFs
#      from /sys/class/net/<eth>/device/sriov_* and advertises them as
#      a schedulable resource (intel.com/sriov by default).
#
# Plus a ConfigMap that pins the device pool by PCIe vendor/device ID.
# **TODO(spike day 2 — operator-run, NOT this sprint):** the ENA VF
# vendor/device IDs in the ConfigMap are PLACEHOLDERS. PRD 07 §
# "Multus + SR-IOV stack" notes "ENA VFs on c5n.* instances surface as
# Amazon Elastic Network Adapter — vendor 1d0f, device ec20 (or
# similar; the spike confirms the exact IDs)". Until the spike
# resolves the IDs the device plugin's pool will be empty on real
# c5n instances; the failure mode is "pods scheduled for
# intel.com/sriov: 1 sit Pending forever". This is the load-bearing
# spike outcome PRD 07 § "Open questions" tracks.
#
# Gated on var.enable_sriov (default true). If the spike's day-3
# BNK-acceptance check fails (PRD 07 § "Spike fail modes" §"VF
# appears but BNK rejects it") the v0.x fallback is to set
# enable_sriov = false and run BNK on the multi-ENI shape.
# ============================================================

locals {
  sriov_cni_image    = "ghcr.io/k8snetworkplumbingwg/sriov-cni:v2.7.0"
  sriov_plugin_image = "ghcr.io/k8snetworkplumbingwg/sriov-network-device-plugin:v3.6.2"

  # TODO(spike day 2): replace these placeholders with the real ENA VF
  # vendor/device IDs surfaced by `lspci -nn | grep -i ena` on a live
  # c5n.4xlarge. The PRD's hypothesis is vendor=1d0f device=ec20; if
  # that's wrong the spike output goes here verbatim.
  ena_vf_vendor_id = "1d0f"
  ena_vf_device_id = "ec20"
}

# ----------------------------------------------------------------
# SR-IOV CNI DaemonSet — drops the binary onto every node's
# /opt/cni/bin. Lifted from the v2.7.0 upstream daemonset YAML.
# ----------------------------------------------------------------
resource "kubernetes_manifest" "sriov_cni_daemonset" {
  count = var.enable_sriov ? 1 : 0

  manifest = {
    apiVersion = "apps/v1"
    kind       = "DaemonSet"
    metadata = {
      name      = "kube-sriov-cni-ds"
      namespace = "kube-system"
      labels = {
        "tier"                         = "node"
        "app"                          = "sriov-cni"
        "app.kubernetes.io/managed-by" = "awsbnkctl"
      }
    }
    spec = {
      selector = {
        matchLabels = { name = "sriov-cni" }
      }
      updateStrategy = { type = "RollingUpdate" }
      template = {
        metadata = {
          labels = { "tier" = "node", "app" = "sriov-cni", "name" = "sriov-cni" }
        }
        spec = {
          hostNetwork = true
          # NB: PRD 07 selected a single instance family per cluster;
          # if we ever support mixed-family we'd add a nodeSelector
          # on awsbnkctl.io/role=sriov-data-plane here.
          tolerations = [{ operator = "Exists", effect = "NoSchedule" }]
          containers = [
            {
              name            = "kube-sriov-cni"
              image           = local.sriov_cni_image
              imagePullPolicy = "IfNotPresent"
              securityContext = { privileged = true }
              resources = {
                requests = { cpu = "100m", memory = "50Mi" }
                limits   = { cpu = "100m", memory = "50Mi" }
              }
              volumeMounts = [
                { name = "cnibin", mountPath = "/host/opt/cni/bin" },
              ]
            },
          ]
          volumes = [
            { name = "cnibin", hostPath = { path = "/opt/cni/bin" } },
          ]
        }
      }
    }
  }

  depends_on = [
    module.eks,
  ]
}

# ----------------------------------------------------------------
# SR-IOV device plugin ConfigMap — declares the VF pool the plugin
# advertises under var.sriov_resource_name. PRD 07 §"Open questions"
# day 2 — the placeholder IDs above MUST be replaced post-spike.
# ----------------------------------------------------------------
resource "kubernetes_manifest" "sriov_device_plugin_config" {
  count = var.enable_sriov ? 1 : 0

  manifest = {
    apiVersion = "v1"
    kind       = "ConfigMap"
    metadata = {
      name      = "sriovdp-config"
      namespace = "kube-system"
      labels = {
        "app.kubernetes.io/managed-by" = "awsbnkctl"
      }
    }
    data = {
      "config.json" = jsonencode({
        resourceList = [
          {
            # The schedulable resource key BNK's CNEInstance reconciler
            # looks up. Override via var.sriov_resource_name.
            resourceName = trimprefix(var.sriov_resource_name, "intel.com/")
            resourcePrefix = trimsuffix(
              replace(var.sriov_resource_name, "/${trimprefix(var.sriov_resource_name, "intel.com/")}", ""),
              "/",
            )
            selectors = {
              vendors = [local.ena_vf_vendor_id]
              devices = [local.ena_vf_device_id]
              drivers = ["ena"]
              # isRdma=false keeps this lane plain ENA (no EFA/RDMA
              # libfabric chaining). PRD 07 § "Background" pins
              # ENA-not-EFA for v1.0.
              isRdma = false
            }
          },
        ]
      })
    }
  }
}

# ----------------------------------------------------------------
# SR-IOV device plugin DaemonSet.
# ----------------------------------------------------------------
resource "kubernetes_manifest" "sriov_device_plugin_daemonset" {
  count = var.enable_sriov ? 1 : 0

  manifest = {
    apiVersion = "apps/v1"
    kind       = "DaemonSet"
    metadata = {
      name      = "kube-sriov-device-plugin"
      namespace = "kube-system"
      labels = {
        "tier"                         = "node"
        "app"                          = "sriov-device-plugin"
        "app.kubernetes.io/managed-by" = "awsbnkctl"
      }
    }
    spec = {
      selector = {
        matchLabels = { name = "sriov-device-plugin" }
      }
      updateStrategy = { type = "RollingUpdate" }
      template = {
        metadata = {
          labels = { "tier" = "node", "app" = "sriov-device-plugin", "name" = "sriov-device-plugin" }
        }
        spec = {
          hostNetwork        = true
          hostPID            = true
          serviceAccountName = "default"
          tolerations        = [{ operator = "Exists", effect = "NoSchedule" }]
          containers = [
            {
              name            = "kube-sriovdp"
              image           = local.sriov_plugin_image
              imagePullPolicy = "IfNotPresent"
              args            = ["--log-dir=sriovdp", "--log-level=10"]
              securityContext = {
                privileged = true
              }
              resources = {
                requests = { cpu = "250m", memory = "40Mi" }
                limits   = { cpu = "1", memory = "200Mi" }
              }
              volumeMounts = [
                { name = "devicesock", mountPath = "/var/lib/kubelet/", readOnly = false },
                { name = "log", mountPath = "/var/log" },
                { name = "config-volume", mountPath = "/etc/pcidp" },
                { name = "device-info", mountPath = "/var/run/k8s.cni.cncf.io/devinfo/dp" },
              ]
            },
          ]
          volumes = [
            { name = "devicesock", hostPath = { path = "/var/lib/kubelet/" } },
            { name = "log", hostPath = { path = "/var/log" } },
            { name = "device-info", hostPath = { path = "/var/run/k8s.cni.cncf.io/devinfo/dp", type = "DirectoryOrCreate" } },
            {
              name = "config-volume"
              configMap = {
                name  = "sriovdp-config"
                items = [{ key = "config.json", path = "config.json" }]
              }
            },
          ]
        }
      }
    }
  }

  depends_on = [
    kubernetes_manifest.sriov_device_plugin_config,
    kubernetes_manifest.sriov_cni_daemonset,
  ]
}
