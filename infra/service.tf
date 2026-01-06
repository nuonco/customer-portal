module "service" {
  source = "../../../infra/modules/service"

  name      = local.name
  namespace = local.namespace
  env       = var.env
  additional_iam_policies = [
    aws_iam_policy.rds_iam_auth.arn
  ]
}
