package chef_solo

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/spf13/afero"
	"path"
)

const (
	clienrb        = "client.rb"
	defaultEnv     = "_default"
	logfileDir     = "logfiles"
	linuxChefCmd   = "/opt/chef/embedded/bin/ruby --disable-gems /usr/bin/chef-client"
	linuxConfDir   = "/opt/chef/0"
	windowsChefCmd = "cmd /c chef-client"
	windowsConfDir = "C:/chef"
	maxBufSize     = 8 * 1024
)

const clientConf = `
log_location            STDOUT
{{ if .HTTPProxy }}
http_proxy          "{{ .HTTPProxy }}"
ENV['http_proxy'] = "{{ .HTTPProxy }}"
ENV['HTTP_PROXY'] = "{{ .HTTPProxy }}"
{{ end -}}

{{ if .HTTPSProxy }}
https_proxy          "{{ .HTTPSProxy }}"
ENV['https_proxy'] = "{{ .HTTPSProxy }}"
ENV['HTTPS_PROXY'] = "{{ .HTTPSProxy }}"
{{ end -}}

{{ if .NOProxy }}
no_proxy          "{{ join .NOProxy "," }}"
ENV['no_proxy'] = "{{ join .NOProxy "," }}"
{{ end -}}

{{ if .SSLVerifyMode }}
ssl_verify_mode  {{ .SSLVerifyMode }}
{{- end -}}

{{ if .DisableReporting }}
enable_reporting false
{{ end -}}

{{ if .ClientOptions }}
{{ join .ClientOptions "\n" }}
{{ end }}

local_mode true
{{ if not .UsePolicyfile }}
cookbook_path '{{ .DefaultConfDir }}/{{ .BaseOutputDir }}/cookbooks'
{{ end }}
node_path '{{ .DefaultConfDir }}/{{ .BaseOutputDir }}/nodes'
role_path '{{ .DefaultConfDir }}/{{ .BaseOutputDir }}/roles'
data_bag_path '{{ .DefaultConfDir }}/{{ .BaseOutputDir }}/data_bags'
rubygems_url 'http://nexus.query.consul/content/groups/rubygems'
environment_path '{{ .DefaultConfDir }}/{{ .BaseOutputDir }}/environments'
`

type provisionFn func(terraform.UIOutput, communicator.Communicator) error
type installFn func(terraform.UIOutput, communicator.Communicator, string) error

func applyFn(ctx context.Context) error {
	o := ctx.Value(schema.ProvOutputKey).(terraform.UIOutput)
	s := ctx.Value(schema.ProvRawStateKey).(*terraform.InstanceState)
	d := ctx.Value(schema.ProvConfigDataKey).(*schema.ResourceData)

	// Decode the provisioner config
	p, err := configureProvisioner(d, afero.NewOsFs())
	if err != nil {
		return err
	}

	if err := p.configurePerOS(s); err != nil {
		return err
	}

	comm, err := getCommunicator(ctx, o, s)
	if err != nil {
		return err
	}

	o.Output("Creating configuration files...")
	if err := p.prepareConfigFiles(ctx, o, comm, p.DefaultConfDir); err != nil {
		return err
	}

	o.Output("Preparing the machine...")
	if err := p.prepareMachine(o, comm, p.DefaultConfDir); err != nil {
		return err
	}

	o.Output("Starting initial Chef-Client run...")
	if err := p.runChefClient(o, comm); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) runChefClientFunc(chefCmd string, confDir string) provisionFn {
	return func(o terraform.UIOutput, comm communicator.Communicator) error {
		var cmd = fmt.Sprintf("%s -z -c %s -j %q",
			chefCmd,
			path.Join(confDir, clienrb),
			path.Join(confDir, p.BaseOutputDir, "dna", p.InstanceId+".json"))

		switch {
		case p.UsePolicyfile && p.NamedRunList == "":
			cmd = fmt.Sprintf(cmd)
		case p.UsePolicyfile && p.NamedRunList != "":
			cmd = fmt.Sprintf("%s -n %q", cmd, p.NamedRunList)
		default:
			cmd = fmt.Sprintf("%s -E %q", cmd, p.Environment)
		}
		if p.installAsService {
			if err := p.installService(o, comm, cmd); err != nil {
				return err
			}
		}
		return p.runRemote(o, comm, fmt.Sprintf("cd %s && %s", path.Join(confDir, p.BaseOutputDir), cmd))
	}
}
