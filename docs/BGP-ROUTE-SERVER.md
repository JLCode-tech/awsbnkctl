# BGP peering with AWS Route Server

Every deployable example under `examples/` sets `bnk.bgp: true`. This page is
the one procedure they all point at: what the flag does, what you still have to
create by hand, how to load the BGP stanza, how to verify, and how to tear it
down without stranding the cluster. It is written for BNK 2.4; the 2.3 form
(F5SPKVlan `allowed_services`) lives on the `release-2.3` branch.

## What `bnk.bgp: true` does, and does not do

The TMM pod already runs the routing container (`f5-tmm-routing`) on every
cluster awsbnkctl builds — the embedded CNEInstance enables `dynamicRouting`,
and the 2.4 TMM chart still defaults to the ZebOS image (`ZEBOS_STATE=legacy`).
The routing container peers from TMM's external self IP. What stops a peer from
reaching it is the data-plane security group, and `bnk.bgp` opens it:

| Phase | Effect |
| --- | --- |
| 07 `iam` | Admits TCP 179 (BGP) and UDP 3784 (BFD) into the data-plane security group `SG_BNK_DATA` from `network.dataPath.external.cidr`. |
| 23b `spk-vlan-gateway-class` | Renders nothing extra. The BNK 2.4 `Infra` CR that defines the external VLAN has no per-VLAN allowed-services (the 2.3 `F5SPKVlan` carried `allowed_services` for the same two ports), and the 2.4 BGP how-tos have no step that opens ports on the VLAN: BGP and BFD reach the routing container by default. The phase logs that when the flag is set and names the ConfigMap below. |

It does **not** create a Route Server, an endpoint, a peer, or the BGP stanza.
Those are below. With no peer the flag is inert, which is why every example can
carry it.

The default VIP path still works without BGP: the cne-controller runs in AWS
cloud mode and assigns the Gateway VIP as a secondary IP on TMM's external ENI,
so anything in the VPC reaches it through ordinary VPC routing. BGP is the
alternative for VIPs that are *not* on the external subnet, for advertising
into route tables the controller does not touch, and for BFD-speed failover.

## Which BGP configuration: the ZebOS ConfigMap, not the routing CRs

FLO 2.30 installs BNK 2.4.0 with the **ZebOS** routing container: F5's 2.4
CNEInstance examples set the TMM env `ZEBOS_STATE: legacy`, the CNEInstance CRD
has no OcNOS switch (`dynamicRouting.enabled` only), and the pod runs
`f5dr-img`. ZebOS takes its BGP configuration from the ConfigMap
`f5-tmm-dynamic-routing-template` (key `ZebOS.conf`) in the CNEInstance
namespace — F5 "Set up dynamic routing with BGP" — and its BFD watcher
hot-reloads it, so no TMM restart is needed. FLO creates the ConfigMap with an
empty `ZebOS.conf`; every `bgp-route-server.yaml` under `examples/` is that
ConfigMap filled in.

The 2.4 `GlobalRoutingConfig` / `RoutingTemplate` CRs are the OcNOS path (the
routing container's `grpcconf_watcher`, which only the OcNOS image runs). On a
FLO-installed 2.4.0 cluster they stay `Programmed=False` with `Failure in
sending CR config to grpc endpoints` and the cne-controller logs `No heartbeat
from grpc-svc-f5dr: [<tmm pod ip>]:8095` (live 2026-09-12, `bnk-bgp-test`).
Do not use them until BNK ships OcNOS through FLO.

## Prerequisites

- A cluster from one of the examples, `up` complete, `awsbnkctl status` green.
- `aws` CLI with the same profile you ran `up` with. Route Server needs
  `ec2:*RouteServer*` permissions on top of the usual set.
- These values from `.awsbnkctl/<cluster>/state.env`:

  ```bash
  STATE=.awsbnkctl/<cluster>/state.env
  VPC_ID=$(grep ^VPC_ID= $STATE | cut -d= -f2)
  BNK_EXT_SUBNET=$(grep ^BNK_EXT_SUBNET= $STATE | cut -d= -f2)
  PUBLIC_RTB=$(grep ^PUBLIC_RTB= $STATE | cut -d= -f2)
  PRIVATE_RTB=$(grep ^PRIVATE_RTB= $STATE | cut -d= -f2)
  TMM_EXT_SELFIP=$(grep ^TMM_EXT_SELFIP= $STATE | cut -d= -f2)
  ```

  `TMM_EXT_SELFIP` is the address the F5 IPAM controller allocated from the
  external self-IP pool (`.224`–`.254`; `.225` on a fresh cluster), which phase
  23b put on the external ENI. It is **not** the nominal `.240` the examples
  use as router-id — always read it from `state.env`.

ASN convention used by every `bgp-route-server.yaml`: **TMM is 65000, the
Route Server is 65001.** Change both sides together if you must change either.

> [!IMPORTANT]
> A Route Server endpoint is a billable ENI (**about $0.75/hour**) that lives in
> the external data-path subnet. `awsbnkctl down` does not know about it, and
> the subnet cannot be deleted while it exists. Tear the Route Server down
> **before** `down` — see the last section.

## 1. Create the Route Server and attach it to the VPC

```bash
RS_ID=$(aws ec2 create-route-server --amazon-side-asn 65001 \
  --query RouteServer.RouteServerId --output text)
until [ "$(aws ec2 describe-route-servers --route-server-ids $RS_ID \
  --query 'RouteServers[0].State' --output text)" = available ]; do sleep 5; done

aws ec2 associate-route-server --route-server-id $RS_ID --vpc-id $VPC_ID
aws ec2 get-route-server-associations --route-server-id $RS_ID   # wait for "associated"
```

## 2. Create an endpoint in the external subnet

```bash
RSE_ID=$(aws ec2 create-route-server-endpoint --route-server-id $RS_ID \
  --subnet-id $BNK_EXT_SUBNET \
  --query RouteServerEndpoint.RouteServerEndpointId --output text)
until [ "$(aws ec2 describe-route-server-endpoints --route-server-endpoint-ids $RSE_ID \
  --query 'RouteServerEndpoints[0].State' --output text)" = available ]; do sleep 5; done

RSE_IP=$(aws ec2 describe-route-server-endpoints --route-server-endpoint-ids $RSE_ID \
  --query 'RouteServerEndpoints[0].EniAddress' --output text)
echo "endpoint address: $RSE_IP"
```

`RSE_IP` is the neighbor TMM will peer with. AWS picks it from the subnet; it is
the one value you have to write into `bgp-route-server.yaml`.

## 3. Enable propagation into the route tables that should learn VIPs

```bash
aws ec2 enable-route-server-propagation --route-server-id $RS_ID --route-table-id $PUBLIC_RTB
aws ec2 enable-route-server-propagation --route-server-id $RS_ID --route-table-id $PRIVATE_RTB
aws ec2 get-route-server-propagations --route-server-id $RS_ID    # wait for "available"
```

The public table is what the jumphost and any public-subnet client use; the
private table is what worker nodes and private-subnet clients use. Add any
other table (a peered VPC's, a Transit Gateway attachment's) the same way.

## 4. Create the peer pointing at TMM's external self IP

```bash
aws ec2 create-route-server-peer --route-server-endpoint-id $RSE_ID \
  --peer-address $TMM_EXT_SELFIP \
  --bgp-options PeerAsn=65000,PeerLivenessDetection=bfd
```

The Route Server never initiates the session. `BgpStatus` stays `down` until
TMM's routing container dials it in the next step.

## 5. Apply the ZebOS ConfigMap

Edit the example's `bgp-route-server.yaml`: replace `10.0.10.31` (three
`neighbor` lines) with `$RSE_IP`. The router-id (`10.0.10.240`) only has to be
unique, so it can stay. Then:

```bash
export KUBECONFIG=.awsbnkctl/<cluster>/kubeconfig
kubectl apply -f examples/<example>/bgp-route-server.yaml
```

`kubectl apply` warns once that the ConfigMap (created by FLO) lacks the
last-applied annotation and patches it in; that is expected. The BFD watcher in
the routing container picks the new stanza up within about a minute — the
session was Established and BFD Up about 70 s after the apply on
`bnk-bgp-test`. To change the stanza later, edit the file and apply again; the
TMM chart preserves the ConfigMap content across upgrades.

## 6. Verify

```bash
# AWS side: session and BFD up, VIP learned (the CLI has no per-endpoint
# selector for peers; filter on RouteServerEndpointId)
aws ec2 describe-route-server-peers \
  --query "RouteServerPeers[?RouteServerEndpointId=='$RSE_ID'].[RouteServerPeerId,State,BgpStatus.Status,BfdStatus.Status]" --output text
aws ec2 get-route-server-routing-database --route-server-id $RS_ID
aws ec2 describe-route-tables --route-table-ids $PRIVATE_RTB \
  --query 'RouteTables[0].Routes[?Origin==`Advertisement`]'

# TMM side: stanza loaded, neighbor Established, BFD Up, VIP in the BGP table
TMM=$(kubectl -n f5-cne-system get pod -l app=f5-tmm -o jsonpath='{.items[0].metadata.name}')
kubectl -n f5-cne-system exec $TMM -c f5-tmm-routing -- imish -e 'show running-config'
kubectl -n f5-cne-system exec $TMM -c f5-tmm-routing -- imish -e 'show ip bgp summary'
kubectl -n f5-cne-system exec $TMM -c f5-tmm-routing -- imish -e 'show bfd session'
kubectl -n f5-cne-system exec $TMM -c f5-tmm-routing -- imish -e 'show ip bgp'
```

Expect a `/32` for the Gateway VIP (10.0.10.100 in the examples) with the TMM
external ENI as its target in every propagated route table (`Origin:
Advertisement`), and the same prefix `in-fib` in the routing database with
`NextHopIp` = `TMM_EXT_SELFIP`. The VIP is advertised only while a Gateway that
owns it exists — deploy one (the `http-routing-e2e` scenario does) before
looking for the route. A client on another subnet (a worker node, a pod)
reaches the VIP through that route; note the cne-controller also puts the VIP
on the external ENI as a secondary IP, so in-VPC clients would reach it without
BGP — the route is what BGP adds for tables the controller does not touch.

## Teardown (before `awsbnkctl down`)

```bash
# Put the ConfigMap back to what FLO created (an empty ZebOS.conf). The BFD
# watcher only adds configuration, so the running stanza stays until the TMM
# pod restarts; deleting the peer below is what drops the session, and `down`
# deletes the cluster anyway.
kubectl -n f5-cne-system patch configmap f5-tmm-dynamic-routing-template \
  --type merge -p '{"data":{"ZebOS.conf":""}}'
for p in $(aws ec2 describe-route-server-peers \
    --query "RouteServerPeers[?RouteServerEndpointId=='$RSE_ID'].RouteServerPeerId" --output text); do
  aws ec2 delete-route-server-peer --route-server-peer-id $p
done
aws ec2 delete-route-server-endpoint --route-server-endpoint-id $RSE_ID
for rtb in $PUBLIC_RTB $PRIVATE_RTB; do
  aws ec2 disable-route-server-propagation --route-server-id $RS_ID --route-table-id $rtb
done
aws ec2 disassociate-route-server --route-server-id $RS_ID --vpc-id $VPC_ID
aws ec2 delete-route-server --route-server-id $RS_ID
```

Wait for the endpoint to reach `deleted` before running `awsbnkctl down`,
otherwise phase 03 fails deleting the external subnet with `DependencyViolation`.

## References

- [Amazon VPC Route Server tutorial](https://docs.aws.amazon.com/vpc/latest/userguide/route-server-tutorial.html)
- [`create-route-server-peer`](https://docs.aws.amazon.com/cli/latest/reference/ec2/create-route-server-peer.html)
- [BNK: Set up dynamic routing with BGP (ZebOS ConfigMap)](https://clouddocs.f5.com/bigip-next-for-kubernetes/latest/how-tos/spk-zebos-config.html)
- [BNK 2.4: OcNOS routing software](https://clouddocs.f5.com/bigip-next-for-kubernetes/latest/how-tos/border-gateway-protocol/OcNOS-routing-software.html)
- [BNK 2.4: BGP routing with Kubernetes CRDs (OcNOS path)](https://clouddocs.f5.com/bigip-next-for-kubernetes/latest/how-tos/border-gateway-protocol/bnk-bgp-Introduction.html)
- [BNK 2.4: sample YAML files by use case](https://clouddocs.f5.com/bigip-next-for-kubernetes/latest/how-tos/border-gateway-protocol/sample-files-by-use-case.html)
- [BNK 2.4: GlobalRoutingConfig reference](https://clouddocs.f5.com/bigip-next-for-kubernetes/latest/how-tos/border-gateway-protocol/bnk-crd-reference-bgp-routing.html)
