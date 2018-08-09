package chef_solo

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/spf13/afero"
	"testing"
)

/* test :
- quand un des nodes est pas un json valide, ça pète
- quand target_node est pas un json valide, ça pète
- quand chef_module_path n'existe pas, ça pète
*/

func TestResourceProvider_DecodeConfig(t *testing.T) {
	cases := map[string]struct {
		Config map[string]interface{}
	}{
		"Node Non-valid": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`sdsd{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},
		},
		"Target-Node Non-valid": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `dsds{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},
		},
		"Module Path does not exist": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/blabla`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},
		},
	}
	for k, tc := range cases {
		os := afero.NewMemMapFs()
		os.MkdirAll("/input", 755)
		os.MkdirAll("/output", 755)
		_, err := configureProvisioner(
			schema.TestResourceDataRaw(t, Provisioner().(*schema.Provisioner).Schema, tc.Config),
			os,
		)
		if err == nil {
			t.Fatalf("Test %q failed: %v", k, "Error should have been triggered")
		}
	}

}
