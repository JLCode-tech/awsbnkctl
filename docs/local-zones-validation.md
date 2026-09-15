# AWS Local Zones: known results

A point-in-time validation of BNK ingress in the Asia Pacific (Perth) Local Zone (`ap-southeast-2-per-1a`). The manifests are preserved under [`examples/local-zone/manifests/`](../examples/local-zone/manifests/) in the 2.4 shape; the example README lists the per-protocol outcome.

| Area | Result | Handled in code |
|---|---|---|
| EBS volumes | Local Zones have no `gp3`; `gp2` is used | phase 10 forces `gp2` on Local Zone node groups |
| Control-plane subnets | EKS control-plane ENIs cannot live in a Local Zone subnet | phase 08 filters Local Zone subnets out of the control-plane set |
| Jumphost access | EC2 Instance Connect Endpoints are not offered; SSM Session Manager was used | not automated; `testing.jumphost` assumes EICE |
| HTTP/2 and Diameter (TCP) data path | Gateway and routes `Programmed=True`; client traffic timed out because backend pods answered via the VPC default route instead of TMM (no SNAT) | on 2.4 the `Infra` `egressDefaults` and `GatewaySettings` `Automap` cover this; not retested in a Local Zone |
| VIP address | the VPC CNI reserved the VIP address on the worker's primary ENI before TMM owned it | keep the `BNK_EXT` subnet out of the CNI's pod IPAM |
| SCTP listener | rejected: `Listener protocol not supported: SCTP` | none; Gateway API listeners are HTTP, HTTPS, TLS, TCP, UDP |
