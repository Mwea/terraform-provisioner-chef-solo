package chef_solo

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
	"path"
	"strings"
	"text/template"
)

const (
	chmod         = "find %s -maxdepth 1 -type f -exec /bin/chmod -R %d {} +"
	installURL    = "https://omnitruck.chef.io/install.sh"
	reloadDeamon  = "systemctl daemon-reload"
	serviceName   = "chef-run.service"
	enableService = "systemctl enable %s"
	servicePath   = "/etc/systemd/system/"
)

const chefService = `
[Unit]
Description=Run chef client each time the machine reboot
After = network.target auditd.service

[Service]
Type=simple
WorkingDirectory={{ .ChefCookbookDirectory }}
ExecStart={{ .ChefCmd }}
ExecReload = /bin/kill -HUP $MAINPID
SuccessExitStatus = 3
Restart = on-failure

[Install]
WantedBy = multi-user.target
`

func (p *provisioner) linuxInstallChefClient(o terraform.UIOutput, comm communicator.Communicator) error {
	// Build up the command prefix
	prefix := ""
	if p.HTTPProxy != "" {
		prefix += fmt.Sprintf("http_proxy='%s' ", p.HTTPProxy)
	}
	if p.HTTPSProxy != "" {
		prefix += fmt.Sprintf("https_proxy='%s' ", p.HTTPSProxy)
	}
	if len(p.NOProxy) > 0 {
		prefix += fmt.Sprintf("no_proxy='%s' ", strings.Join(p.NOProxy, ","))
	}
	if err := p.runMultipleCommands(o, comm, []string{
		fmt.Sprintf("%scurl -LO %s", prefix, installURL),
		fmt.Sprintf("%sbash ./install.sh -v %q -c %s", prefix, p.Version, p.Channel),
		fmt.Sprintf("%srm -f install.sh", prefix),
	}); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) preUploadDirectory(o terraform.UIOutput, comm communicator.Communicator, dir string) error {
	// Make sure the config directory exists
	commands := []string{"mkdir -p " + dir}
	// Make sure we have enough rights to upload the files if using sudo
	if p.useSudo {
		commands = append(commands, "chmod -R 777 "+dir)
	}
	if err := p.runMultipleCommands(o, comm, commands); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) postUploadDirectory(o terraform.UIOutput, comm communicator.Communicator, dir string) error {
	if !p.useSudo {
		return nil
	}
	if err := p.runMultipleCommands(o, comm, []string{
		fmt.Sprintf("chmod -R 755 %s", dir),
		fmt.Sprintf(chmod, dir, 600),
		fmt.Sprintf("chown -R root.root %s", dir),
	}); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) linuxUploadConfigFiles(o terraform.UIOutput, comm communicator.Communicator) error {

	// Make sure we have enough rights to upload the files if using sudo
	if err := p.preUploadDirectory(o, comm, linuxConfDir); err != nil {
		return err
	}

	o.Output("Uploading client conf")
	if err := p.uploadClientConf(comm, linuxConfDir); err != nil {
		return err
	}

	configDir := path.Join(linuxConfDir, p.BaseOutputDir)

	o.Output("Deploying " + configDir)

	if err := p.uploadDirectory(o, comm, p.OutputDir, linuxConfDir); err != nil {
		return err
	}

	for _, resource := range p.Resources {
		if err := p.uploadDirectory(o, comm, resource.(string), path.Join(linuxConfDir, p.BaseOutputDir)); err != nil {
			return err
		}
	}

	if err := p.postUploadDirectory(o, comm, linuxConfDir); err != nil {
		return err
	}

	return nil
}

func (p *provisioner) linuxInstallChefAsAService(o terraform.UIOutput, comm communicator.Communicator,
	chefCmd string) error {

	if !p.useSudo {
		return fmt.Errorf("you need to use the option use_sudo to install chef as a service")
	}
	// evaluate tpl
	// Create a new template and parse the client config into it
	type ChefService struct {
		ChefCmd               string
		ChefCookbookDirectory string
	}

	chefStruct := ChefService{chefCmd, path.Join(linuxConfDir, p.BaseOutputDir)}
	t, _ := template.New(serviceName).Parse(chefService)

	var buf bytes.Buffer
	err := t.Execute(&buf, chefStruct)
	if err != nil {
		return fmt.Errorf("error executing %s template: %s", serviceName, err)
	}

	// Copy the client config to the new instance
	var service = path.Join("/", "tmp", serviceName)

	if err = comm.Upload(service, &buf); err != nil {
		return fmt.Errorf("uploading %s failed: %v", serviceName, err)
	}

	if err := p.runMultipleCommands(o, comm, []string{
		fmt.Sprintf(chmod, service, 755),
		fmt.Sprintf("mv %s %s", service, path.Join(servicePath, serviceName)),
		reloadDeamon,
		fmt.Sprintf(enableService, serviceName),
	}); err != nil {
		return err
	}
	return nil
}
