package client

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
)

var (
	flags  = flag.NewFlagSet("client", flag.ExitOnError)
	remote = flags.String("remote", "merf.ais-ucla.org:1337", "server (and port) to connect to")
	local  = flags.String("local", "http://localhost:8000", "address to serve")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: merf client [options]\n")
	flags.PrintDefaults()
	os.Exit(1)
}

type MerfClient struct {
	domain     string
	remoteConn net.Conn
	remoteRead *bufio.Reader
	localURL   *url.URL
	client     http.Client
}

func newMerfClient(remote string, local *url.URL) (*MerfClient, error) {
	r, err := net.Dial("tcp", remote)
	if err != nil {
		return nil, err
	}

	remoteBuf := bufio.NewReader(r)
	domain, err := remoteBuf.ReadString('\n')
	domain = domain[:len(domain)-1]
	if err != nil {
		r.Close()
		return nil, err
	}

	return &MerfClient{
		domain:     domain,
		remoteConn: r,
		remoteRead: remoteBuf,
		localURL:   local,
	}, nil
}

func (mc MerfClient) run() error {
	for {
		req, err := http.ReadRequest(mc.remoteRead)
		if err != nil {
			return err
		}
		// rewrite request
		req.URL.Scheme = mc.localURL.Scheme
		req.URL.Host = mc.localURL.Host
		req.URL.Path = req.RequestURI
		req.RequestURI = ""
		resp, err := mc.client.Do(req)
		if err != nil {
			return err
		}
		resp.Write(mc.remoteConn)
	}
}

func Main() {
	flags.Usage = usage
	flags.Parse(os.Args[2:])

	localURL, err := url.Parse(*local)
	if err != nil {
		log.Fatal(err)
	}
	client, err := newMerfClient(*remote, localURL)

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("connection established, serving %s at %s", *local, client.domain)

	log.Fatal(client.run())
}
