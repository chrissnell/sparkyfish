package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/gizak/termui"
)

func (sc *sparkyClient) beginSession() {
	var err error

	sc.conn, err = net.Dial("tcp", os.Args[1])
	if err != nil {
		fatalError(err)
	}

	// Create a bufio.Reader for our connection
	sc.reader = bufio.NewReader(sc.conn)

	err = sc.writeCommand("HELO0")
	if err != nil {
		termui.Clear()
		termui.Close()
		log.Fatalln(err)
	}

	response, err := sc.reader.ReadString('\n')
	if err != nil {
		termui.Clear()
		termui.Close()
		log.Fatalln(err)
	}
	sc.serverAddr = strings.Split(response, " ")[0]

}

func (sc *sparkyClient) writeCommand(cmd string) error {
	s := fmt.Sprintf("%v\r\n", cmd)
	_, err := sc.conn.Write([]byte(s))
	return err
}
