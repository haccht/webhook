package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/facebookgo/pidfile"
	flags "github.com/jessevdk/go-flags"
)

type HookList struct {
	Hooks []HookItem
}

type HookItem struct {
	Name    string
	Exec    string
	Workdir string
}

func webhookHandleFunc(h HookItem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		log.Printf("Triggered %s hook successfully", h.Name)

		tmpfile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("%s-", h.Name))
		if err != nil {
			http.Error(w, "HTTP Post request could not be read", 400)
			return
		}
		defer tmpfile.Close()
		defer os.Remove(tmpfile.Name())

		_, err = io.Copy(tmpfile, r.Body)
		if err != nil {
			http.Error(w, "HTTP Post request could not be read", 400)
			return
		}
		defer r.Body.Close()

		commands := strings.Fields(h.Exec)
		commands = append(commands, tmpfile.Name())
		log.Printf("Executing %s", strings.Join(commands, " "))

		cmd := exec.Command(commands[0], commands[1:]...)
		cmd.Stdout = w
		cmd.Stderr = w
		if h.Workdir != "" {
			cmd.Dir = h.Workdir
		}

		cmd.Run()
	}
}

func main() {
	var options struct {
		Addr        string `short:"a" long:"addr" description:"Address to listen on" default:":8080"`
		Hook        string `short:"h" long:"hook" description:"Path to the toml file containing hooks definition" required:"true"`
		PidPath     string `long:"pid" description:"Create PID file at the given path"`
		EnableTLS   bool   `long:"tls" description:"Activate https instead of http"`
		TLSKeyPath  string `long:"tls-key"  description:"Path to the private key pem file for HTTPS"`
		TLSCertPath string `long:"tls-cert" description:"Path to the certificate pem file for HTTPS"`
	}

	if _, err := flags.ParseArgs(&options, os.Args); err != nil {
		if fe, ok := err.(*flags.Error); ok && fe.Type == flags.ErrHelp {
			os.Exit(0)
		}
		log.Fatal(err)
	}

	var hooks HookList
	_, err := toml.DecodeFile(options.Hook, &hooks)
	if err != nil {
		log.Fatal(err)
	}

	for _, h := range hooks.Hooks {
		log.Printf("Loaded %s hook\n", h.Name)
		http.HandleFunc(path.Join("/", h.Name), webhookHandleFunc(h))
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})

	if options.PidPath != "" {
		pidfile.SetPidfilePath(options.PidPath)
		if err := pidfile.Write(); err != nil {
			log.Fatal(err)
		}
		defer os.Remove(options.PidPath)
	}

	if options.EnableTLS {
		log.Printf("Listening on %s with secured connection\n", options.Addr)
		log.Fatal(http.ListenAndServeTLS(options.Addr, options.TLSCertPath, options.TLSKeyPath, nil))
	} else {
		log.Printf("Listening on %s\n", options.Addr)
		log.Fatal(http.ListenAndServe(options.Addr, nil))
	}
}
