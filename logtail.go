package logtail

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/vogo/logger"
	"github.com/vogo/vogo/vos"
)

// Start parse command config, and start logtail servers with http listener.
func Start() {
	config, err := parseConfig()
	if err != nil {
		fmt.Println(err)
		flag.PrintDefaults()
		os.Exit(1)
	}

	vos.LoadUserEnv()

	// stop exist servers first
	StopLogtail()

	go StartLogtail(config)

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Port), &httpHandler{}); err != nil {
			panic(err)
		}
	}()

	handleSignal()
}

func handleSignal() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	sig := <-signalChan
	logger.Infof("signal: %v", sig)
	StopLogtail()
}
