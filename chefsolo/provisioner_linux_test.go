package chefsolo

import (
	"path"
	"testing"

	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/spf13/afero"
)

/*
	test pre upload directory :
		- sudo
 		- no_sudo
*/

func TestResourceProvider_preUploadDirectory(t *testing.T) {
	directory := "toto"
	cases := map[string]struct {
		Config   map[string]interface{}
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

			Commands: map[string]bool{
				"sudo bash -c 'mkdir -p " + directory + "'":     true,
				"sudo bash -c 'chmod -R 777 " + directory + "'": true,
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

			Commands: map[string]bool{
				"mkdir -p " + directory + "":     true,
				"chmod -R 777 " + directory + "": true,
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
		p.DefaultConfDir = linuxConfDir

		if err = p.preUploadDirectory(o, c, directory); err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}
func TestResourceProvider_postUploadDirectory(t *testing.T) {
	directory := "toto"
	cases := map[string]struct {
		Config   map[string]interface{}
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

			Commands: map[string]bool{
				"sudo bash -c 'chmod -R 755 " + directory + "'":                                          true,
				"sudo bash -c 'find " + directory + " -maxdepth 1 -type f -exec /bin/chmod -R 600 {} +'": true,
				"sudo bash -c 'chown -R root.root " + directory + "'":                                    true,
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

			Commands: map[string]bool{},
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
		p.DefaultConfDir = linuxConfDir

		if err = p.postUploadDirectory(o, c, directory); err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestResourceProvider_uploadConfigFiles(t *testing.T) {
	cases := map[string]struct {
		Config     map[string]interface{}
		Commands   map[string]bool
		Uploads    map[string]string
		UploadDirs map[string]string
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

			Commands: map[string]bool{
				"sudo bash -c 'mkdir -p " + linuxConfDir + "'":                                              true,
				"sudo bash -c 'chmod -R 777 " + linuxConfDir + "'":                                          true,
				"sudo bash -c 'chmod -R 755 " + linuxConfDir + "'":                                          true,
				"sudo bash -c 'find " + linuxConfDir + " -maxdepth 1 -type f -exec /bin/chmod -R 600 {} +'": true,
				"sudo bash -c 'chown -R root.root " + linuxConfDir + "'":                                    true,
			},
			Uploads: map[string]string{
				path.Join(linuxConfDir, "client.rb"): defaultLinuxClientConf,
			},
			UploadDirs: map[string]string{
				"/output":     linuxConfDir,
				"/custom_dir": path.Join(linuxConfDir, "output"),
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
			Commands: map[string]bool{
				"mkdir -p " + linuxConfDir + "":     true,
				"chmod -R 777 " + linuxConfDir + "": true,
			},
			Uploads: map[string]string{
				path.Join(linuxConfDir, "client.rb"): defaultLinuxClientConf,
			},
			UploadDirs: map[string]string{
				"/output":     linuxConfDir,
				"/custom_dir": path.Join(linuxConfDir, "output"),
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands
		c.Uploads = tc.Uploads
		c.UploadDirs = tc.UploadDirs
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
		p.DefaultConfDir = linuxConfDir
		p.osUploadConfigFiles = p.linuxUploadConfigFiles

		if err = p.osUploadConfigFiles(o, c); err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

/*
	test installChefAsService :
	- sudo
	- no_sudo
*/

func TestResourceProvider_installChefAsService(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
		Uploads  map[string]string
		Error    bool
	}{
		"Sudo": {
			Config: map[string]interface{}{
				"instance_id":        `toto`,
				"chef_module_path":   `/input`,
				"output_dir":         `/output`,
				"nodes":              []string{`{ "id":"toto"}`},
				"target_node":        `{ "id":"toto"}`,
				"use_sudo":           true,
				"run_list":           []interface{}{"cookbook::recipe"},
				"install_as_service": true,
			},

			Commands: map[string]bool{
				"sudo bash -c 'find /tmp/chef-run.service -maxdepth 1 -type f -exec /bin/chmod -R 755 {} +'": true,
				"sudo bash -c 'mv /tmp/chef-run.service /etc/systemd/system/chef-run.service'":               true,
				"sudo bash -c 'systemctl daemon-reload'":                                                     true,
				"sudo bash -c 'systemctl enable chef-run.service'":                                           true,
			},
			Uploads: map[string]string{
				path.Join("/tmp", serviceName): defaultChefService,
			},
			Error: false,
		},

		"NoSudo": {
			Config: map[string]interface{}{
				"instance_id":        `toto`,
				"chef_module_path":   `/input`,
				"output_dir":         `/output`,
				"nodes":              []string{`{ "id":"toto"}`},
				"target_node":        `{ "id":"toto"}`,
				"use_sudo":           false,
				"run_list":           []interface{}{"cookbook::recipe"},
				"install_as_service": true,
			},
			Commands: map[string]bool{},
			Uploads:  map[string]string{},
			Error:    true,
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands
		c.Uploads = tc.Uploads
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
		p.DefaultConfDir = linuxConfDir
		p.osUploadConfigFiles = p.linuxUploadConfigFiles

		cmd := fmt.Sprintf(`%s -z -c %s -j %q -E %q`,
			linuxChefCmd,
			path.Join(linuxConfDir, clienrb),
			path.Join(linuxConfDir, "output", "dna", "toto.json"),
			defaultEnv)

		if err = p.linuxInstallChefAsAService(o, c, cmd); err != nil && !tc.Error {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestResourceProvider_linuxInstallChefClient(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
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

			Commands: map[string]bool{
				"sudo bash -c 'curl -LO https://omnitruck.chef.io/install.sh'": true,
				"sudo bash -c 'bash ./install.sh -v \"\" -c stable'":           true,
				"sudo bash -c 'rm -f install.sh'":                              true,
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

			Commands: map[string]bool{
				"curl -LO https://omnitruck.chef.io/install.sh": true,
				"bash ./install.sh -v \"\" -c stable":           true,
				"rm -f install.sh":                              true,
			},
		},

		"HTTPProxy": {
			Config: map[string]interface{}{
				"http_proxy":       "http://proxy.local",
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			Commands: map[string]bool{
				"http_proxy='http://proxy.local' curl -LO https://omnitruck.chef.io/install.sh": true,
				"http_proxy='http://proxy.local' bash ./install.sh -v \"\" -c stable":           true,
				"http_proxy='http://proxy.local' rm -f install.sh":                              true,
			},
		},

		"HTTPSProxy": {
			Config: map[string]interface{}{
				"https_proxy":      "https://proxy.local",
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			Commands: map[string]bool{
				"https_proxy='https://proxy.local' curl -LO https://omnitruck.chef.io/install.sh": true,
				"https_proxy='https://proxy.local' bash ./install.sh -v \"\" -c stable":           true,
				"https_proxy='https://proxy.local' rm -f install.sh":                              true,
			},
		},

		"NoProxy": {
			Config: map[string]interface{}{
				"http_proxy":       "http://proxy.local",
				"no_proxy":         []interface{}{"http://local.local", "http://local.org"},
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"run_list":         []interface{}{"cookbook::recipe"},
			},

			Commands: map[string]bool{
				"http_proxy='http://proxy.local' no_proxy='http://local.local,http://local.org' " +
					"curl -LO https://omnitruck.chef.io/install.sh": true,
				"http_proxy='http://proxy.local' no_proxy='http://local.local,http://local.org' " +
					"bash ./install.sh -v \"\" -c stable": true,
				"http_proxy='http://proxy.local' no_proxy='http://local.local,http://local.org' " +
					"rm -f install.sh": true,
			},
		},

		"Version": {
			Config: map[string]interface{}{
				"instance_id":      `toto`,
				"chef_module_path": `/input`,
				"output_dir":       `/output`,
				"nodes":            []string{`{ "id":"toto"}`},
				"target_node":      `{ "id":"toto"}`,
				"use_sudo":         false,
				"run_list":         []interface{}{"cookbook::recipe"},
				"version":          "11.18.6",
			},

			Commands: map[string]bool{
				"curl -LO https://omnitruck.chef.io/install.sh": true,
				"bash ./install.sh -v \"11.18.6\" -c stable":    true,
				"rm -f install.sh":                              true,
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

		err = p.linuxInstallChefClient(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

const defaultLinuxClientConf = `log_location            STDOUT


local_mode true

cookbook_path '/opt/chef/0/output/cookbooks'

node_path '/opt/chef/0/output/nodes'
role_path '/opt/chef/0/output/roles'
data_bag_path '/opt/chef/0/output/data_bags'
rubygems_url 'http://nexus.query.consul/content/groups/rubygems'
environment_path '/opt/chef/0/output/environments'`

const defaultChefService = `
[Unit]
Description=Run chef client each time the machine reboot
After = network.target auditd.service

[Service]
Type=simple
WorkingDirectory=/opt/chef/0/output
ExecStart=/opt/chef/embedded/bin/ruby /usr/bin/chef-client -z -c /opt/chef/0/client.rb -j "/opt/chef/0/output/dna/toto.json" -E "_default"
ExecReload = /bin/kill -HUP $MAINPID
SuccessExitStatus = 3
Restart = on-failure

[Install]
WantedBy = multi-user.target
`
