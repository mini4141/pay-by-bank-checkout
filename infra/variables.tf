variable "region" {
  type    = string
  default = "ap-southeast-2"
}

variable "name" {
  type    = string
  default = "pay-by-bank"
}

variable "deployer_cidr" {
  type        = string
  description = "Deployer public IP as /32; only this CIDR may reach the Temporal UI listener (:8233)."
}

variable "desired_count" {
  type        = number
  default     = 0
  description = "0 while images are being pushed and migrations run, then 1."
}

variable "enable_cloudfront" {
  type        = bool
  default     = false
  description = "Front the dashboard with CloudFront. Off by default: new AWS accounts must be verified by AWS Support before they can create distributions."
}
