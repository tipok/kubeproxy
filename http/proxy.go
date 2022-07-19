package http

import (
	"bufio"
	"fmt"
	"github.com/elazarl/goproxy"
	log "github.com/go-pkgz/lgr"
	"github.com/tipok/kubeproxy/k8s"
	"io"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/portforward"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
)

type Proxy struct {
	k8sc          *k8s.Api
	requestID     int
	requestIDLock sync.Mutex
	parser        *Parser
}

func (p *Proxy) nextRequestID() int {
	p.requestIDLock.Lock()
	defer p.requestIDLock.Unlock()
	id := p.requestID
	p.requestID++
	return id
}

func NewProxy(k8sc *k8s.Api) *Proxy {
	return &Proxy{k8sc: k8sc, requestID: 0, parser: &Parser{
		ClusterDomain: "cluster.local",
	}}
}

func (p *Proxy) handleRequest(req *http.Request, tp *k8s.TargetPod, streamConn httpstream.Connection) (*http.Request, *http.Response, error) {

	requestID := p.nextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, tp.Port)
	headers.Set(v1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
	errorStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating error stream for pod %s -> %s: %v", tp.Name, tp.Port, err)
	}
	// we're not writing to this stream
	err = errorStream.Close()
	if err != nil {
		log.Printf("[DEBUG] error closing error stream for pod %s -> %s: %v", tp.Name, tp.Port, err)
	}

	errorChan := make(chan error)
	go func() {
		message, err := ioutil.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- fmt.Errorf("error reading from error stream for pod %s -> %s: %v", tp.Name, tp.Port, err)
		case len(message) > 0:
			errorChan <- fmt.Errorf("an error occurred forwarding on pod %s -> %s: %v", tp.Name, tp.Port, string(message))
		}
		close(errorChan)
	}()

	// create data stream
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating forwarding stream for pod %s -> %s: %v", tp.Name, tp.Port, err)
	}

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	var resp *http.Response
	go func() {
		// Copy from the remote side to the local port.
		reader := bufio.NewReader(dataStream)
		resp, err = http.ReadResponse(reader, req)
		if err != nil {
			runtime.HandleError(fmt.Errorf("error copying from remote stream to local connection: %v", err))
		}

		close(remoteDone)
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer func() {
			err := dataStream.Close()
			if err != nil {
				log.Printf("[DEBUG] error closing data stream: %v", err)
			}
		}()

		// Copy from the local port to the remote side.
		reqBytes, err := httputil.DumpRequestOut(req, false)
		if err != nil {
			runtime.HandleError(fmt.Errorf("error dumping request: %v", err))
		}
		if _, err := dataStream.Write(reqBytes); err != nil {
			runtime.HandleError(fmt.Errorf("error copying request: %v", err))
		}
		if _, err := io.Copy(dataStream, req.Body); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from local connection to remote stream: %v", err))
			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	// always expect something on errorChan (it may be nil)
	err = <-errorChan
	if err != nil {
		runtime.HandleError(err)
		err := streamConn.Close()
		if err != nil {
			log.Printf("[DEBUG] error closing stream connection: %v", err)
		}
		return nil, nil, err
	}

	return req, resp, nil
}

func (p *Proxy) handleConnection(conn net.Conn, tp *k8s.TargetPod, streamConn httpstream.Connection) {
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Printf("[DEBUG] could not close connection: %v", err)
		}
	}()

	requestID := p.nextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, tp.Port)
	headers.Set(v1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
	errorStream, err := streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating error stream for pod %s -> %s: %v", tp.Name, tp.Port, err))
		return
	}
	// we're not writing to this stream
	err = errorStream.Close()
	if err != nil {
		log.Printf("[DEBUG] error on closing error stream: %v", err)
	}

	errorChan := make(chan error)
	go func() {
		message, err := ioutil.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- fmt.Errorf("error reading from error stream for pod %s -> %s: %v", tp.Name, tp.Port, err)
		case len(message) > 0:
			errorChan <- fmt.Errorf("an error occurred forwarding on pod %s -> %s: %v", tp.Name, tp.Port, string(message))
		}
		close(errorChan)
	}()

	// create data stream
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating forwarding stream for pod %s -> %s: %v", tp.Name, tp.Port, err))
		return
	}

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// Copy from the remote side to the local port.
		if _, err := io.Copy(conn, dataStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from remote stream to local connection: %v", err))
		}

		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer func() {
			err := dataStream.Close()
			if err != nil {
				log.Printf("[DEBUG] error closing data stream: %v", err)
			}
		}()

		// Copy from the local port to the remote side.
		if _, err := io.Copy(dataStream, conn); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			runtime.HandleError(fmt.Errorf("error copying from local connection to remote stream: %v", err))
			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	// always expect something on errorChan (it may be nil)
	err = <-errorChan
	if err != nil {
		runtime.HandleError(err)
		err := streamConn.Close()
		if err != nil {
			log.Printf("[DEBUG] error closing stream connection: %v", err)
		}
	}
}

func (p *Proxy) getTargetPod(r *http.Request) (*k8s.TargetPod, error) {
	h, err := p.parser.ParseHost(r.Host, r.URL.Scheme == "https")
	if err != nil {
		return nil, fmt.Errorf("could not parse host %w", err)
	}
	if h.Type == "pod" {
		return p.k8sc.GetMatchingPod(r.Context(), h.Namespace, h.Name, h.Port)
	}
	return p.k8sc.GetMatchingPodForService(r.Context(), h.Namespace, h.Name, h.Port)
}

func (p *Proxy) Do(r *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	tp, err := p.getTargetPod(r)
	if err != nil {
		log.Printf("[INFO] could not get pod %v", err)
		return r, nil
	}

	dialer, err := p.k8sc.Dialer(tp)
	if err != nil {
		log.Printf("[ERROR] could not create dialer: %v", err)
		return r, nil
	}

	con, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		log.Printf("[ERROR] could not dial: %v", err)
		return r, nil
	}
	r, resp, err := p.handleRequest(r, tp, con)
	if err != nil {
		log.Printf("[ERROR] could not dial: %v", err)
		return r, nil
	}

	return r, resp
}

func (p *Proxy) HijackConnect(r *http.Request, client net.Conn, _ *goproxy.ProxyCtx) {
	tp, err := p.getTargetPod(r)
	if err != nil {
		log.Printf("[INFO] could not get pod %v", err)
		_, err := client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
		if err != nil {
			log.Printf("[ERROR] could not write to client: %v", err)
		}
		return
	}

	dialer, err := p.k8sc.Dialer(tp)
	if err != nil {
		log.Printf("[ERROR] could not create dialer: %v", err)
		_, err := client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
		if err != nil {
			log.Printf("[ERROR] could not write to client: %v", err)
		}
		return
	}

	con, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		log.Printf("[ERROR] could not dial: %v", err)
		_, err := client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
		if err != nil {
			log.Printf("[ERROR] could not write to client: %v", err)
		}
		return
	}
	p.handleConnection(client, tp, con)
}
