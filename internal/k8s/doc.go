// Package k8s wraps client-go for awsbnkctl's internal Kubernetes
// operations:
//
//   - kubeconfig loading (env / file / raw bytes)
//   - iperf3 test fixture lifecycle (deploy, wait-ready, wait-LB,
//     teardown)
//   - (v1.x) component log fetching, pod-readiness watch for status
//
// `awsbnkctl kubectl` shells to a local install and does not use this
// package — it's a convenience verb that just loads the workspace's
// KUBECONFIG before exec'ing.
//
// Kubeconfig source for v1: $KUBECONFIG env or ~/.kube/config. v1.x
// adds direct fetch from the AWS EKS SDK so users don't need to run
// `aws eks update-kubeconfig` themselves.
package k8s
