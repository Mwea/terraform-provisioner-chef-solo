package chef_solo

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-homedir"
	"github.com/theckman/go-flock"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (p *provisioner) prepareConfigFiles(ctx context.Context, o terraform.UIOutput, comm communicator.Communicator, confDir string) error {
	if err := p.renderChefData(ctx, o); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) renderChefData(ctx context.Context, o terraform.UIOutput) error {
	fileLock := flock.NewFlock(path.Join(p.OutputDir, "chef-solo.lock"))
	if locked, err := fileLock.TryLock(); err == nil && locked {
		if err := p.bundleChef(ctx, o); err != nil {
			fileLock.Unlock()
			return err
		}
		if err := p.buildNodeFiles(o); err != nil {
			fileLock.Unlock()
			return err
		}

		p.os.Create(path.Join(p.OutputDir, "bundle-done"))
		fileLock.Unlock()
	} else {
		_, err := p.os.Stat(path.Join(p.OutputDir, "bundle-done"))
		ttw := 2 * time.Second
		for (os.IsNotExist(err) || err != nil) && ttw < 5*time.Second {
			time.Sleep(ttw)
			ttw *= 2
			_, err = p.os.Stat(path.Join(p.OutputDir, "bundle-done"))
		}
		if os.IsNotExist(err) {
			return fmt.Errorf("bundle seems stuck, stopping it")
		}
	}

	o.Output("Bundling dna file " + p.InstanceId)
	if err := p.buildDna(o); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) getBundleCommand() []string {
	chefPath, _ := homedir.Expand(p.ChefModulePath)
	prefix := "bundle exec"
	commands := []string{
		fmt.Sprintf("%s berks vendor -b=\"%s/Berksfile\" %s", prefix, chefPath, p.OutputDir),
	}
	if p.UsePolicyfile {
		commands = []string{
			fmt.Sprintf("%s chef install %s/Policyfile.rb", prefix, chefPath),
			fmt.Sprintf("%s chef export --force %s/Policyfile.rb %s", prefix, chefPath, p.OutputDir),
		}
	}
	return commands
}

func (p *provisioner) bundleChef(ctx context.Context, o terraform.UIOutput) error {
	for _, comm := range p.getBundleCommand() {
		if err := p.runLocal(ctx, o, comm); err != nil {
			return err
		}
	}
	return nil
}

func (p *provisioner) buildDna(o terraform.UIOutput) error {
	if err := p.os.MkdirAll(filepath.Join(p.OutputDir, "dna"), 0755); err != nil {
		return fmt.Errorf("error creating dna directory for output dir: %v", err)
	}
	tmp := make(map[string]interface{})
	if err := json.Unmarshal([]byte(p.TargetNode), &tmp); err != nil {
		return fmt.Errorf("error unable to render json %s: %v", p.TargetNode, err)
	}
	if err := p.bumpFile(path.Join(p.OutputDir, "dna", p.InstanceId+".json"), p.TargetNode, o); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) buildNodeFiles(o terraform.UIOutput) error {
	if err := p.os.MkdirAll(filepath.Join(p.OutputDir, "nodes"), 0755); err != nil {
		return fmt.Errorf("error creating node directory for output dir: %v", err)
	}
	lockedNode, err := flock.NewFlock(path.Join(p.OutputDir, "nodes", "nodes.lock")).TryLock()
	if lockedNode && err == nil {
		for _, node := range p.Nodes {
			tmp := make(map[string]interface{})
			if err := json.Unmarshal([]byte(node.(string)), &tmp); err != nil {
				return fmt.Errorf("error unable to render json %s: %v", node, err)
			}
			if err := p.bumpFile(path.Join(p.OutputDir, "nodes", tmp["id"].(string)+".json"), node.(string), o); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *provisioner) bumpFile(filePath string, data string, o terraform.UIOutput) error {
	nodePath, err := homedir.Expand(filePath)
	o.Output("Looking for " + nodePath + " existence")
	if _, err = p.os.Stat(nodePath); err == nil {
		o.Output("File already exist, not building it again")
		return nil
	}

	f, err := p.os.Create(path.Join(nodePath))
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", nodePath, err)
	}

	o.Output("Writing " + nodePath)
	_, err = f.Write([]byte(data))
	o.Output("File written " + nodePath)

	if err != nil {
		return fmt.Errorf("failed to write data %s to node file %s: %v", data, nodePath, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("error closing node file %s: %v", nodePath, err)
	}
	return nil
}
