output "endpoint" {
  description = "RDS endpoint (hostname:port)"
  value       = "${aws_db_instance.main.address}:${aws_db_instance.main.port}"
}

output "address" {
  description = "RDS hostname"
  value       = aws_db_instance.main.address
}

output "port" {
  description = "RDS port"
  value       = aws_db_instance.main.port
}

output "security_group_id" {
  description = "Security group ID for RDS"
  value       = aws_security_group.rds.id
}
