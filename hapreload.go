package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codeskyblue/go-sh"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
)

const frontendTmpl = `
acl is{{.Acl}} hdr_beg(host) {{.Hostname}}
use_backend {{.Backend}} if is{{.Acl}}
`
const backendTmpl = `
backend {{.Backend}}
  server {{.Backend}} {{.Hostname}}:{{.Port}} check inter 10000
`

var confPath = "/haproxy/conf"
var haproxyPath = "/haproxy"

// Service to be added
type Service struct {
	// service name
	Name string
	// service port
	Port string
	// .example.com
	Domain string
}

// Haproxy ...
type Haproxy int

// Result ...
type Result int

// Add a frontend and backend
func (h *Haproxy) Add(r *http.Request, service *Service, result *Result) error {
	log.Printf("Add service %s.%s:%s", service.Name, service.Domain, service.Port)
	data := struct {
		Acl      string
		Hostname string
		Backend  string
		Port     string
	}{
		strings.Title(service.Name),
		service.Name + service.Domain,
		service.Name,
		service.Port,
	}

	// Generate frontend entry
	tmpl := template.Must(template.New("frontend").Parse(frontendTmpl))
	f, err := os.OpenFile(confPath+"/"+service.Name+".frontend", os.O_CREATE|os.O_RDWR, 0777)
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
	f, err = os.OpenFile(confPath+"/"+service.Name+".backend", os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		*result = 0
		return err
	}
	err = tmpl.Execute(f, data)
	if err != nil {
		*result = 0
		return err
	}

	//join all the configs
	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

// Remove a frontend and backend
func (h *Haproxy) Remove(r *http.Request, service *Service, result *Result) error {
	log.Printf("Remove service %s.%s:%s", service.Name, service.Domain, service.Port)
	sh.Command("rm", "-f", confPath+"/"+service.Name+".backend").Run()
	sh.Command("rm", "-f", confPath+"/"+service.Name+".frontend").Run()

	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

func (h *Haproxy) generateCfg() error {
	// if conf doesn't exist , create from default
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		_, err := sh.Command("cp", "-rf", "/default_conf", confPath).Output()
		if err != nil {
			log.Println("error:", err.Error())
		}
	}

	// check if haproxy.cfg already exists and take a backup
	if _, err := os.Stat(haproxyPath + "/haproxy.cfg"); !os.IsNotExist(err) {
		err := os.Rename(haproxyPath+"/haproxy.cfg", haproxyPath+"/haproxy.cfg.BAK."+string(time.Now().Format("20060102150405")))
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
	parts := []string{".globalcfg", ".defaultcfg", ".frontendcfg", ".frontend", ".backend"}
	for i := range parts {
		partFunc(parts[i])
	}

	//write the file
	ioutil.WriteFile(haproxyPath+"/haproxy.cfg", haproxyCfg, 0777)

	// restart haproxy container
	session := sh.NewSession()
	//reload haproxy
	haproxyName := os.Getenv("HAPROXY_CONTAINER_NAME")
	out, err := session.Command("docker", "inspect", "-f", "{{.State.Running}}", haproxyName).Output()
	if err != nil {
		log.Println("error:", err.Error())
	}
	log.Println("Haproxy isRunning", string(out))
	if strings.Contains(string(out), "false") {
		log.Printf("Can't reload. %v is not running", haproxyName)
		return nil
	}

	log.Println("Reloading haproxy container....", haproxyName)
	out, err = session.Command("docker", "kill", "-s", "HUP", haproxyName).Output()
	if err != nil {
		log.Println("error:", err.Error())
	}
	log.Println("isReloaded: ", string(out))
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

func main() {

	if os.Getenv("HAPROXY_CONTAINER_NAME") == "" {
		log.Println("Please set env HAPROXY_CONTAINER_NAME")
		return
	}

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
	s.RegisterService(haproxy, "")
	r := mux.NewRouter()
	r.Handle("/haproxy", s)
	http.ListenAndServe(":34015", r)

}
