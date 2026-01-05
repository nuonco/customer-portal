variable "region" {
  description = "AWS region"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where the cluster will be created"
  type        = string
}

variable "nuon_id" {
  description = "Nuon install ID for resource naming"
  type        = string
}
