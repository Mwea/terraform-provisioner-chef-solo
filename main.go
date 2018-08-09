package main

import (
	"github.com/Mwea/terraform-provisioner-chef-solo/chef-solo"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProvisionerFunc: chef_solo.Provisioner,
	})
}
