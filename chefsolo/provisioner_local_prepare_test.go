package chefsolo

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
	"path/filepath"
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
	os := afero.NewOsFs()

	o := new(terraform.MockUIOutput)
	for k, tc := range cases {

		dir, err := ioutil.TempDir("", "tf-test")

		tc.Config["chef_module_path"] = filepath.Join(dir, tc.Config["chef_module_path"].(string))
		tc.Config["output_dir"] = filepath.Join(dir, tc.Config["output_dir"].(string))

		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("Failed to remove directory, %v", err)
		}
		os.MkdirAll(filepath.Join(dir, "/tmp/tf/input"), 0777)
		os.MkdirAll(filepath.Join(dir, "/tmp/tf/output"), 0777)
		var lock *flock.Flock

		p, err := configureProvisioner(
			schema.TestResourceDataRaw(t, Provisioner().(*schema.Provisioner).Schema, tc.Config),
			os,
		)

		if tc.NodeDirExist {
			os.MkdirAll(filepath.Join(dir, "/tmp/tf/output/nodes"), 0777)
			os.Create(filepath.Join(dir, "/tmp/tf/output/nodes/popo.json"))
			if tc.Locked {
				lock = flock.NewFlock(path.Join(dir, "/tmp/tf", "output", "nodes", "nodes.lock"))
				lock.TryLock()
			}
		}

		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !tc.Locked {
			lock, err = p.buildNodeFiles(o)
		}

		lock.Unlock()
		if (tc.Error == false && err != nil) || (tc.Error == true && err == nil) {
			os.RemoveAll(dir)
			t.Fatalf("Test %q failed: %v", k, err)
		}

		if tc.NodeDirExist {
			tc.FilesProduced = append(tc.FilesProduced, "/tmp/tf/output/nodes/popo.json")
			tc.FilesProduced = append(tc.FilesProduced, "/tmp/tf/output/nodes/nodes.lock")
			files, err := ioutil.ReadDir(filepath.Join(dir, "/tmp/tf/output/nodes/"))
			if err != nil {
				log.Fatal(err)
			}

			for _, f := range files {
				if !contains(tc.FilesProduced, "/tmp/tf/output/nodes/"+f.Name()) {
					os.RemoveAll(dir)
					t.Fatalf("Test %q failed: file is missing %s", k, f.Name())
				}
			}
		}
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
		os.MkdirAll("/tmp/tf/input", 766)
		os.MkdirAll("/tmp/tf/output", 766)

		p, err := configureProvisioner(
			schema.TestResourceDataRaw(t, Provisioner().(*schema.Provisioner).Schema, tc.Config),
			os,
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if tc.Locked {
			flock.NewFlock(path.Join("/tmp/tf", "output", "chefsolo.lock")).TryLock()
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
