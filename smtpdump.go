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
	"time"

	"github.com/mhale/smtpd"
)

var (
	addr      string
	debug     bool
	extension string
	hostname  string
	output    string
	verbose   bool
)

func init() {
	hn, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	flag.StringVar(&hostname, "hostname", hn, "Server host name")
	flag.StringVar(&addr, "addr", "127.0.0.1:2525", "Listen address:port")
	flag.StringVar(&output, "output", "", "Output directory (default to current directory)")
	flag.StringVar(&extension, "extension", "eml", "Saved file extension")
	flag.BoolVar(&debug, "debug", false, "debug output")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
}

func main() {
	flag.Parse()

	if hostname == "" {
		log.Fatalln("Hostname cannot be empty")
	}

	if debug {
		smtpd.Debug = true
		verbose = true
	}

	var err error
	if output == "" {
		output, err = os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}
	}
	_, err = os.Stat(output)
	if err != nil {
		log.Fatalln(err)
	}

	if verbose {
		log.Printf("Listening on %q ...\n", addr)
	}
	log.Fatalln(
		smtpd.ListenAndServe(
			addr,
			outputHandler(output, extension, verbose),
			"SMTPDump",
			"",
		),
	)
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
		// Quick and Dirty congruential generator from Numerical Recipies.
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
