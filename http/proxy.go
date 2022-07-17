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
}

func (p *Proxy) nextRequestID() int {
	p.requestIDLock.Lock()
	defer p.requestIDLock.Unlock()
	id := p.requestID
	p.requestID++
	return id
}

func NewProxy(k8sc *k8s.Api) *Proxy {
	return &Proxy{k8sc: k8sc, requestID: 0}
}

func (p *Proxy) handleRequest(req *http.Request, tp *k8s.TargetPod, streamConn httpstream.Connection) (*http.Request, *http.Response, error) {

	requestID := p.nextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", tp.Port))
	headers.Set(v1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
	errorStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating error stream for pod %s -> %d: %v", tp.Name, tp.Port, err)
	}
	// we're not writing to this stream
	errorStream.Close()

	errorChan := make(chan error)
	go func() {
		message, err := ioutil.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- fmt.Errorf("error reading from error stream for pod %s -> %d: %v", tp.Name, tp.Port, err)
		case len(message) > 0:
			errorChan <- fmt.Errorf("an error occurred forwarding on pod %s -> %d: %v", tp.Name, tp.Port, string(message))
		}
		close(errorChan)
	}()

	// create data stream
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating forwarding stream for pod %s -> %d: %v", tp.Name, tp.Port, err)
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
		defer dataStream.Close()

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
		streamConn.Close()
		return nil, nil, err
	}

	return req, resp, nil
}

func (p *Proxy) handleConnection(conn net.Conn, tp *k8s.TargetPod, streamConn httpstream.Connection) {
	defer conn.Close()

	requestID := p.nextRequestID()

	// create error stream
	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", tp.Port))
	headers.Set(v1.PortForwardRequestIDHeader, strconv.Itoa(requestID))
	errorStream, err := streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating error stream for pod %s -> %d: %v", tp.Name, tp.Port, err))
		return
	}
	// we're not writing to this stream
	errorStream.Close()

	errorChan := make(chan error)
	go func() {
		message, err := ioutil.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- fmt.Errorf("error reading from error stream for pod %s -> %d: %v", tp.Name, tp.Port, err)
		case len(message) > 0:
			errorChan <- fmt.Errorf("an error occurred forwarding on pod %s -> %d: %v", tp.Name, tp.Port, string(message))
		}
		close(errorChan)
	}()

	// create data stream
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error creating forwarding stream for pod %s -> %d: %v", tp.Name, tp.Port, err))
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
		defer dataStream.Close()

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
		streamConn.Close()
	}
}

func (p *Proxy) Do(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {

	tp, err := p.k8sc.GetMatchingPodForService(r.Context(), "home-notifier", "hnn-service", "8080")
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

func (p *Proxy) HijackConnect(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
	tp, err := p.k8sc.GetMatchingPodForService(req.Context(), "home-notifier", "hnn-service", "8080")
	if err != nil {
		log.Printf("[INFO] could not get pod %v", err)
		client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
	}

	dialer, err := p.k8sc.Dialer(tp)
	if err != nil {
		log.Printf("[ERROR] could not create dialer: %v", err)
		client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
	}

	con, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		log.Printf("[ERROR] could not dial: %v", err)
		client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
	}
	p.handleConnection(client, tp, con)
}
