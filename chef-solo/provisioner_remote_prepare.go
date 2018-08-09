package chef_solo

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
	"os"
	"path"
	"strings"
	"text/template"
)

func (p *provisioner) prepareMachine(o terraform.UIOutput, comm communicator.Communicator, confDir string) error {
	o.Output("Uploading config files")
	if err := p.osUploadConfigFiles(o, comm); err != nil {
		return err
	}

	if !p.SkipInstall {
		o.Output("Installing chef client")
		if err := p.installChefClient(o, comm); err != nil {
			return err
		}
	}
	return nil
}

func (p *provisioner) uploadClientConf(comm communicator.Communicator, confDir string) error {
	// Make strings.Join available for use within the template
	funcMap := template.FuncMap{
		"join": strings.Join,
	}

	// Create a new template and parse the client config into it
	t := template.Must(template.New(clienrb).Funcs(funcMap).Parse(clientConf))

	var buf bytes.Buffer
	err := t.Execute(&buf, p)
	if err != nil {
		return fmt.Errorf("error executing %s template: %s", clienrb, err)
	}

	// Copy the client config to the new instance
	if err = comm.Upload(path.Join(confDir, clienrb), &buf); err != nil {
		return fmt.Errorf("uploading %s failed: %v", clienrb, err)
	}
	return nil
}

func (p *provisioner) uploadDirectory(o terraform.UIOutput, comm communicator.Communicator, src, confDir string) error {
	_, err := p.os.Stat(src)
	if os.IsNotExist(err) || err != nil {
		o.Output("Warning: " + src + " does not exist, uploading nothing.")
		return nil
	}

	if err := comm.UploadDir(confDir, src); err != nil {
		return fmt.Errorf("uploading %s failed: %v", src, err)
	}

	return nil
}
