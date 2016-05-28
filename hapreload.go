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

	// Generate docker compose
	tmpl := template.Must(template.New("frontend").Parse(frontendTmpl))
	f, err := os.OpenFile("./conf/"+service.Name+".frontend", os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		*result = 0
		return err
	}
	err = tmpl.Execute(f, data)
	if err != nil {
		*result = 0
		return err
	}

	tmpl = template.Must(template.New("backend").Parse(backendTmpl))
	f, err = os.OpenFile("./conf/"+service.Name+".backend", os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		*result = 0
		return err
	}
	err = tmpl.Execute(f, data)
	if err != nil {
		*result = 0
		return err
	}

	//join
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

	if _, err := os.Stat("haproxy.cfg"); !os.IsNotExist(err) {
		err := os.Rename("haproxy.cfg", "haproxy.cfg.BAK."+string(time.Now().Format("20060102150405")))
		if err != nil {
			return err
		}
	}

	var haproxyCfg []byte

	var partFunc = func(part string) {
		// walk all files in directory
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

	partFunc(".globalcfg")
	partFunc(".defaultcfg")
	partFunc(".frontendcfg")
	partFunc(".frontend")
	partFunc(".backend")
	ioutil.WriteFile("haproxy.cfg", haproxyCfg, 0777)

	session := sh.NewSession()
	session.SetEnv("DOCKER_HOST", os.Getenv("DOCKER_HOST"))
	//reload
	session.Command("docker", "kill", "-s", "HUP", "haproxy").Run()

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
