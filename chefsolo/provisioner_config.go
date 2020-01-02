package chefsolo

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/afero"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
)

type provisioner struct {
	Channel             string
	ClientOptions       []string
	Environment         string
	UsePolicyfile       bool
	HTTPProxy           string
	HTTPSProxy          string
	NamedRunList        string
	NOProxy             []string
	OSType              string
	InstanceId          string
	SkipInstall         bool
	SSLVerifyMode       string
	Version             string
	DefaultConfDir      string
	ChefModulePath      string
	OutputDir           string
	BaseOutputDir       string
	Nodes               []interface{}
	Resources           []interface{}
	TargetNode          string
	osUploadConfigFiles provisionFn
	installChefClient   provisionFn
	installService      installFn
	os                  afero.Fs

	runChefClient    provisionFn
	useSudo          bool
	installAsService bool
}

// Provisioner returns a Chef provisioner
func Provisioner() terraform.ResourceProvisioner {

	return &schema.Provisioner{
		Schema: map[string]*schema.Resource{
			"user_name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"user_key": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"channel": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "stable",
			},
			"client_options": {
				Type:     schema.TypeList,
				Elem:     &schema.Resource{Type: schema.TypeString},
				Optional: true,
			},
			"disable_reporting": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"environment": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  defaultEnv,
			},
			"fetch_chef_certificates": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"log_to_file": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"use_policyfile": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"http_proxy": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"https_proxy": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"no_proxy": {
				Type:     schema.TypeList,
				Elem:     &schema.Resource{Type: schema.TypeString},
				Optional: true,
			},
			"resources": {
				Type:     schema.TypeList,
				Elem:     &schema.Resource{Type: schema.TypeString},
				Optional: true,
			},
			"named_run_list": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"os_type": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"skip_install": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"prevent_sudo": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"secret_key": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"ssl_verify_mode": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"use_sudo": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"install_as_service": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"version": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"instance_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"chef_module_path": {
				Type:     schema.TypeString,
				Required: true,
			},
			"output_dir": {
				Type:     schema.TypeString,
				Required: true,
			},
			"nodes": {
				Type:     schema.TypeList,
				Elem:     &schema.Resource{Type: schema.TypeString},
				Required: true,
			},
			"target_node": {
				Type:     schema.TypeString,
				Required: true,
			},
		},

		ApplyFunc:    applyFn,
		ValidateFunc: validateFn,
	}
}

func configureProvisioner(d *schema.ResourceData, osType afero.Fs) (*provisioner, error) {
	p := &provisioner{
		Channel:          d.Get("channel").(string),
		ClientOptions:    getStringList(d.Get("client_options")),
		Environment:      d.Get("environment").(string),
		UsePolicyfile:    d.Get("use_policyfile").(bool),
		SkipInstall:      d.Get("skip_install").(bool),
		HTTPProxy:        d.Get("http_proxy").(string),
		HTTPSProxy:       d.Get("https_proxy").(string),
		NOProxy:          getStringList(d.Get("no_proxy")),
		NamedRunList:     d.Get("named_run_list").(string),
		OSType:           d.Get("os_type").(string),
		SSLVerifyMode:    d.Get("ssl_verify_mode").(string),
		Version:          d.Get("version").(string),
		InstanceId:       d.Get("instance_id").(string),
		useSudo:          d.Get("use_sudo").(bool),
		installAsService: d.Get("install_as_service").(bool),
		Nodes:            d.Get("nodes").([]interface{}),
		Resources:        d.Get("resources").([]interface{}),
		TargetNode:       d.Get("target_node").(string),
		OutputDir:        d.Get("output_dir").(string),
		ChefModulePath:   d.Get("chef_module_path").(string),
		os:               afero.NewOsFs(),
	}

	if osType != nil {
		p.os = osType
	}

	if nodes, ok := d.GetOk("nodes"); ok {
		for _, node := range nodes.([]interface{}) {
			tmp := make(map[string]interface{})
			if err := json.Unmarshal([]byte(node.(string)), &tmp); err != nil {
				return nil, fmt.Errorf("error unable to render json %s: %v", node, err)
			}
		}
	}

	if targetNode, ok := d.GetOk("target_node"); ok {
		tmp := make(map[string]interface{})
		if err := json.Unmarshal([]byte(targetNode.(string)), &tmp); err != nil {
			return nil, fmt.Errorf("error unable to render json %s: %v", targetNode.(string), err)
		}
	}

	chefPath, err := homedir.Expand(p.ChefModulePath)
	if _, err = p.os.Stat(chefPath); err != nil {
		return nil, fmt.Errorf("error expanding the chef module path %s: %v", chefPath, err)
	}

	outputDir, err := homedir.Expand(p.OutputDir)
	if _, err := p.os.Stat(outputDir); err == nil {
		p.os.RemoveAll(outputDir)
	}
	if err := p.os.MkdirAll(outputDir, 0766); err != nil {
		return nil, fmt.Errorf("error creating output directory %s: %v", outputDir, err)
	}
	p.OutputDir = outputDir
	p.BaseOutputDir = path.Base(p.OutputDir)

	// Make sure the SSLVerifyMode value is written as a symbol
	if p.SSLVerifyMode != "" && !strings.HasPrefix(p.SSLVerifyMode, ":") {
		p.SSLVerifyMode = fmt.Sprintf(":%s", p.SSLVerifyMode)
	}

	return p, nil
}

func (p *provisioner) configurePerOS(s *terraform.InstanceState) error {
	if p.OSType == "" {
		switch t := s.Ephemeral.ConnInfo["type"]; t {
		case "ssh", "": // The default connection type is ssh, so if the type is empty assume ssh
			p.OSType = "linux"
		case "winrm":
			p.OSType = "windows"
		default:
			return fmt.Errorf("unsupported connection type: %s", t)
		}
	}
	// Set some values based on the targeted OS
	switch p.OSType {
	case "linux":
		p.osUploadConfigFiles = p.linuxUploadConfigFiles
		p.installChefClient = p.linuxInstallChefClient
		p.installService = p.linuxInstallChefAsAService
		p.DefaultConfDir = linuxConfDir
		p.runChefClient = p.runChefClientFunc(linuxChefCmd, linuxConfDir)
	case "windows":
		p.osUploadConfigFiles = p.windowsUploadConfigFiles
		p.installChefClient = p.windowsInstallChefClient
		p.installService = p.windowsInstallChefAsAService
		p.DefaultConfDir = windowsConfDir
		p.runChefClient = p.runChefClientFunc(windowsChefCmd, windowsConfDir)
		p.useSudo = false
	default:
		return fmt.Errorf("unsupported os type: %s", p.OSType)
	}
	return nil
}
func validateFn(_ *terraform.ResourceConfig) (ws []string, es []error) {
	return ws, es
}

// Output implementation of terraform.UIOutput interface
func (p *provisioner) Output(output string) {
	logFile := path.Join(logfileDir, p.InstanceId)
	f, err := p.os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("Error creating logfile %s: %v", logFile, err)
		return
	}
	defer f.Close()

	// These steps are needed to remove any ANSI escape codes used to colorize
	// the output and to make sure we have proper line endings before writing
	// the string to the logfile.
	re := regexp.MustCompile(`\x1b\[[0-9;]+m`)
	output = re.ReplaceAllString(output, "")
	output = strings.Replace(output, "\r", "\n", -1)

	if _, err := f.WriteString(output); err != nil {
		log.Printf("Error writing output to logfile %s: %v", logFile, err)
	}

	if err := f.Sync(); err != nil {
		log.Printf("Error saving logfile %s to disk: %v", logFile, err)
	}
}
