package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"strconv"

	"github.com/codeskyblue/go-sh"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

const frontendHeaderACL = `  acl is{{.ACL}} hdr_beg(host) {{index .HaproxyURLs #}}
`
const frontendPathACL = `  acl is{{.ACL}} path_beg -i {{index .HaproxyURLs #}}
`

const frontendUse = `  use_backend {{.Backend}} if is{{.ACL}}
`

// const frontendTmpl = `
// acl is{{.ACL}} hdr_beg(host) {{.HaproxyURL}}
// use_backend {{.Backend}} if is{{.ACL}}
// `
const backendTmpl = `##
backend {{.Backend}}
  server {{.Backend}} {{.Hostmachine}}:{{.Port}} check inter 10000
`

const defaultBackendTmpl = `##
  default_backend {{.Backend}}
`

var confPath = "/usr/local/etc/haproxy/conf"
var haproxyPath = "/usr/local/etc/haproxy"

// Service to be added
type Service struct {
	// ACL to be used
	ACL string
	// URL by which service will be called
	HaproxyURLs []string
	// Backend name
	Backend string
	// storefront-services-1.myntra.com
	Hostmachine string
	// Port on Hostmachine where service runs
	Port string
	//Lock Time
	EpocTime time.Time
	//ClusterName
	ClusterName string
}

// Services ...
type Services struct {
	Services    []Service
	ClusterName string
	EpocTime    int64
}

// Haproxy ...
type Haproxy int

// Result ...
type Result int

// Health Check
type Health int

// Add a frontend and backend
func (h *Haproxy) Add(r *http.Request, services *Services, result *Result) error {
	var defaultBackend interface{}
	isDefaultBackedDefined := false
	for _, service := range services.Services {
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontendtop").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontendbottom").Run()

		log.Printf("Add service %s:%s", service.HaproxyURLs, service.Port)
		data := struct {
			ACL         string
			HaproxyURLs []string
			Backend     string
			Hostmachine string
			Port        string
		}{
			strings.Title(service.ACL),
			service.HaproxyURLs,
			service.Backend,
			service.Hostmachine,
			service.Port,
		}

		frontendACLs := `##
`

		frontendType := ".frontend"

		for haproxyURLId, haproxyURL := range service.HaproxyURLs {
			if haproxyURL == "default_backend" {
				defaultBackend = data
				isDefaultBackedDefined = true
			} else if strings.EqualFold(haproxyURL, "top") || strings.EqualFold(haproxyURL, "bottom") {
				frontendType += haproxyURL
			} else if string(haproxyURL[0]) == "/" {
				frontendACLs += strings.Replace(frontendPathACL, "#", strconv.Itoa(haproxyURLId), -1)
			} else {
				frontendACLs += strings.Replace(frontendHeaderACL, "#", strconv.Itoa(haproxyURLId), -1)
			}
		}

		frontendTmpl := frontendACLs + frontendUse

		log.Println(frontendTmpl)

		// Generate frontend entry
		tmpl := template.Must(template.New("frontend").Parse(frontendTmpl))
		f, err := os.OpenFile(confPath+"/"+service.ACL+frontendType, os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			*result = 0
			return err
		}
		// fill in the template
		err = tmpl.Execute(f, data)
		if err != nil {
			*result = 0
			return err
		}

		// Generate backend entry
		tmpl = template.Must(template.New("backend").Parse(backendTmpl))
		f, err = os.OpenFile(confPath+"/"+service.ACL+".backend", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			*result = 0
			return err
		}
		err = tmpl.Execute(f, data)
		if err != nil {
			*result = 0
			return err
		}
	}

	// Generate default_backend if needed
	if isDefaultBackedDefined {
		tmpl := template.Must(template.New("default_backend").Parse(defaultBackendTmpl))
		f, err := os.OpenFile(confPath+"/"+".default_backend", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			*result = 0
			return err
		}
		err = tmpl.Execute(f, defaultBackend)
		if err != nil {
			*result = 0
			return err
		}
	}

	//join all the configs
	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	if err := h.ReloadHaproxy(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

// Remove a frontend and backend
func (h *Haproxy) Remove(r *http.Request, services *Services, result *Result) error {
	for _, service := range services.Services {
		log.Printf("Remove service %s:%s", service.HaproxyURLs, service.Port)
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
	}

	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	if err := h.ReloadHaproxy(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

func (h *Haproxy) generateCfg() error {
	if _, err := os.Stat(haproxyPath + "/haproxy.cfg"); !os.IsNotExist(err) {
		currentTime := string(time.Now().Format("20060102150405"))
		err := os.Rename(haproxyPath+"/haproxy.cfg", haproxyPath+"/haproxy.cfg.BAK."+currentTime)
		if err != nil {
			return err
		}
	}

	var haproxyCfg []byte

	var partFunc = func(part string) {
		// walk all files in the directory
		filepath.Walk(confPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), part) {
				b, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				haproxyCfg = append(haproxyCfg, b...)
			}
			return nil

		})
	}

	//append the configs in the following order
	parts := []string{".globalcfg", ".defaultcfg", ".frontendcfg", ".frontendtop", ".frontend", ".frontendbottom", ".default_backend", ".backend"}
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
func (h *Haproxy) StartHaproxy() error {
	// restart haproxy container
	session := sh.NewSession()
	err := session.Command("haproxy", "-p", "/var/run/haproxy.pid", "-f", "/usr/local/etc/haproxy/haproxy.cfg").Run()
	if err != nil {
		return err
	}
	return nil
}

// ReloadHaproxy ...
func (h *Haproxy) ReloadHaproxy() error {
	session := sh.NewSession()
	err := session.Command("/usr/bin/reload.sh").Run()
	if err != nil {
		return err
	}
	return nil
}

// Generate regenerates haproxy config from existing configs
func (h *Haproxy) Generate(r *http.Request, service *Service, result *Result) error {
	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

//BringIntoLB ...
func (h *Haproxy) BringIntoLB(r *http.Request, services *Services, result *Result) error {
	session := sh.NewSession()
	err := session.Command("touch", "/usr/local/etc/live").Run()
	if err != nil {
		*result = 0
		return err
	}
	*result = 1
	return nil
}

//BringOutOfLB ...
func (h *Haproxy) BringOutOfLB(r *http.Request, services *Services, result *Result) error {
	session := sh.NewSession()
	err := session.Command("rm", "-f", "/usr/local/etc/live").Run()
	if err != nil {
		*result = 0
		return fmt.Errorf("Failed to BringOutOfLB:%s", err)
	}
	*result = 1
	return nil
}

//LockHAPSession ...
func (h *Haproxy) LockHAPSession(r *http.Request, services *Services, result *Result) error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		b, err := ioutil.ReadFile(haproxyPath + "/lock")
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in Opening the Hap Lock file:%s", err)
		}
		*result = 0
		return fmt.Errorf("Hap Session is Locked by :%s", string(b))
	}
	err := ioutil.WriteFile(haproxyPath+"/lock", []byte(fmt.Sprintf("%d", services.EpocTime)+"-"+services.ClusterName), 0755)
	if err != nil {
		*result = 0
		return fmt.Errorf("Error in writing to file:%s", err)
	}
	*result = 1
	return nil
}

//UnLockHAPSession ...
func (h *Haproxy) UnLockHAPSession(r *http.Request, services *Services, result *Result) error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		b, err := ioutil.ReadFile(haproxyPath + "/lock")
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in Opening the Hap Lock file:%s", err)
		}
		if string(b) == (fmt.Sprintf("%d", services.EpocTime) + "-" + services.ClusterName) {
			session := sh.NewSession()
			err := session.Command("rm", "-f", haproxyPath+"/lock").Run()
			if err != nil {
				*result = 0
				return fmt.Errorf("Error in removing the Lock File for Hap Reload:%s", err)
			}
		}
	}
	*result = 1
	return nil
}

//KillHAP ...
func (h *Haproxy) KillHAP(r *http.Request, services *Services, result *Result) error {
	if _, err := os.Stat("/var/run/haproxy.pid"); !os.IsNotExist(err) {
		session := sh.NewSession()
		err = session.Command("/usr/bin/kill.sh").Run()
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in Killing the Old Hap Instance:%s", err)
		}
	}
	*result = 1
	return nil
}

//HealthCheck ...
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method == "HEAD" {
		if _, err := os.Stat("/usr/local/etc/live"); !os.IsNotExist(err) {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	//if running without docker
	if os.Getenv("CONF_PATH") != "" {
		confPath = os.Getenv("CONF_PATH")
	}

	if os.Getenv("HAPROXY_PATH") != "" {
		haproxyPath = os.Getenv("HAPROXY_PATH")
	}

	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	haproxy := new(Haproxy)
	// generate default haproxy.cfg
	log.Println("Generating default haproxy.cfg")
	haproxy.generateCfg()
	err := haproxy.StartHaproxy()
	if err == nil {
		session := sh.NewSession()
		err = session.Command("touch", "/usr/local/etc/live").Run()
		if err != nil {
			log.Println("Failed to create the live File")
		}
	}
	s.RegisterService(haproxy, "")
	r := mux.NewRouter()
	r.Handle("/haproxy", s)
	//Route Added for Hap Health Check
	r.HandleFunc("/health", HealthCheck).Methods("HEAD")
	http.ListenAndServe(":34015", r)
}
