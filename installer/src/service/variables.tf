variable "region" {
  description = "AWS region"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where service will be created"
  type        = string
}

variable "nuon_id" {
  description = "Nuon install ID for resource naming"
  type        = string
}

variable "cluster_arn" {
  description = "ARN of the ECS cluster"
  type        = string
}

variable "image_url" {
  description = "Container image URL (without tag)"
  type        = string
}

variable "image_tag" {
  description = "Container image tag"
  type        = string
}

variable "desired_count" {
  description = "Number of tasks to run"
  type        = number
  default     = 2
}

# Vendor ALB (routes vendor.domain.com to port 8080)
variable "vendor_target_group_arn" {
  description = "ARN of the vendor ALB target group"
  type        = string
}

variable "vendor_security_group_id" {
  description = "Security group ID of the vendor ALB"
  type        = string
}

# Customer ALB (routes customer.domain.com to port 8080)
variable "customer_target_group_arn" {
  description = "ARN of the customer ALB target group"
  type        = string
}

variable "customer_security_group_id" {
  description = "Security group ID of the customer ALB"
  type        = string
}

# Application environment variables
variable "jwt_secret" {
  description = "JWT secret for authentication"
  type        = string
  sensitive   = true
}

variable "database_url" {
  description = "PostgreSQL connection URL"
  type        = string
  sensitive   = true
}

variable "customer_base_url" {
  description = "Base URL for the customer portal (public)"
  type        = string
}
