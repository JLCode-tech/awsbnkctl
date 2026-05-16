# ============================================================
# testing — outputs (Sprint 3 AWS retarget)
# ============================================================

output "testing_jumphost_shared_public_key" {
  description = "Public key installed on all jumphosts (operators add to ~/.ssh/authorized_keys to log in)"
  value       = trimspace(tls_private_key.jumphost_shared_key.public_key_openssh)
}

output "testing_jumphost_shared_private_key" {
  description = "Private key shared across all jumphosts. Write to a local file (chmod 600) to SSH between hosts"
  value       = tls_private_key.jumphost_shared_key.private_key_openssh
  sensitive   = true
}

output "testing_cluster_jumphost_public_ips" {
  description = "Map of subnet id to Elastic IP for cluster jumphosts"
  value       = { for s, eip in aws_eip.cluster_jumphost_eip : s => eip.public_ip }
}

output "testing_cluster_jumphost_private_ips" {
  description = "Map of subnet id to private IP for cluster jumphosts"
  value       = { for s, inst in aws_instance.cluster_jumphost : s => inst.private_ip }
}

output "testing_cluster_jumphost_ssh_commands" {
  description = "Map of subnet id to SSH command for cluster jumphosts"
  value = {
    for s, eip in aws_eip.cluster_jumphost_eip :
    s => (var.testing_ssh_key_name != "" ? "ssh -i <pem-for-${var.testing_ssh_key_name}> ubuntu@${eip.public_ip}" : "ssh ubuntu@${eip.public_ip}")
  }
}
