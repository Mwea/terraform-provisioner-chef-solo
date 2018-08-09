package chef_solo

import (
	"path"
	"testing"

	"context"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/spf13/afero"
	"github.com/theckman/go-flock"
	"io/ioutil"
	"log"
)

func TestResourceProvider_buildNodeFiles(t *testing.T) {
	cases := map[string]struct {
		Config        map[string]interface{}
		NodeDirExist  bool
		Locked        bool
		Error         bool
		FilesProduced []string
	}{
		"Node dir does not exist": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/tmp/tf/input`,
				"output_dir":       `/tmp/tf/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			NodeDirExist: false,

			Locked: false,

			Error: false,

			FilesProduced: []string{
				"/tmp/tf/output/nodes/toto.json",
			},
		},
		"Node dir does exist": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/tmp/tf/input`,
				"output_dir":       `/tmp/tf/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			NodeDirExist: true,

			Locked: false,

			Error: false,

			FilesProduced: []string{
				"/tmp/tf/output/nodes/toto.json",
			},
		},
		"Node dir does exist but locked": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/tmp/tf/input`,
				"output_dir":       `/tmp/tf/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			NodeDirExist: true,

			Locked: true,

			Error: false,

			FilesProduced: []string{},
		},
	}

	o := new(terraform.MockUIOutput)
	for k, tc := range cases {
		os := afero.NewOsFs()
		os.RemoveAll("/tmp/tf")
		os.MkdirAll("/tmp/tf/input", 755)
		os.MkdirAll("/tmp/tf/output", 755)
		var lock *flock.Flock

		if tc.NodeDirExist {
			os.MkdirAll("/tmp/tf/output/nodes", 755)
			os.Create("/tmp/tf/output/nodes/popo.json")
			if tc.Locked {
				lock = flock.NewFlock(path.Join("/tmp/tf", "output", "nodes", "nodes.lock"))
				lock.TryLock()
			}
		}

		p, err := configureProvisioner(
			schema.TestResourceDataRaw(t, Provisioner().(*schema.Provisioner).Schema, tc.Config),
			os,
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		err = p.buildNodeFiles(o)
		if (tc.Error == false && err != nil) || (tc.Error == true && err == nil) {
			os.RemoveAll("/tmp/tf")
			t.Fatalf("Test %q failed: %v", k, err)
		}

		if tc.NodeDirExist {
			tc.FilesProduced = append(tc.FilesProduced, "/tmp/tf/output/nodes/toto.json")
			tc.FilesProduced = append(tc.FilesProduced, "/tmp/tf/output/nodes/nodes.lock")
			files, err := ioutil.ReadDir("/tmp/tf/output/nodes/")
			if err != nil {
				log.Fatal(err)
			}

			for _, f := range files {
				if !contains(tc.FilesProduced, "/tmp/tf/output/nodes/"+f.Name()) {
					os.RemoveAll("/tmp/tf")
					t.Fatalf("Test %q failed: file is missing %s", k, f.Name())
				}
			}
		}
		os.RemoveAll("/tmp/tf")
	}
}

func TestResourceProvider_renderChefData(t *testing.T) {
	cases := map[string]struct {
		Config        map[string]interface{}
		BundleEnd     bool
		Locked        bool
		Error         bool
		FilesProduced []string
	}{
		"lock exist and bundle does not end": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/tmp/tf/input`,
				"output_dir":       `/tmp/tf/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},
			Locked:    true,
			BundleEnd: false,
			Error:     true,
		},
		"lock exist and bundle does end": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/tmp/tf/input`,
				"output_dir":       `/tmp/tf/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         true,
				"run_list":         []interface{}{"cookbook::recipe"},
			},
			Locked:    true,
			BundleEnd: true,
			Error:     false,
		},
	}

	o := new(terraform.MockUIOutput)
	for k, tc := range cases {
		os := afero.NewOsFs()
		os.RemoveAll("/tmp/tf")
		os.MkdirAll("/tmp/tf/input", 755)
		os.MkdirAll("/tmp/tf/output", 755)

		p, err := configureProvisioner(
			schema.TestResourceDataRaw(t, Provisioner().(*schema.Provisioner).Schema, tc.Config),
			os,
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if tc.Locked {
			flock.NewFlock(path.Join("/tmp/tf", "output", "chef-solo.lock")).TryLock()
		}
		if tc.BundleEnd {
			os.Create(path.Join("/tmp/tf", "output", "bundle-done"))
		}
		err = p.renderChefData(context.Background(), o)

		if (tc.Error == false && err != nil) || (tc.Error == true && err == nil) {
			os.RemoveAll("/tmp/tf")
			t.Fatalf("Test %q failed: %v", k, err)
		}
		os.RemoveAll("/tmp/tf/*")
	}
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}
