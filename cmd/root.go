package cmd

import (
	"fmt"
	log "github.com/go-pkgz/lgr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

var rootCmd = &cobra.Command{
	Use:   "kubeproxy",
	Short: "A HTTP proxy accessing kubernetes resources",
	Long:  `A HTTP proxy allowing accessing pods as you where in the cluster.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := viper.SafeWriteConfig(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var cfgFile string
var kubeconfig string
var clusterDomain string
var debug bool

func initLogging() {
	logOpts := []log.Option{log.Debug, log.Msec, log.LevelBraces, log.CallerFile, log.CallerFunc}
	if debug {
		logOpts = append(logOpts, log.Debug)
	}
	log.Setup(logOpts...)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"config file (default is $HOME/.kubeproxy/config.yaml)",
	)
	rootCmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")

	defaultKubeConfig := ""
	home := homedir.HomeDir()
	if home != "" {
		defaultKubeConfig = filepath.Join(home, ".kube", "config")
	}

	rootCmd.PersistentFlags().StringVar(
		&kubeconfig,
		"kubeconfig",
		defaultKubeConfig,
		"K8s configuration file (default is $HOME/.kube/config)",
	)
	rootCmd.PersistentFlags().StringVar(
		&clusterDomain,
		"cluster-domain",
		"cluster.local",
		"K8s cluster domain, this domain will be used to identify cluster requests (default is cluster.local)",
	)
	err := viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	if err != nil {
		log.Printf("[PANIC] could not bind kubeconfig flag: %v", err)
		os.Exit(1)
	}
	err = viper.BindPFlag("cluster-domain", rootCmd.PersistentFlags().Lookup("cluster-domain"))
	if err != nil {
		log.Printf("[PANIC] could not bind cluster-domain flag: %v", err)
		os.Exit(1)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home := homedir.HomeDir()
		configPath := filepath.Join(home, ".config", "kubeproxy")
		viper.AddConfigPath(configPath)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Println("Can't read config:", err)
			os.Exit(1)
		}
	}
}
