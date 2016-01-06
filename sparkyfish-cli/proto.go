package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/gizak/termui"
)

func (sc *sparkyClient) beginSession() {
	var err error

	sc.conn, err = net.Dial("tcp", sc.serverHostname)
	if err != nil {
		fatalError(err)
	}

	// Create a bufio.Reader for our connection
	sc.reader = bufio.NewReader(sc.conn)

	// First command is always HELO, immediately followed by a single-digit protocol version
	// e.g. "HELO0".
	err = sc.writeCommand(fmt.Sprint("HELO", protocolVersion))
	if err != nil {
		sc.protocolError(err)
	}

	// In response to our HELO, the server will respond like this:
	//   HELO\n
	//   canonicalName\n
	//   location\n
	// where canonicalName is the server's canonical hostname and
	// location is the physical location of the server

	// First, we check for the HELO response
	response, err := sc.reader.ReadString('\n')
	if err != nil {
		sc.protocolError(err)
	}
	response = strings.TrimSpace(response)

	if response != "HELO" {
		sc.protocolError(fmt.Errorf("invalid HELO response from server"))
	}

	var serverBanner bytes.Buffer

	// Next, we check to see if the server provided a cname
	cname, err := sc.reader.ReadString('\n')
	if err != nil {
		sc.protocolError(err)
	}
	cname = strings.TrimSpace(cname)

	if cname == "none" {
		// If a cname was not provided, we'll just show the hostname that the
		// test was run against
		cname, _, _ = net.SplitHostPort(sc.serverHostname)
	}

	serverBanner.WriteString(sanitize(cname))

	// Finally we check to see if the server provided a location
	location, err := sc.reader.ReadString('\n')
	if err != nil {
		sc.protocolError(err)
	}
	location = strings.TrimSpace(location)

	if location != "none" {
		serverBanner.WriteString(" :: ")
		serverBanner.WriteString(sanitize(location))
	}

	if serverBanner.Len() > 0 {
		// Don't write a banner longer than 60 characters
		if serverBanner.Len() > 60 {
			sc.wr.jobs["bannerbox"].(*termui.Par).Text = serverBanner.String()[:59]
		} else {
			sc.wr.jobs["bannerbox"].(*termui.Par).Text = serverBanner.String()
		}
		sc.wr.Render()
	}

}

func (sc *sparkyClient) protocolError(err error) {
	termui.Clear()
	termui.Close()
	log.Fatalln(err)
}

func (sc *sparkyClient) writeCommand(cmd string) error {
	s := fmt.Sprintf("%v\r\n", cmd)
	_, err := sc.conn.Write([]byte(s))
	return err
}

func sanitize(str string) string {
	b := make([]byte, len(str))
	var bi int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c < 127 {
			b[bi] = c
			bi++
		}
	}
	return string(b[:bi])
}
