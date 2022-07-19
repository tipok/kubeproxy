package cmd

import (
	"context"
	"fmt"
	"github.com/elazarl/goproxy"
	log "github.com/go-pkgz/lgr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	myhttp "github.com/tipok/kubeproxy/http"
	"github.com/tipok/kubeproxy/k8s"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

var listen string

var startProxyCmd = &cobra.Command{
	Use:   "http-proxy",
	Short: "Starting HTTP proxy",
	Long:  `Starting the HTTP proxy.`,
	Run: func(cmd *cobra.Command, args []string) {
		initLogging()
		startProxy()
	},
}

func init() {
	startProxyCmd.PersistentFlags().StringVar(
		&listen,
		"listen",
		":3128",
		"Address to listen on (default :3128)",
	)
	err := viper.BindPFlag("listen", startProxyCmd.PersistentFlags().Lookup("listen"))
	if err != nil {
		log.Printf("[PANIC] could not bind listen flag: %v", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(startProxyCmd)
}

func startProxy() {
	sig := make(chan os.Signal, 1)
	signal.Notify(
		sig,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer signal.Stop(sig)

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

	srv := &http.Server{
		Addr:     listen,
		ErrorLog: log.ToStdLogger(log.Default(), "[ERROR]"),
		Handler:  proxy,
	}

	shuttingDown := false
	go func() {
		log.Printf("[INFO] starting proxy on %s", srv.Addr)
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
