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

// Service ..
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
	f, err := os.OpenFile("./conf/"+service.Name+".frontend", os.O_CREATE|os.O_RDWR, 0644)
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
	f, err = os.OpenFile("./conf/"+service.Name+".backend", os.O_CREATE|os.O_RDWR, 0644)
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

	sh.Command("rm", "-f", "conf/"+service.Name+".backend").Run()
	sh.Command("rm", "-f", "conf/"+service.Name+".frontend").Run()

	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

func (h *Haproxy) generateCfg() error {
	// check if haproxy.cfg already exists and take a backup
	if _, err := os.Stat("haproxy.cfg"); !os.IsNotExist(err) {
		err := os.Rename("haproxy.cfg", "haproxy.cfg.BAK."+string(time.Now().Format("20060102150405")))
		if err != nil {
			return err
		}
	}

	var haproxyCfg []byte

	var partFunc = func(part string) {
		// walk all files in the directory
		filepath.Walk("conf", func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() && strings.HasSuffix(info.Name(), part) {
				b, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				log.Println(string(b))
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
	ioutil.WriteFile("haproxy.cfg", haproxyCfg, 0644)

	// restart haproxy container
	session := sh.NewSession()
	session.SetEnv("DOCKER_HOST", os.Getenv("DOCKER_HOST"))
	//reload
	session.Command("docker", "kill", "-s", "HUP", os.Getenv("HAPROXY_CONTAINER_NAME")).Run()

	return nil

}

// Generate regenerates haproxy config
func (h *Haproxy) Generate(r *http.Request, service *Service, result *Result) error {

	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

func main() {

	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	haproxy := new(Haproxy)
	s.RegisterService(haproxy, "")
	if !s.HasMethod("Haproxy.Add") {
		return
	}
	r := mux.NewRouter()
	r.Handle("/haproxy", s)
	http.ListenAndServe(":34015", r)

}
