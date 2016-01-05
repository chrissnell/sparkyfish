package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/gizak/termui"
)

func (sc *sparkyClient) beginSession() {
	var err error

	sc.conn, err = net.Dial("tcp", os.Args[1])
	if err != nil {
		fatalError(err)
	}

	err = sc.writeCommand("HELO0")
	if err != nil {
		termui.Clear()
		termui.Close()
		log.Fatalln(err)
	}

}

func (sc *sparkyClient) writeCommand(cmd string) error {
	s := fmt.Sprintf("%v\r\n", cmd)
	_, err := sc.conn.Write([]byte(s))
	return err
}
