module "certificate" {
  source = "../../../infra/modules/certificate"

  aws_region                = local.vars.region
  subdomain                 = local.vars.subdomain
  use_root_domain           = local.vars.use_root_domain
  env                       = var.env
  service                   = local.name
  subject_alternative_names = [local.vars.wildcard_domain]
}
