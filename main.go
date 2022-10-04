package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/facebookgo/pidfile"
	flags "github.com/jessevdk/go-flags"
	"github.com/rs/xid"
)

type HookList struct {
	Hooks []HookItem
}

type HookItem struct {
	Name    string
	Workdir string
	Command string
	Inline  string
}

func ellipsis(text string, length int) string {
	r := []rune(text)
	if len(r) > length {
		return string(r[0:length]) + "..."
	}
	return text
}

func webhookHandleFunc(h HookItem) http.HandlerFunc {
	prefix := fmt.Sprintf("[%s]: [%s] ", h.Name, xid.New())
	logger := log.New(os.Stdout, prefix, log.LstdFlags|log.Lmsgprefix)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		var commands []string
		if h.Inline != "" {
			logger.Printf("Execute command \"sh\"")
			commands = append(commands, "sh", "-c", h.Inline)
		} else if h.Command != "" {
			logger.Printf("Execute command \"%s\"", ellipsis(h.Command, 80))
			commands = append(commands, strings.Fields(h.Command)...)
		} else {
			return
		}

		cmd := exec.Command(commands[0], commands[1:]...)
		cmd.Env = os.Environ()
		if h.Workdir != "" {
			cmd.Dir = h.Workdir
		}

		stdin, _ := cmd.StdinPipe()
		io.Copy(stdin, r.Body)
		defer r.Body.Close()

		var stdout bytes.Buffer
		mw := io.MultiWriter(w, &stdout)
		cmd.Stdout = mw
		cmd.Stderr = mw
		stdin.Close()

		cmd.Run()
		errCode := cmd.ProcessState.ExitCode()
		if errCode != 0 {
			logger.Printf("Failed with error code: %d", errCode)
		}

		summary := ellipsis(stdout.String(), 80)
		logger.Printf("Result: %s", strconv.Quote(summary))
	}
}

func main() {
	var options struct {
		Addr        string `short:"a" long:"addr" description:"Address to listen on" default:":8080"`
		Hook        string `short:"f" long:"file" description:"Path to the toml file containing hooks definition" required:"true"`
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
		log.Printf("[%s]: Registered hook", h.Name)
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
