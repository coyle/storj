// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/coyle/storj/pkg/kademlia"

	readline "gopkg.in/readline.v1"
)

var (
	ip    string
	port  string
	bIP   string
	bPort string
	help  bool
	stun  bool
)

func initializeFlags() {
	flag.StringVar(&ip, "ip", "0.0.0.0", "IP Address to use")
	flag.StringVar(&port, "port", "0", "Port to use")
	flag.StringVar(&bIP, "bip", "", "IP Address to bootstrap against")
	flag.StringVar(&bPort, "bport", "", "Port to bootstrap against")
	flag.BoolVar(&help, "help", false, "Display Help")
	flag.BoolVar(&stun, "stun", true, "Use STUN")

	flag.Parse()

	if help {
		displayFlagHelp()
		os.Exit(0)
	}

	if ip == "" {
		displayFlagHelp()
		os.Exit(0)
	}

	if port == "" {
		displayFlagHelp()
		os.Exit(0)
	}
}

func main() {

	initializeFlags()

	var bootstrapNodes []*kademlia.NetworkNode
	if bIP != "" || bPort != "" {
		bootstrapNodes = append(bootstrapNodes, kademlia.NewNetworkNode(bIP, bPort))
	}

	dht, err := kademlia.NewDHT(&kademlia.MemoryStore{}, &kademlia.Options{
		BootstrapNodes: bootstrapNodes,
		IP:             ip,
		Port:           port,
		UseStun:        stun,
	})

	fmt.Println("Opening socket..")

	if stun {
		fmt.Println("Discovering public address using STUN..")
	}

	if err = dht.CreateSocket(); err != nil {
		fmt.Printf("error creating socket %s\n", err)
		os.Exit(1)
	}

	fmt.Println("..done")

	e := make(chan error)
	go func(err chan<- error) {
		fmt.Println("Now listening on " + dht.GetNetworkAddr())
		err <- dht.Listen()
	}(e)

	if len(bootstrapNodes) > 0 {
		fmt.Println("Bootstrapping..")
		dht.Bootstrap()
		fmt.Println("..done")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func(err chan<- error) {
		for range c {
			err <- dht.Disconnect()

		}
	}(e)

	rl, err := readline.New("> ")
	if err != nil {
		fmt.Printf("Got error from readline.New('> '): %s\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF, readline.ErrInterrupt
			break
		}

		input := strings.Split(line, " ")
		switch input[0] {
		case "help":
			displayHelp()
		case "store":
			if len(input) != 2 {
				displayHelp()
				continue
			}
			id, err := dht.Store([]byte(input[1]))
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Println("Stored ID: " + id)
		case "get":
			if len(input) != 2 {
				displayHelp()
				continue
			}
			data, exists, err := dht.Get(input[1])
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Println("Searching for", input[1])
			if exists {
				fmt.Println("..Found data:", string(data))
			} else {
				fmt.Println("..Nothing found for this key!")
			}
		case "info":
			nodes := dht.NumNodes()
			self := dht.GetSelfID()
			addr := dht.GetNetworkAddr()
			fmt.Println("Addr: " + addr)
			fmt.Println("ID: " + self)
			fmt.Println("Known Nodes: " + strconv.Itoa(nodes))
		}
	}
}

func displayFlagHelp() {
	fmt.Println(`cli-example

Usage:
	cli-example --port [port]

Options:
	--help Show this screen.
	--ip=<ip> Local IP [default: 0.0.0.0]
	--port=[port] Local Port [default: 0]
	--bip=<ip> Bootstrap IP
	--bport=<port> Bootstrap Port
	--stun=<bool> Use STUN protocol for public addr discovery [default: true]`)
}

func displayHelp() {
	fmt.Println(`
help - This message
store <message> - Store a message on the network
get <key> - Get a message from the network
info - Display information about this node
	`)
}
