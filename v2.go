package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sh "github.com/codeskyblue/go-sh"
)

// ServicesV2 ...
type ServicesV2 struct {
	Services              []ServiceV2 `yaml:"services" json:"services"`
	EnableFileBasedReload bool        `yaml:"enableFileBasedReload" json:"enableFileBasedReload"`
	ID                    string      `yaml:"ID" json:"ID"`
}

// ServiceV2 ...
type ServiceV2 struct {
	ID        string     `yaml:"id" json:"id"`
	Frontends []Frontend `yaml:"frontends" json:"frontends"`
	Backends  []Backend  `yaml:"backends" json:"backends"`
	Action    string     `yaml:"action" json:"action"`
}

// Frontend ...
type Frontend struct {
	FrontendName    string           `yaml:"frontendName" json:"frontendName"`
	FrontendConfigs []FrontendConfig `yaml:"frontendConfigs" json:"frontendConfigs"`
	DefaultBackend  string           `yaml:"defaultBackend" json:"defaultBackend"`
	IsTop           bool             `yaml:"isTop" json:"isTop"`
	IsBottom        bool             `yaml:"isBottom" json:"isBottom"`
}

// FrontendConfig ...
type FrontendConfig struct {
	ACLName    string   `yaml:"ACLName" json:"ACLName"`
	ACLs       []string `yaml:"ACLs" json:"ACLs"`
	UseBackend string   `yaml:"useBackend" json:"useBackend"`
}

// Backend ...
type Backend struct {
	Name   string   `yaml:"name" json:"name"`
	IPs    []string `yaml:"IPs" json:"IPs"`
	Server string   `yaml:"server" json:"server"`
	Port   int      `yaml:"port" json:"port"`
	Tail   string   `yaml:"tail" json:"tail"`
}

// HaproxyV2 ...
type HaproxyV2 int

func (h *HaproxyV2) generateCfg() error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		return fmt.Errorf("LOCKED")
	}
	if _, err := os.Stat(haproxyPath + "/haproxy.cfg"); !os.IsNotExist(err) {
		currentTime := string(time.Now().Format("20060102150405"))
		err := os.Rename(haproxyPath+"/haproxy.cfg", haproxyPath+"/haproxy.cfg.BAK."+currentTime)
		if err != nil {
			return fmt.Errorf("Error in backing up Haproxy.cfg: %s", err)
		}
	}
	var haproxyCfg []byte
	var partFunc = func(part string) {
		// walk all files in the directory
		filepath.Walk(confPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("Error in walk through director of conf Folder: %s", err)
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), part) {
				b, err := ioutil.ReadFile(path)
				if err != nil {
					return fmt.Errorf("Error in Generating the Haproxy.cfg: %s", err)
				}
				haproxyCfg = append(haproxyCfg, b...)
			}
			return nil
		})
	}
	//append the configs in the following order
	parts := []string{".globalcfg", ".defaultcfg"}
	filepath.Walk(confPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("Error in walk through director of conf Folder: %s", err)
		}
		// Check if it is not a directory and has prefix .frontend
		if !info.IsDir() && strings.Contains(info.Name(), ".frontend") && strings.HasSuffix(info.Name(), "cfg") {
			frontendName := info.Name()[:len(info.Name())-3]
			log.Println("Found frontend - ", frontendName)
			parts = append(parts, info.Name(), frontendName+"top", frontendName, frontendName+"bottom", frontendName+"default_backend")
		}
		return nil
	})
	parts = append(parts, ".backend")
	log.Println("Parts - ", parts)
	for i := range parts {
		partFunc(parts[i])
	}
	//write the file
	err := ioutil.WriteFile(haproxyPath+"/haproxy.cfg", haproxyCfg, 0777)
	if err != nil {
		return err
	}
	return nil
}

// StartHaproxy ...
func (h *HaproxyV2) StartHaproxy() error {
	return startHaproxy()
}

// Update Add or Remove services from the Haproxy
func (h *HaproxyV2) Update(r *http.Request, services *ServicesV2, result *Result) error {
	time.Sleep(2)
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		*result = 0
		return fmt.Errorf("LOCKED")
	}
	// var defaultBackend interface{}
	for _, service := range services.Services {
		removeServiceConfigurations(service, confPath)
		log.Printf("Add service %+v", service) //, service.Port)
		// Dont add the HAP Rule for the Service Which as to be deleted.
		if service.Action == "Remove" {
			continue
		}
		for _, singleFrontend := range service.Frontends {
			frontendACLs := "\n"
			frontendType := "." + singleFrontend.FrontendName
			if singleFrontend.IsTop {
				frontendType += "top"
			}
			if singleFrontend.IsBottom {
				frontendType += "bottom"
			}
			for _, singleFrontendConfig := range singleFrontend.FrontendConfigs {
				for _, singleACL := range singleFrontendConfig.ACLs {
					frontendACLs += "\n\tacl " + singleFrontendConfig.ACLName + " " + singleACL
				}
				frontendACLs = frontendACLs + "\n\tuse_backend " + singleFrontendConfig.UseBackend + " if " + singleFrontendConfig.ACLName
			}
			log.Println(frontendACLs)
			err := ioutil.WriteFile(confPath+"/"+service.ID+frontendType, []byte(frontendACLs), 0777)
			if err != nil {
				*result = 0
				return fmt.Errorf("Error in creating the frontend files, Error:%s", err)
			}
		}
		backends := ""
		for _, singleBackend := range service.Backends {
			backends += "\n\nbackend " + singleBackend.Name
			for index, singleIP := range singleBackend.IPs {
				backends += "\n\tserver " +
					singleBackend.Server + "-" + strconv.Itoa(index) + " " +
					singleIP + ":" + strconv.Itoa(singleBackend.Port) + " " +
					singleBackend.Tail
			}
		}
		err := ioutil.WriteFile(confPath+"/"+service.ID+".backend", []byte(backends), 0777)
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in creating the backend files, Error:%s", err)
		}
		for _, singleFrontend := range service.Frontends {
			if singleFrontend.DefaultBackend != "" {
				defaultBackendContent := "\n\tdefault_backend " + singleFrontend.DefaultBackend
				err := ioutil.WriteFile(confPath+"/"+service.ID+"."+singleFrontend.FrontendName+"default_backend",
					[]byte(defaultBackendContent), 0777)
				if err != nil {
					*result = 0
					return fmt.Errorf("Error in creating the default backend file, Error:%s", err)
				}
			}
		}
	}
	// join all the configs
	if err := h.generateCfg(); err != nil {
		for _, service := range services.Services {
			removeServiceConfigurations(service, confPath)
		}
		if err := h.generateCfg(); err != nil {
			*result = 0
			return err
		}
		*result = 0
		return err
	}
	if err := validateHaproxy(); err != nil {
		for _, service := range services.Services {
			removeServiceConfigurations(service, confPath)
		}
		if err := h.generateCfg(); err != nil {
			*result = 0
			return err
		}
		*result = 0
		return err
	}
	if services.EnableFileBasedReload {
		if err := addToReloadFile(services.ID); err != nil {
			for _, service := range services.Services {
				removeServiceConfigurations(service, confPath)
			}
			if err := h.generateCfg(); err != nil {
				*result = 0
				return err
			}
			*result = 0
			return err
		}
	} else {
		if err := reloadHaproxy(); err != nil {
			*result = 0
			return err
		}
	}
	*result = 1
	return nil
}

func removeServiceConfigurations(service ServiceV2, confPath string) {
	log.Printf("Remove service %+v", service)
	sh.Command("rm", "-f", confPath+"/"+service.ID+".backend").Run()
	filepath.Walk(confPath, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("Error in walk through director of conf Folder: %s", err)
		}
		if !fileInfo.IsDir() && strings.Contains(fileInfo.Name(), ".frontend") && strings.HasPrefix(fileInfo.Name(), service.ID) {
			sh.Command("rm", "-f", confPath+"/"+fileInfo.Name()).Run()
		}
		return nil
	})
	sh.Command("rm", "-f", confPath+"/"+service.ID+".frontend*").Run()
}
