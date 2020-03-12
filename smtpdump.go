package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	c "github.com/fatih/color"
	"github.com/mhale/smtpd"
)

var (
	addr        = flag.String("addr", "127.0.0.1:2525", "Listen address:port")
	cert        = flag.String("cert", "", "PEM-encoded certificate")
	color       = flag.Bool("color", true, "color debug output")
	extension   = flag.String("extension", "eml", "Saved file extension")
	hostname    string
	pkey        = flag.String("key", "", "PEM-encoded private key")
	output      = flag.String("output", "", "Output directory (default to current directory)")
	verbose     = flag.Bool("verbose", false, "verbose output")
	readPrintf  = c.New(c.FgGreen).Printf
	writePrintf = c.New(c.FgCyan).Printf
)

func init() {
	hn, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	flag.StringVar(&hostname, "hostname", hn, "Server host name")
	flag.BoolVar(&smtpd.Debug, "debug", false, "debug output")
}

func main() {
	flag.Parse()

	if hostname == "" {
		log.Fatalln("Hostname cannot be empty")
	}

	if smtpd.Debug {
		*verbose = true

		if !*color {
			readPrintf = fmt.Printf
			writePrintf = fmt.Printf
		}
	}

	var err error
	if *output == "" {
		*output, err = os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}
	}
	_, err = os.Stat(*output)
	if err != nil {
		log.Fatalln(err)
	}

	srv := &smtpd.Server{
		Addr:        *addr,
		Appname:     "SMTPDump",
		AuthHandler: authHandler,
		Handler:     outputHandler(*output, *extension, *verbose),
		LogRead: func(_, _, line string) {
			line = strings.Replace(line, "\n", "\n  ", -1)
			_, _ = readPrintf("  %s\n", line)
		},
		LogWrite: func(_, _, line string) {
			line = strings.Replace(line, "\n", "\n  ", -1)
			_, _ = writePrintf("  %s\n", line)
		},
		HandlerRcpt: rcptHandler,
	}

	if *cert != "" && *pkey != "" {
		err = srv.ConfigureTLS(*cert, *pkey)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("Enabled TLS support")
	}

	if *verbose {
		log.Printf("Listening on %q ...\n", *addr)
	}

	log.Fatalln(srv.ListenAndServe())
}

// authHandler logs credentials and always returns true.
func authHandler(origin net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
	log.Printf("[AUTH] User: %q; Password: %q\n", username, password)
	return true, nil
}

// outputHandler is called when a new message is received by the server.
func outputHandler(output, ext string, verbose bool) smtpd.Handler {
	return func(origin net.Addr, from string, to []string, data []byte) {
		if verbose {
			msg, err := mail.ReadMessage(bytes.NewReader(data))
			if err != nil {
				log.Println(err)

				return
			}
			subject := msg.Header.Get("Subject")

			log.Printf("Received mail from %q with subject %q\n", from, subject)
		}

		f, err := randFile(output, fmt.Sprintf("%d", time.Now().UnixNano()), ext)
		if err != nil {
			log.Println(err)

			return
		}
		defer func() { _ = f.Close() }()

		_, err = io.Copy(f, bytes.NewReader(data))
		if err != nil {
			log.Println(err)
		}

		if verbose {
			log.Printf("Wrote %q\n", f.Name())
		}
	}
}

func rcptHandler(origin net.Addr, from string, to string) bool {
	log.Printf("[RCPT] %q => %q\n", from, to)
	return true
}

// randFile returns a pointer to a new file or an error.  If
// dir is empty, the temporary directory is used.
func randFile(dir, prefix, suffix string) (*os.File, error) {
	var (
		err error
		f   *os.File
	)

	if dir == "" {
		dir = os.TempDir()
	}

	// Make a reasonable number of attempts to find a unique file name.
	for i := 0; i < 10000; i++ {
		// Quick and Dirty congruential generator from Numerical Recipes.
		r := int(time.Now().UnixNano()+int64(os.Getpid()))*1664525 + 1013904223
		fn := fmt.Sprintf("%s_%d.%s", prefix, r, suffix)
		name := filepath.Join(dir, fn)
		f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		if err == nil {
			break
		}
	}

	return f, err
}
