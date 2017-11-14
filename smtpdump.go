package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/mail"
	"os"
	"time"

	"github.com/mhale/smtpd"
)

var (
	addr     string
	hostname string
	output   string
	verbose  bool
)

func init() {
	hn, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	flag.StringVar(&hostname, "hostname", hn, "Server host name")
	flag.StringVar(&addr, "addr", "127.0.0.1:2525", "Listen address:port")
	flag.StringVar(&output, "output", "", "Output directory (default to current directory)")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
}

func main() {
	flag.Parse()

	if hostname == "" {
		log.Fatalln("Hostname cannot be empty")
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
	log.Fatalln(smtpd.ListenAndServe(addr, outputHandler(output, verbose), "SMTPDump", ""))
}

func outputHandler(output string, verbose bool) smtpd.Handler {
	return func(origin net.Addr, from string, to []string, data []byte) {
		now := time.Now()
		msg, err := mail.ReadMessage(bytes.NewReader(data))
		if err != nil {
			log.Println(err)

			return
		}
		subject := msg.Header.Get("Subject")

		log.Printf("Received mail from %q with subject %q\n", from, subject)

		f, err := ioutil.TempFile(output, now.Format(time.RFC3339))
		if err != nil {
			log.Println(err)

			return
		}
		defer func() { _ = f.Close() }()

		_, err = f.WriteString(
			fmt.Sprintf(
				"From: %s\r\nTo: %v\r\nSubject: %s\r\nDate: %s\r\n\r\n",
				from,
				to,
				subject,
				now.Format(time.RFC1123Z)),
		)
		if err != nil {
			log.Println(err)

			return
		}

		_, err = io.Copy(f, msg.Body)
		if err != nil {
			log.Println(err)
		}

		if verbose {
			log.Printf("Wrote %q\n", f.Name())
		}
	}
}
