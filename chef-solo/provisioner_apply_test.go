package chef_solo

import (
	"fmt"
	"path"
	"testing"

	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/spf13/afero"
)

func TestResourceProvisioner_impl(t *testing.T) {
	var _ = Provisioner()
}

func TestProvisioner(t *testing.T) {
	if err := Provisioner().(*schema.Provisioner).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestResourceProvider_runChefClient(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		ChefCmd  string
		ConfDir  string
		Commands map[string]bool
	}{
		"Sudo": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			ChefCmd: linuxChefCmd,

			ConfDir: "/",

			Commands: map[string]bool{
				fmt.Sprintf(`sudo bash -c 'cd %s && %s -z -c %s -j %q -E %q'`,
					path.Join("/", "output"),
					linuxChefCmd,
					path.Join("/", clienrb),
					path.Join("/", "output", "dna", "toto.json"),
					defaultEnv): true,
			},
		},
		"NoSudo": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			ChefCmd: linuxChefCmd,

			ConfDir: "/",

			Commands: map[string]bool{
				fmt.Sprintf(`cd %s && %s -z -c %s -j %q -E %q`,
					path.Join("/", "output"),
					linuxChefCmd,
					path.Join("/", clienrb),
					path.Join("/", "output", "dna", "toto.json"),
					defaultEnv): true,
			},
		},
		"Environment": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"run_list":         []interface{}{"cookbook::recipe"},
				"environment":      "tototo",
			},

			ChefCmd: linuxChefCmd,

			ConfDir: "/",

			Commands: map[string]bool{
				fmt.Sprintf(`cd %s && %s -z -c %s -j %q -E %q`,
					path.Join("/", "output"),
					linuxChefCmd,
					path.Join("/", clienrb),
					path.Join("/", "output", "dna", "toto.json"),
					"tototo"): true,
			},
		},
		"NamedRunList": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"policy_name":      "mesos",
				"policy_group":     "local",
				"use_policyfile":   true,
				"named_run_list":   "tototo",
			},

			ChefCmd: linuxChefCmd,

			ConfDir: "/",

			Commands: map[string]bool{
				fmt.Sprintf(`cd %s && %s -z -c %s -j %q -n %q`,
					path.Join("/", "output"),
					linuxChefCmd,
					path.Join("/", clienrb),
					path.Join("/", "output", "dna", "toto.json"),
					"tototo"): true,
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands

		os := afero.NewMemMapFs()
		os.MkdirAll("/input", 766)
		os.MkdirAll("/output", 766)
		p, err := configureProvisioner(
			schema.TestResourceDataRaw(t, Provisioner().(*schema.Provisioner).Schema, tc.Config),
			os,
		)

		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		p.runChefClient = p.runChefClientFunc(tc.ChefCmd, tc.ConfDir)

		err = p.runChefClient(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}
