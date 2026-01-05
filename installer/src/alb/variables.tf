variable "region" {
  description = "AWS region"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where ALB will be created"
  type        = string
}

variable "nuon_id" {
  description = "Nuon install ID for resource naming"
  type        = string
}

variable "certificate_arn" {
  description = "ARN of ACM certificate for HTTPS"
  type        = string
}

variable "zone_id" {
  description = "Route53 hosted zone ID"
  type        = string
}

variable "domain_name" {
  description = "Domain name for the ALB (e.g., vendor.install-id.nuon.run)"
  type        = string
}

variable "internal" {
  description = "Whether the ALB is internal (true) or internet-facing (false)"
  type        = bool
  default     = false
}

variable "container_port" {
  description = "Port the container listens on"
  type        = number
}

variable "alb_name" {
  description = "Name prefix for ALB resources (e.g., vendor, customer)"
  type        = string
}

variable "block_admin_paths" {
  description = "Whether to block /admin paths (true for customer ALB to prevent vendor access)"
  type        = bool
  default     = false
}
