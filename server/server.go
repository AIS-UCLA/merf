package server

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"sync"
)

var (
	flags    = flag.NewFlagSet("server", flag.ExitOnError)
	httpPort = flags.Int("http_port", 8000, "address to server http on")
	merfPort = flags.Int("merf_port", 1337, "address to listen for clients on")
	domain   = flags.String("domain", "example.com", "base domain to serve on")
	status   = flags.String("template", "server/index.html", "path to status page template file")

	iupac = []string{"mono", "di", "tri", "tetra", "penta", "hexa", "hepta",
		"octa", "nona", "deca", "undeca", "dodeca", "trideca",
		"tetradeca", "pentadeca", "hexadeca", "heptadeca",
		"octadeca", "nonadeca", "icosa", "henicosa", "docosa",
		"tricoda", "tetracosa", "pentacosa", "hexacosa",
		"heptacosa", "octacosa", "nonacosa"}
	nato = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot",
		"golf", "hotel", "india", "juliette", "kilo", "lima",
		"mike", "november", "oscar", "papa", "quebec", "romeo",
		"sierra", "tango", "uniform", "victor", "whiskey",
		"xray", "yankee", "zulu"}
	stars = []string{"alpheratz", "ankaa", "schedar", "diphda", "achernar",
		"hamal", "acamar", "menkak", "mirfak", "aldebaran",
		"rigel", "capella", "bellatrix", "elnath", "alnilam",
		"betelgeuse", "canopus", "sirius", "adhara", "procyon",
		"pollux", "avior", "suhail", "miaplacidus", "alphard",
		"regulus", "dubhe", "denebola", "gienah", "acrux",
		"gacrux", "alioth", "spica", "alkaid", "hadar",
		"menkent", "rigil-kentaurus", "arcturus",
		"zubenelgenubi", "kochab", "alphecca", "antares",
		"atria", "sabik", "shaula", "rasalhague", "eltanin",
		"kaus-australis", "vega", "nunki", "altair", "peacock",
		"deneb", "enif", "al-nair", "fomalhaut", "markab",
		"polaris"}
	diplomacy = []string{"ankara", "belgium", "berlin", "brest", "budapest",
		"bulgaria", "constantinople", "denmark", "edinburgh",
		"greece", "holland", "kiel", "liverpool", "london",
		"marseilles", "moscow", "munich", "naples", "norway",
		"paris", "portugal", "rome", "rumania", "stp",
		"serbia", "sevastopol", "smryna", "spain", "sweden",
		"trieste", "tunis", "venice", "vienna", "warsaw",
		"albania", "apulia", "bohemia", "burgundy", "clyde",
		"finland", "galicia", "gascony", "livonia", "picardy",
		"piedmont", "prussia", "ruhr", "silesia", "syria",
		"tuscany", "tyrolia", "ukraine", "wales", "yorkshire"}
)

var status_tmpl *template.Template

func usage() {
	fmt.Fprintf(os.Stderr, "usage: merf server [options]\n")
	flags.PrintDefaults()
	os.Exit(1)
}

type MerfConn struct {
	conn net.Conn
	bufr *bufio.Reader
}

func NewMerfConn(c net.Conn) MerfConn {
	return MerfConn{
		conn: c,
		bufr: bufio.NewReader(c),
	}
}

func (mc MerfConn) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Write(mc.conn); err != nil {
		return nil, err
	}

	resp, err := http.ReadResponse(mc.bufr, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type MerfServer struct {
	clients map[string]MerfConn
	mu      sync.RWMutex
	domain  string
}

func NewMerfServer(domain string) *MerfServer {
	return &MerfServer{
		clients: make(map[string]MerfConn),
		domain:  domain,
	}
}

func (m *MerfServer) RegisterClient(conn net.Conn) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	subdomain := fmt.Sprintf("%s-%s-%s-%s", iupac[rand.Intn(len(iupac))],
		nato[rand.Intn(len(nato))],
		stars[rand.Intn(len(stars))],
		diplomacy[rand.Intn(len(diplomacy))])
	m.clients[subdomain+"."+*domain] = NewMerfConn(conn)
	log.Printf("INFO: Client registered for subdomain: %s", subdomain)
	return subdomain
}

func (m *MerfServer) HandleClient(conn net.Conn) {
	subdomain := m.RegisterClient(conn)
	conn.Write([]byte(subdomain + "." + *domain + "\n"))
}

func (m *MerfServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host == *domain {
		err := status_tmpl.Execute(w, m.clients)
		if err != nil {
			log.Printf("ERR: failed to parse status page: %v", err)
		}
		return
	}

	var mc MerfConn
	found := false
	hostname := r.Host
	m.mu.Lock()
	for {
		if c, exists := m.clients[hostname]; exists {
			mc = c
			found = true
			break
		}

		if dot := strings.Index(hostname, "."); dot != -1 {
			hostname = hostname[dot+1:]
		} else {
			break
		}
	}
	m.mu.Unlock()

	if !found {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "http"
			pr.Out.URL.Host = r.Host
		},
		Transport: mc,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			m.mu.Lock()
			delete(m.clients, hostname)
			m.mu.Unlock()
			mc.conn.Close()
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			log.Printf("LOG: failed connection to client for subdomain '%s': %v", hostname, err)
		},
	}

	proxy.ServeHTTP(w, r)
}

func Main() {
	flags.Usage = usage
	flags.Parse(os.Args[2:])

	var err error
	status_tmpl, err = template.ParseFiles(*status)
	if err != nil {
		log.Fatalf("ERR: failed to open status template: %v", err)
	}

	log.Printf("staring http server on :%d, listening for clients on :%d\n", *httpPort, *merfPort)

	m := NewMerfServer(*domain)

	go func() {
		l, err := net.Listen("tcp4", fmt.Sprint(":", *merfPort))
		if err != nil {
			log.Fatal(err)
		}
		defer l.Close()

		for {
			c, err := l.Accept()
			if err != nil {
				log.Println("WARN: failed to accept client:", err)
				continue
			}
			go m.HandleClient(c)
		}
	}()

	log.Fatal("ERR:", http.ListenAndServe(fmt.Sprint(":", *httpPort), m))
}
