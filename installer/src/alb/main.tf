# Application Load Balancer Module
# Creates ALB, target group, listener, and Route53 DNS record
# Supports both internet-facing (vendor) and internal (customer) configurations

data "aws_subnets" "selected" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }

  filter {
    name   = "tag:network.nuon.co/domain"
    values = var.internal ? ["*internal*"] : ["*public*"]
  }
}

data "aws_vpc" "main" {
  id = var.vpc_id
}

resource "aws_security_group" "alb" {
  name        = "${var.alb_name}-${var.nuon_id}"
  description = "Security group for ${var.alb_name} ALB"
  vpc_id      = var.vpc_id

  ingress {
    description = "HTTPS from ${var.internal ? "VPC" : "Internet"}"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = var.internal ? [data.aws_vpc.main.cidr_block] : ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTP from ${var.internal ? "VPC" : "Internet"}"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = var.internal ? [data.aws_vpc.main.cidr_block] : ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name   = "${var.alb_name}-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

resource "aws_lb" "main" {
  name               = "${var.alb_name}-${var.nuon_id}"
  internal           = var.internal
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = data.aws_subnets.selected.ids

  enable_deletion_protection = false

  tags = {
    Name   = "installer-${var.alb_name}-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

resource "aws_lb_target_group" "main" {
  name        = "${var.alb_name}-${var.nuon_id}"
  port        = var.container_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = {
    Name   = "${var.alb_name}-${var.nuon_id}"
    NuonID = var.nuon_id
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.main.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn

  # Default action: forward to target group
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main.arn
  }
}

# Block /admin paths for customer ALB (prevent vendor access via customer domain)
resource "aws_lb_listener_rule" "block_admin" {
  count        = var.block_admin_paths ? 1 : 0
  listener_arn = aws_lb_listener.https.arn
  priority     = 1

  action {
    type = "fixed-response"
    fixed_response {
      content_type = "text/plain"
      message_body = "Not Found"
      status_code  = "404"
    }
  }

  condition {
    path_pattern {
      values = ["/admin", "/admin/*"]
    }
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_route53_record" "main" {
  zone_id = var.zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_lb.main.dns_name
    zone_id                = aws_lb.main.zone_id
    evaluate_target_health = true
  }
}
