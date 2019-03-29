package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sh "github.com/codeskyblue/go-sh"
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
	//action to be added or to be deleted
	Action string
}

// Services ...
type Services struct {
	Services              []Service
	ID                    string
	EnableFileBasedReload bool
}

//ReloadStatus ...
type ReloadStatus struct {
	Time   string
	Result bool
}

// Haproxy ...
type Haproxy int

// Result ...
type Result int

// Health Check
type Health int

// Add a frontend and backend
func (h *Haproxy) Add(r *http.Request, services *Services, result *Result) error {
	time.Sleep(2)
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		*result = 0
		return fmt.Errorf("LOCKED")
	}
	var defaultBackend interface{}
	isDefaultBackedDefined := false
	for _, service := range services.Services {
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontendtop").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontendbottom").Run()

		log.Printf("Add service %s:%s", service.HaproxyURLs, service.Port)
		//Dont add the HAP Rule for the Service Which as to be deleted.
		if service.Action == "Remove" && services.ID != "" {
			continue
		}
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
			return fmt.Errorf("Error in creating the frontend files, Error:%s", err)
		}
		// fill in the template
		err = tmpl.Execute(f, data)
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in creating the frontend template, Error:%s", err)
		}

		// Generate backend entry
		tmpl = template.Must(template.New("backend").Parse(backendTmpl))
		f, err = os.OpenFile(confPath+"/"+service.ACL+".backend", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in creating the backend files, Error:%s", err)
		}
		err = tmpl.Execute(f, data)
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in creating the frontend template, Error:%s", err)
		}
	}

	// Generate default_backend if needed
	if isDefaultBackedDefined {
		tmpl := template.Must(template.New("default_backend").Parse(defaultBackendTmpl))
		f, err := os.OpenFile(confPath+"/"+".default_backend", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in creating the default files, Error:%s", err)
		}
		err = tmpl.Execute(f, defaultBackend)
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in creating the template default, Error:%s", err)
		}
	}

	//join all the configs
	if err := h.generateCfg(); err != nil {
		for _, service := range services.Services {
			log.Printf("Remove service %s:%s", service.HaproxyURLs, service.Port)
			sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
			sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
		}
		*result = 0
		return err
	}

	if err := h.ValidateHaproxy(); err != nil {
		for _, service := range services.Services {
			log.Printf("Remove service %s:%s", service.HaproxyURLs, service.Port)
			sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
			sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
		}
		if err := h.generateCfg(); err != nil {
			*result = 0
			return err
		}
		*result = 0
		return err
	}
	if services.EnableFileBasedReload {
		if err := h.AddToReloadFile(services.ID); err != nil {
			for _, service := range services.Services {
				log.Printf("Remove service %s:%s", service.HaproxyURLs, service.Port)
				sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
				sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
			}
			if err := h.generateCfg(); err != nil {
				*result = 0
				return err
			}
			*result = 0
			return err
		}
	} else {
		//Reload Haproxy for Non-Swarm Setup[i.e Standalone containers].
		if err := h.ReloadHaproxy(); err != nil {
			*result = 0
			return err
		}
	}

	*result = 1
	return nil

}

// Remove a frontend and backend
func (h *Haproxy) Remove(r *http.Request, services *Services, result *Result) error {
	time.Sleep(2)
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		*result = 0
		return fmt.Errorf("LOCKED")
	}
	for _, service := range services.Services {
		log.Printf("Remove service %s:%s", service.HaproxyURLs, service.Port)
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".backend").Run()
		sh.Command("rm", "-f", confPath+"/"+service.ACL+".frontend").Run()
	}

	if err := h.generateCfg(); err != nil {
		*result = 0
		return err
	}

	if err := h.ValidateHaproxy(); err != nil {
		*result = 0
		return err
	}

	if err := h.AddToReloadFile(services.ID); err != nil {
		*result = 0
		return err
	}

	*result = 1
	return nil

}

func (h *Haproxy) generateCfg() error {
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
	return startHaproxy()
}

func startHaproxy() error {
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
	return reloadHaproxy()
}

func reloadHaproxy() error {
	session := sh.NewSession()
	err := session.Command("/usr/bin/reload.sh").Run()
	if err != nil {
		return err
	}
	return nil
}

//Reload ...
func (h *Haproxy) Reload(r *http.Request, reloadStatus *ReloadStatus, result *Result) error {
	log.Printf("%s: Cron Reload Started", reloadStatus.Time)
	if err := h.ReloadHaproxy(); err != nil {
		log.Printf("%s: Cron Reload Failed Reason:%s", reloadStatus.Time, err)
		*result = 0
		return err
	}
	log.Printf("%s: Cron Reload Success", reloadStatus.Time)
	*result = 1
	return nil
}

// ValidateHaproxy ...
func (h *Haproxy) ValidateHaproxy() error {
	return validateHaproxy()
}

func validateHaproxy() error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		return fmt.Errorf("LOCKED")
	}
	session := sh.NewSession()
	err := session.Command("haproxy", "-c", "-f", haproxyPath+"/haproxy.cfg").Run()
	if err != nil {
		return fmt.Errorf("Validation Status: Error in Hap Config: %s", err)
	}
	return nil
}

// LockForReload ...
func (h *Haproxy) LockForReload(r *http.Request, reloadStatus *ReloadStatus, result *Result) error {
	if _, err := os.Stat(haproxyPath + "/toReload"); os.IsNotExist(err) {
		*result = 0
		return fmt.Errorf("toReload_File_Absent")
	}
	currentTime := string(time.Now().Format("20060102150405"))
	if _, err := os.Stat(haproxyPath + "/reloadFailed"); !os.IsNotExist(err) {
		err := os.Rename(haproxyPath+"/reloadFailed", haproxyPath+"/reloadFailed.BAK."+currentTime)
		if err != nil {
			*result = 0
			return fmt.Errorf("Failed to rename the reloadFailed File:%s", err)
		}
	}
	if _, err := os.Stat(haproxyPath + "/reloadSuccess"); !os.IsNotExist(err) {
		err := os.Rename(haproxyPath+"/reloadSuccess", haproxyPath+"/reloadSuccess.BAK."+currentTime)
		if err != nil {
			*result = 0
			return fmt.Errorf("Failed to rename the reloadSuccess File:%s", err)
		}
	}
	session := sh.NewSession()
	err := session.Command("touch", haproxyPath+"/lock").Run()
	if err != nil {
		*result = 0
		return fmt.Errorf("Failed to create the lock File:%s", err)
	}
	*result = 1
	return nil
}

// ReleaseReloadLock ...
func (h *Haproxy) ReleaseReloadLock(r *http.Request, reloadStatus *ReloadStatus, result *Result) error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		if reloadStatus.Result == true {
			if _, ferr := os.Stat(haproxyPath + "/toReload"); !os.IsNotExist(ferr) {
				rnerr := os.Rename(haproxyPath+"/toReload", haproxyPath+"/reloadSuccess")
				if rnerr != nil {
					session := sh.NewSession()
					err := session.Command("rm", "-f", haproxyPath+"/lock").Run()
					if err != nil {
						*result = 0
						return fmt.Errorf("Failed to remove the Lock File when trying to rename toRelaod to reloadSuccess:%s", err)
					}
					log.Printf("%s: Cron Reload Failed Rename to reloadSuccess File:%s", reloadStatus.Time, rnerr)
					*result = 0
					return fmt.Errorf("Failed to rename the toReload reloadSuccess:%s", rnerr)
				}
			}
		} else {
			if _, ferr := os.Stat(haproxyPath + "/toReload"); !os.IsNotExist(ferr) {
				rnerr := os.Rename(haproxyPath+"/toReload", haproxyPath+"/reloadFailed")
				if rnerr != nil {
					session := sh.NewSession()
					err := session.Command("rm", "-f", haproxyPath+"/lock").Run()
					if err != nil {
						*result = 0
						return fmt.Errorf("Failed to remove the Lock File when trying to rename toRelaod to reloadFailed:%s", err)
					}
					log.Printf("%s: Cron Reload Failed Rename to reloadFailed File:%s", reloadStatus.Time, rnerr)
					*result = 0
					return fmt.Errorf("Failed to rename the toReload reloadFailed:%s", rnerr)
				}
			}
		}
		session := sh.NewSession()
		err := session.Command("rm", "-f", haproxyPath+"/lock").Run()
		if err != nil {
			*result = 0
			return fmt.Errorf("Failed to remove the Lock File:%s", err)
		}
	}
	*result = 1
	return nil
}

// CheckReloadStatus ...
func (h *Haproxy) CheckReloadStatus(r *http.Request, services *Services, result *Result) error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		*result = 0
		return fmt.Errorf("LOCKED")
	}
	if _, err := os.Stat(haproxyPath + "/toReload"); !os.IsNotExist(err) {
		fileContent, err := ioutil.ReadFile(haproxyPath + "/toReload")
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in reading the toReload File:%s", err)
		}
		fileContentString := string(fileContent)
		clusterPresent := false
		fileContentSlice := strings.Split(fileContentString, "\n")
		for _, cluster := range fileContentSlice {
			if cluster == services.ID {
				clusterPresent = true
			}
		}
		if clusterPresent {
			*result = 0
			return fmt.Errorf("WAITING")
		}
	}
	if _, err := os.Stat(haproxyPath + "/reloadSuccess"); !os.IsNotExist(err) {
		fileContent, err := ioutil.ReadFile(haproxyPath + "/reloadSuccess")
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in reading the reloadSuccess File:%s", err)
		}
		fileContentString := string(fileContent)
		if strings.Contains(fileContentString, services.ID) {
			*result = 1
			return nil
		}
	}
	if _, err := os.Stat(haproxyPath + "/reloadFailed"); !os.IsNotExist(err) {
		fileContent, err := ioutil.ReadFile(haproxyPath + "/reloadFailed")
		if err != nil {
			*result = 0
			return fmt.Errorf("Error in reading the reloadFailed File:%s", err)
		}
		fileContentString := string(fileContent)
		if strings.Contains(fileContentString, services.ID) {
			*result = 0
			return fmt.Errorf("Hap Reload Failed Cluster name found in reloadFailed File:%s", err)
		}
	}
	*result = 0
	return fmt.Errorf("Failed to find the Cluster Name in reloadSuccess or reloadFailed File")
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
func (h *Haproxy) BringIntoLB(r *http.Request, ReloadStatus *ReloadStatus, result *Result) error {
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
func (h *Haproxy) BringOutOfLB(r *http.Request, ReloadStatus *ReloadStatus, result *Result) error {
	session := sh.NewSession()
	err := session.Command("rm", "-f", "/usr/local/etc/live").Run()
	if err != nil {
		*result = 0
		return fmt.Errorf("Failed to BringOutOfLB:%s", err)
	}
	*result = 1
	return nil
}

//AddToReloadFile ... id -> refers to clusterName
func (h *Haproxy) AddToReloadFile(ID string) error {
	return addToReloadFile(ID)
}

func addToReloadFile(ID string) error {
	if _, err := os.Stat(haproxyPath + "/lock"); !os.IsNotExist(err) {
		return fmt.Errorf("LOCKED")
	}
	if _, err := os.Stat(haproxyPath + "/toReload"); os.IsNotExist(err) {
		f, err := os.OpenFile(haproxyPath+"/toReload", os.O_CREATE, 0755)
		defer f.Close()
		if err != nil {
			return fmt.Errorf("Error in creating the toReload File:%s", err)
		}
	}
	fileContent, err := ioutil.ReadFile(haproxyPath + "/toReload")
	if err != nil {
		return fmt.Errorf("Error in reading the toReload File:%s", err)
	}
	fileContentString := string(fileContent)
	clusterPresent := false
	fileContentSlice := strings.Split(fileContentString, "\n")
	for _, clusterID := range fileContentSlice {
		if clusterID == ID {
			clusterPresent = true
		}
	}
	if !clusterPresent {
		f, err := os.OpenFile(haproxyPath+"/toReload", os.O_APPEND|os.O_RDWR, 0755)
		defer f.Close()
		if err != nil {
			return fmt.Errorf("Error in Writing to toReload File:%s", err)
		}
		_, err = f.WriteString(ID + "\n")
		if err != nil {
			return fmt.Errorf("Error in Writing to toReload File:%s", err)
		}
		f.Sync()
	}
	return nil
}

//KillHAP ...
func (h *Haproxy) KillHAP(r *http.Request, ReloadStatus *ReloadStatus, result *Result) error {
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
	log.Println("Healthcheck")
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
	version := flag.String("version", "v1", "v1 will run the old version while v2 will run the new version")
	flag.Parse()
	//if running without docker
	if os.Getenv("CONF_PATH") != "" {
		confPath = os.Getenv("CONF_PATH")
	}
	if os.Getenv("HAPROXY_PATH") != "" {
		haproxyPath = os.Getenv("HAPROXY_PATH")
	}
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	r := mux.NewRouter()
	r.HandleFunc("/health", HealthCheck).Methods("HEAD")
	switch *version {
	case "v1":
		haproxy := new(Haproxy)
		haproxy.generateCfg()
		err := haproxy.StartHaproxy()
		createLiveFile(err)
		s.RegisterService(haproxy, "")
		r.Handle("/haproxy", s)
	case "v2":
		haproxy := new(HaproxyV2)
		haproxy.generateCfg()
		err := haproxy.StartHaproxy()
		createLiveFile(err)
		s.RegisterService(haproxy, "")
		r.Handle("/haproxy", s)
	}
	http.ListenAndServe(":34015", r)
}

func createLiveFile(err error) {
	if err == nil {
		session := sh.NewSession()
		err = session.Command("touch", "/usr/local/etc/live").Run()
		if err != nil {
			log.Println("Failed to create the live File")
		}
	}
}
