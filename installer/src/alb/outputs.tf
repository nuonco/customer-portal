output "target_group_arn" {
  description = "ARN of the target group"
  value       = aws_lb_target_group.main.arn
}

output "security_group_id" {
  description = "Security group ID for the ALB"
  value       = aws_security_group.alb.id
}

output "dns_name" {
  description = "DNS name of the ALB"
  value       = aws_lb.main.dns_name
}

output "zone_id" {
  description = "Zone ID of the ALB"
  value       = aws_lb.main.zone_id
}

output "url" {
  description = "HTTPS URL for the ALB"
  value       = "https://${var.domain_name}"
}
