package main

import (
	"bytes"
	"fmt"
	"github.com/pin/tftp"
	"io"
	"net/http"
	"os"
	"net"
	"strings"
	"time"
)

func defaultFile(ip string) *bytes.Buffer {
	var response string
	response = "default slax-http\r\n"
	response += "prompt 1\r\n"
	response += "timeout 15\r\n\r\n"

	response += "LABEL slax-http\r\n"
	response += "LINUX http://" + ip + "/vmlinuz\r\n"
	response += "APPEND initrd=http://" + ip + "/initrfs.img password=skod vga=773 load_ramdisk=1 prompt_ramdisk=0 rw printk.time=1 slax.flags=perch,xmode\r\n"
	response += "ipappend 1\r\n"
	buf := bytes.NewBufferString(response)
	return buf
}

func readHandler(filename string, r io.ReaderFrom) error {

	fmt.Printf("open: %s\n", filename)
	if strings.Contains(filename, "default") {
		ip := r.(tftp.RequestPacketInfo).LocalIP().String()
		fmt.Printf("Generate default with ip:%s \n", ip)
		n, err := r.ReadFrom(defaultFile(ip))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return err
		}
		fmt.Printf("%d bytes sent\n", n)
		return nil
	}
	file, err := os.Open(filename)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}

	if t, ok := r.(tftp.OutgoingTransfer); ok {
		if fi, err := file.Stat(); err == nil {
			t.SetSize(fi.Size())
		}
	}

	n, err := r.ReadFrom(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	fmt.Printf("%d bytes sent\n", n)
	return nil
}

func localAddresses() {
    ifaces, err := net.Interfaces()
    if err != nil {
        fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
        return
    }
    for _, i := range ifaces {
        addrs, err := i.Addrs()
        if err != nil {
            fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
            continue
        }
        for _, a := range addrs {
            switch v := a.(type) {
            case *net.IPAddr:
                fmt.Printf("%v : %s (%s)\n", i.Name, v, v.IP.DefaultMask())

            case *net.IPNet:
                fmt.Printf("%v : %s [%v/%v]\n", i.Name, v, v.IP, v.Mask)
            }

        }
    }
}

func main() {
	localAddresses()
	http.Handle("/", http.FileServer(http.Dir("/files/")))
	go func() {
		http.ListenAndServe(":80", nil)
	}()
	go func() {
		http.ListenAndServe(":7529", nil)
	}()
	go func() {
		StartDhcpServer()
	}()

	// use nil in place of handler to disable read or write operations
	s := tftp.NewServer(readHandler, nil)
	s.SetTimeout(5 * time.Second)  // optional
	err := s.ListenAndServe(":69") // blocks until s.Shutdown() is called
	if err != nil {
		fmt.Fprintf(os.Stdout, "server: %v\n", err)
		os.Exit(1)
	}
}
