package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type RequestItem struct {
	RequestType  string `json:"request_type"`
	RequestData  string `json:"request_data"`
	ResponseType string `json:"response_type"`
	ResponseData string `json:"response_data"`
	ByePacket    bool   `json:"bye_packet"`
}

type MockTCPConfig struct {
	Mode        string        `json:"mode"`
	Host        string        `json:"host"`
	Port        int32         `json:"port"`
	DumpRequest bool          `json:"dump_request"`
	Requests    []RequestItem `json:"requests"`
}

var (
	reqID              int64
	dumpDir            = "./dump"
	tcpListenerChannel = make(chan *net.TCPListener, 64)
	tcpConnChannel     = make(chan *net.TCPConn, 64)
)

func main() {

	log.SetOutput(os.Stdout)

	var configFile string
	var viaHTTP bool
	short := " (shorthand)"
	configfileUsage := "the mock tcp server/client config file"
	configureViaHTTP := "start http server on port 8008"
	flag.StringVar(&configFile, "config", "", configfileUsage)
	flag.StringVar(&configFile, "c", "", configfileUsage+short)
	flag.BoolVar(&viaHTTP, "h", false, configureViaHTTP)

	flag.Parse()

	if viaHTTP {
		httpApp()
	} else {
		consoleApp(configFile)
	}

}

func consoleApp(configFile string) {
	if configFile == "" {
		configFile = "mocktcp.conf"
	}

	conf := MockTCPConfig{}

	ioFile, e := os.Open(configFile)
	if e != nil {
		log.Fatalln(e)
		return
	}
	decoder := json.NewDecoder(ioFile)
	err := decoder.Decode(&conf)
	if err != nil {
		log.Fatalln(e)
		return
	}

	if conf.Mode == "server" {
		startServer(conf)
	} else if conf.Mode == "client" {
		startClient(conf)
	}
}

func setup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	decoder := json.NewDecoder(r.Body)
	conf := MockTCPConfig{}
	err := decoder.Decode(&conf)
	if err != nil {
		panic(err)
	}
	log.Println(conf.Mode)

	if conf.Mode == "server" {
		go startServer(conf)
	} else if conf.Mode == "client" {
		go startClient(conf)
	}

}

func reset(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	loop := true
	for loop {
		select {
		case x, ok := <-tcpListenerChannel:
			if ok {
				log.Printf("Closing TCP Listener %s.\n", x.Addr().String())
				x.Close()
			} else {
				log.Println("Channel closed!")
				loop = false
			}
		default:
			log.Println("No value ready, moving on.")
			loop = false
		}
	}
	loop = true
	for loop {
		select {
		case x, ok := <-tcpConnChannel:
			if ok {
				log.Printf("Closing TCP Connection %s.\n", x.RemoteAddr().String())
				x.Close()
			} else {
				log.Println("Channel closed!")
				loop = false
			}
		default:
			log.Println("No value ready, moving on.")
			loop = false
		}
	}

}

func httpApp() {
	router := httprouter.New()
	router.POST("/mock/setup", setup)
	router.POST("/mock/reset", reset)

	log.Fatalln(http.ListenAndServe(":8008", router))

}
func startClient(conf MockTCPConfig) {

	var tcpAddr *net.TCPAddr
	if addr, e := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", conf.Host, conf.Port)); e != nil {
		log.Fatalln(e)
		return
	} else {
		tcpAddr = addr
	}

	var tcpConn *net.TCPConn
	if conn, e := net.DialTCP("tcp4", nil, tcpAddr); e != nil {
		log.Fatalln(e)
		return
	} else {
		tcpConn = conn
	}
	tcpConnChannel <- tcpConn

	if e := createDumpDir(conf); e != nil {
		log.Fatal(e)
		return
	}

	log.Printf("Sending to:%s:%d\n", conf.Host, conf.Port)

	processClientConnection(tcpConn, conf)
}

func startServer(conf MockTCPConfig) {

	var tcpAddr *net.TCPAddr
	if addr, e := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", conf.Host, conf.Port)); e != nil {
		log.Fatalln(e)
		return
	} else {
		tcpAddr = addr
	}

	var tcpListener *net.TCPListener
	if listener, e := net.ListenTCP("tcp4", tcpAddr); e != nil {
		log.Fatalln(e)
		return
	} else {
		tcpListener = listener
	}
	tcpListenerChannel <- tcpListener

	if e := createDumpDir(conf); e != nil {
		log.Fatal(e)
		return
	}

	log.Printf("Listening:%s:%d\n", conf.Host, conf.Port)

	for {
		if conn, e := tcpListener.Accept(); e != nil {
			log.Println(e)
			break
		} else {
			go processServerConnection(conn, conf)
		}
	}
}

func processClientConnection(conn *net.TCPConn, conf MockTCPConfig) {
	defer conn.Close()
	atomic.AddInt64(&reqID, 1)

	log.Printf("[%d]connected to server: %s\n", reqID, conn.RemoteAddr().String())

	var buf [2048]byte
	var e error
	var l int

	for _, requestItem := range conf.Requests {
		matched := false
		var bPayload []byte
		if requestItem.RequestType == "string" {
			bPayload = []byte(requestItem.RequestData)
		} else if requestItem.RequestType == "byte" {
			bPayload, e = hex.DecodeString(requestItem.RequestData)
			if e != nil {
				log.Fatalln(e)
				return
			}
		} else {
			log.Fatalln("unknown request item type, only could be [string|byte]")
			return
		}

		l, e = conn.Write(bPayload)
		if e != nil {
			log.Fatalln(e)
			return
		}
		log.Printf("  sent %d bytes\n", l)

		l, e = conn.Read(buf[0:])
		if e != nil {
			log.Fatalln(e)
			return
		}
		log.Printf("  received %d bytes\n", l)

		if conf.DumpRequest {
			filename := fmt.Sprintf("%s/%d.dat", dumpDir, reqID)
			ioutil.WriteFile(filename, buf[0:l], 0666)
		}

		if requestItem.ResponseType == "string" {
			if strings.Contains(string(buf[0:l]), requestItem.ResponseData) {
				matched = true
			}
		} else if requestItem.ResponseType == "byte" {
			bPayload, e = hex.DecodeString(requestItem.ResponseData)
			if e != nil {
				log.Fatalln(e)
				return
			} else if bytes.Contains(buf[0:l], bPayload) {
				matched = true
			}
		} else {
			log.Fatalln("unknown response item type, only could be [string|byte]")
			return
		}

		if matched {
			log.Printf("  [matched %s:%s] %s\n", requestItem.ResponseType, requestItem.ResponseData, requestItem.RequestData)
		} else {
			fmt.Println("no match.")
			os.Exit(-1)
		}
		log.Printf("  --------\n")
	}
	fmt.Println("all requests were sent.")
}

func processServerConnection(conn net.Conn, conf MockTCPConfig) {
	defer conn.Close()
	atomic.AddInt64(&reqID, 1)

	log.Printf("[%d]client connected: %s\n", reqID, conn.RemoteAddr().String())

	var buf [2048]byte
	var bPayload []byte
	var l int
	var e error

	for l, e = conn.Read(buf[0:]); l > 0; l, e = conn.Read(buf[0:]) {
		if e != nil {
			log.Fatalln(e)
			return
		}
		log.Printf("  received %d bytes\n", l)

		if conf.DumpRequest {
			filename := fmt.Sprintf("%s/%d.dat", dumpDir, reqID)
			ioutil.WriteFile(filename, buf[0:l], 0666)
		}
		matched := false
		for _, requestItem := range conf.Requests {
			if requestItem.RequestType == "string" {
				if strings.Contains(string(buf[0:l]), requestItem.RequestData) {
					matched = true
				}
			} else if requestItem.RequestType == "byte" {
				bPayload, e = hex.DecodeString(requestItem.RequestData)
				if e != nil {
					log.Fatalln(e)
					return
				} else if bytes.Contains(buf[0:l], bPayload) {
					matched = true
				}
			} else {
				log.Fatalln("unknown request item type, only could be [string|byte]")
				return
			}

			if matched {
				log.Printf("  [matched %s:%s] %s\n", requestItem.RequestType, requestItem.RequestData, requestItem.ResponseData)

				if requestItem.ResponseType == "string" {
					bPayload = []byte(requestItem.ResponseData)
				} else if requestItem.ResponseType == "byte" {
					bPayload, e = hex.DecodeString(requestItem.ResponseData)
					if e != nil {
						log.Fatalln(e)
						return
					}
				} else {
					log.Fatalln("unknown response item type, only could be [string|byte]")
					return
				}
				l, e := conn.Write(bPayload)
				if e != nil {
					log.Fatalln(e)
					return
				}
				log.Printf("  sent %d bytes\n", l)

				if requestItem.ByePacket {
					os.Exit(0)
				}
				break
			}
		}
		if !matched {
			fmt.Println("nothing matched.")
		}
		log.Printf("  --------\n")
	}
}

func createDumpDir(conf MockTCPConfig) error {
	now := time.Now()
	if conf.DumpRequest {
		dumpDir = fmt.Sprintf("./dump/%d", now.UnixNano())
		log.Printf("[dump dir]: %s\n", dumpDir)
		if !isDirExist(dumpDir) {
			if e := os.MkdirAll(dumpDir, os.ModePerm); e != nil {
				log.Fatal(e)
				return e
			}
		}
	}
	return nil
}

func isDirExist(path string) bool {
	fi, err := os.Stat(path)

	if err != nil {
		return os.IsExist(err)
	} else {
		return fi.IsDir()
	}
}
