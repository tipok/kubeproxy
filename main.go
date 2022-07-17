package main

import (
	"context"
	"fmt"
	"github.com/elazarl/goproxy"
	log "github.com/go-pkgz/lgr"
	myhttp "github.com/tipok/kubeproxy/http"
	"github.com/tipok/kubeproxy/k8s"
	"k8s.io/client-go/util/homedir"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

func main() {
	logOpts := []log.Option{log.Debug, log.Msec, log.LevelBraces, log.CallerFile, log.CallerFunc}
	if true {
		logOpts = append(logOpts, log.Debug)
	}
	log.Setup(logOpts...)
	sig := make(chan os.Signal, 1)
	signal.Notify(
		sig,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer signal.Stop(sig)

	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	clusterDomain := "cluster.local"

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	k8sc, err := k8s.New(kubeconfig)
	if err != nil {
		log.Fatalf("[PANIC] could not create k8s client %v", err)
	}

	clusterRegEx, err := regexp.Compile(fmt.Sprintf("^.*\\.%s:?\\d*", strings.Replace(clusterDomain, ".", "\\.", -1)))
	if err != nil {
		log.Fatalf("[PANIC] could not compile cluster regex: %v", err)
	}
	p := myhttp.NewProxy(k8sc)
	onReq := proxy.OnRequest(goproxy.ReqHostMatches(clusterRegEx))
	onReq.HijackConnect(p.HijackConnect)
	onReq.DoFunc(p.Do)

	tp, err := k8sc.GetMatchingPodForService(context.TODO(), "home-notifier", "hnn-service", "8080")
	if err != nil {
		log.Fatalf("[PANIC] could not get pod %v", err)
	}

	log.Printf("[INFO] pod: %s", tp.Name)

	listen := ":8080"
	srv := &http.Server{
		Addr:     listen,
		ErrorLog: log.ToStdLogger(log.Default(), "[ERROR]"),
		Handler:  proxy,
	}

	shuttingDown := false
	go func() {
		log.Printf("[INFO] listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil {
			if !shuttingDown {
				log.Fatalf("[PANIC] while listening to %s: %v", listen, err)
			}
			log.Printf("[INFO] shutting down")
		}
	}()

	<-sig

	shuttingDown = true
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("[ERROR] during shutdown: %v", err)
	}
}
