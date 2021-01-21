package main

import (
    "context"
	"flag"
	"fmt"
	"time"
	"os"
	"os/signal"

	"github.com/rmedvedev/grpcdump/internal/app/filter"
	"github.com/rmedvedev/grpcdump/internal/app/httpparser"
	"github.com/rmedvedev/grpcdump/internal/app/models"
	"github.com/rmedvedev/grpcdump/internal/app/packetprovider"
	"github.com/rmedvedev/grpcdump/internal/app/protoprovider"
	"github.com/rmedvedev/grpcdump/internal/app/renderers"
	"github.com/rmedvedev/grpcdump/internal/pkg/config"
	"github.com/rmedvedev/grpcdump/internal/pkg/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	flag.Parse()

	config.Init()
	cfg := config.GetConfig()

	err := protoprovider.Init(cfg.ProtoPaths, cfg.ProtoFiles)
	if err != nil {
		logrus.Fatal("Proto files init error: ", err)
	}

	err = logger.Init(config.GetConfig().LoggerLevel)
	if err != nil {
		logrus.Fatal("Logger init error: ", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	for _, port := range config.GetConfig().Ports {
	    go kickoffGrpcdump(ctx, config.GetConfig().Iface, port)
	}
	// Wait for SIGINT.
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    <-sig
    // Terminate all go routines
	cancel()
}

func kickoffGrpcdump(ctx context.Context, iface string, port uint) {
    logrus.Infof("Starting sniff ethernet packets at interface %s on port %d", iface, port)

	provider, err := packetprovider.NewEthernetProvider(iface)
	if err != nil {
		logrus.Fatal("Error to create packet provider", err)
	}

	packetFilter := filter.New()
	packetFilter.SetPort(uint32(port))

	err = provider.SetFilter(packetFilter)
	if err != nil {
		logrus.Fatal("Error to create filter", err)
	}

	modelsCh := make(chan models.RenderModel, 1)
	go renderOutput(modelsCh)
	httpParser := httpparser.New(&modelsCh)
	packets := provider.GetPackets()

	for {
		select {
		case packet := <-packets:
			if packet == nil {
				return
			}
			err = httpParser.Parse(packet)
			if err != nil {
				logrus.Warning(err)
			}
		case <- ctx.Done():
		    return
		}
	}
}

func renderOutput(models chan models.RenderModel) {
	renderer := renderers.GetApplicationRenderer()
	for {
		select {
		case model := <-models:
            fmt.Println(time.Now().Format("2006-01-02 15:04:05.000000") + renderer.Render(model))
		}
	}
}
