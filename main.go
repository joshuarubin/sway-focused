package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"

	sway "github.com/joshuarubin/go-sway"
	"github.com/joshuarubin/lifecycle"
)

var socketPath string

func init() {
	flag.StringVar(&socketPath, "socketpath", "", "Use the specified socket path")
}

func main() {
	if err := run(); err != nil && !isSignal(err) {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func isSignal(err error, sigs ...os.Signal) bool {
	serr, ok := err.(lifecycle.ErrSignal)
	if !ok {
		return false
	}
	switch serr.Signal {
	case syscall.SIGINT, syscall.SIGTERM:
		return true
	}
	return false
}

func run() error {
	ctx := lifecycle.New(context.Background())

	flag.Parse()

	client, err := sway.New(ctx, sway.WithSocketPath(socketPath))
	if err != nil {
		return err
	}

	n, err := client.GetTree(ctx)
	if err != nil {
		return err
	}

	h := handler{
		EventHandler: sway.NoOpEventHandler(),
		client:       client,
	}

	h.processFocus(ctx, client, n.FocusedNode())

	lifecycle.GoErr(ctx, func() error {
		return sway.Subscribe(ctx, &h, sway.EventTypeWindow)
	})

	return lifecycle.Wait(ctx)
}

type handler struct {
	sway.EventHandler
	client     sway.Client
	isTerminal bool
}

func (h *handler) Window(ctx context.Context, e sway.WindowEvent) {
	if e.Change != "focus" {
		return
	}

	h.processFocus(ctx, h.client, e.Container.FocusedNode())
}

func (h *handler) processFocus(ctx context.Context, client sway.Client, node *sway.Node) {
	if node == nil {
		return
	}

	opt := "''"

	var isTerminal bool

	if node.AppID != nil && *node.AppID == "kitty" {
		isTerminal = true
	}

	if node.WindowProperties != nil && node.WindowProperties.Class == "kitty" {
		isTerminal = true
	}

	if isTerminal == h.isTerminal {
		return
	}

	h.isTerminal = isTerminal

	if !isTerminal {
		opt = "altwin:ctrl_win"
	}

	if _, err := client.RunCommand(ctx, `input '*' xkb_options `+opt); err != nil {
		log.Println(err)
	}
}
