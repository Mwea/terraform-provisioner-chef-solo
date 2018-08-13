package main

import (
	"github.com/Mwea/terraform-provisioner-chefsolo/chefsolo"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProvisionerFunc: chefsolo.Provisioner,
	})
}
