package main

import (
	"github.com/mougams/terraform-provisioner-chef-solo/chefsolo"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProvisionerFunc: chefsolo.Provisioner,
	})
}
