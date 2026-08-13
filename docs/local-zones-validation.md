# F5 BNK/SPK Multi-Protocol Ingress Validation on AWS Local Zones

## Executive Summary
This report summarizes the technical validation of deploying F5 BNK (SPK) 5G Multi-Protocol Ingress within an AWS Local Zone. The objective was to validate 5G edge ingress capabilities across HTTP/2, TCP (Diameter), and SCTP. 

The environment deployed successfully via `awsbnkctl`, and Gateway API configurations were applied. While control-plane integration succeeded for TCP-based protocols, data-plane connectivity faced routing anomalies native to AWS VPC CNI, and SCTP encountered Gateway API protocol limitations.

## 1. Environment Deployment

The validation was performed in the Asia Pacific (Perth) Local Zone (`ap-southeast-2-per-1a`).
**Target Architecture:**
- **EKS Cluster:** v1.32+
- **Location:** an AWS Local Zone
- **Node Groups:** Managed Node Groups with `gp2` volumes (Local Zone constraint).
- **Networks:** Isolated subnets for Management (`10.0.3.x`), External / 5G Edge (`10.0.10.x`), and Pod Network.
- **F5 Components:** F5 CNE Controller, F5 TMM (DPDK-enabled), DSSM.

**Infrastructure Notes & Workarounds:**
- **Volume types:** Local Zones do not support `gp3`. `gp2` was enforced.
- **EICE:** EC2 Instance Connect Endpoints are not supported in Local Zones. SSM Session Manager was used for jumphost connectivity.

## 2. Telco Test Environment
We deployed 3 core validation scenarios targeting typical 5G edge workload profiles:
1. **HTTP/2** - Standard 5G SBI (Service Based Interface) style traffic.
2. **TCP / Diameter** - Emulated Diameter traffic on port 3868.
3. **SCTP** - Emulated Telco signaling traffic on port 9000.

## 3. Multi-Protocol Ingress Validation

### 3.1. HTTP/2 (SBI Style Traffic)
* **Configuration:** `Gateway` (HTTP 80) and `HTTPRoute` mapped to `http2-backend`.
* **Control Plane Status:** **PASS**
  * `F5BnkGateway` properly parsed `10.0.10.202/32`.
  * `http2-gateway` correctly acquired the IP and reached `Programmed=True`.
* **Data Plane Status:** **FAIL (Timeouts)**
  * **Observation:** Client traffic sent to `10.0.10.202:80` timed out.
  * **Root Cause:** Asymmetric routing / EKS VPC CNI IPAM conflict. The VPC CNI automatically reserved `10.0.10.202` on the worker node's primary ENI before F5 could utilize it. After manually correcting the ENI secondary IP assignments to point to TMM, traffic reached TMM but failed to return to the client. This indicates missing Source NAT (SNAT) rules, causing the backend pod to route its return traffic via the AWS default gateway rather than back through TMM.

### 3.2. Diameter (TCP L4)
* **Configuration:** `Gateway` (TCP 3868) and `L4Route` mapped to `diameter-backend`.
* **Control Plane Status:** **PASS**
  * `L4Route` successfully deployed and reached `Programmed=True`.
* **Data Plane Status:** **FAIL (Timeouts)**
  * **Observation:** Identical to HTTP/2. The L4 connection established through the `BNK_EXT` subnet but timed out awaiting server reply due to SNAT/Asymmetric routing constraints.

### 3.3. SCTP (Signaling)
* **Configuration:** `Gateway` (SCTP 9000) and `L4Route` mapped to `sctp-echo-backend`.
* **Control Plane Status:** **FAIL**
  * **Observation:** The Gateway listener rejected the configuration.
  * **Error:** `Listener protocol not supported: SCTP`.
  * **Root Cause:** Standard Kubernetes Gateway API (v1beta1/v1) does not natively support SCTP listeners in the cluster's current implementation. F5's proprietary `F5SPKIngressSCTP` CRD may be required over the standard Gateway API to achieve SCTP ingress.

## 4. Key Findings & Recommendations

### AWS / EKS Fabric Recommendations
1. **Isolate `BNK_EXT` from EKS Pod IPAM:** The AWS VPC CNI actively hijacked VIP addresses (`10.0.10.202`) from the `BNK_EXT` subnet because the worker node was attached to it. The `BNK_EXT` subnet should be excluded from `ENIConfig` or secondary IP pool allocations so that only F5 controllers manage those IPs.
2. **Automate Secondary IP Assignment:** F5 IPAM logs showed successful internal allocation, but the secondary IPs were not actively attached to the AWS EC2 ENI. F5 SPK needs an active AWS cloud-provider integration (or manual attachment workflow) to signal the AWS SDN to forward packets to the TMM MAC address.

### F5 Product Recommendations
1. **SNAT Configuration:** Data plane timeouts occurred because backend pods replied to the client directly via the AWS default route. Provide clear documentation on enabling SNAT on `F5BnkGateway` / `L4Route` to maintain symmetric routing in AWS environments.
2. **SCTP Support in Gateway API:** F5 should validate SCTP support within standard `Gateway` listeners. If `L4Route` is expected to handle SCTP, the `Gateway` standard validation webhook currently rejects it. Documentation should clarify whether to use `TCP` on the Gateway listener and `SCTP` on the `L4Route`, or fallback to `F5SPKIngressSCTP`.
