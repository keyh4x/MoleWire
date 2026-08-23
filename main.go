package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"MoleWire/tor"
	"MoleWire/transfer"
)


const connectionAttempts = 3
const connectionDelays = 10 * time.Second
const burrowAttempts = 40
const burrowWait = 3 * time.Second

func torDataDir(role string)(string, error){
	base, err := os.UserConfigDir()
	if err != nil{
		return "", err
	}
	return filepath.Join(base, "molewire", "tor", role), nil
} // base/molewire/tor/send or get

func newHSDir()(string, error){
	base, err := os.UserConfigDir()
	if err != nil{
		return "", err
	}
	// base/molewire/tor/onions/hs(links)
	parent := filepath.Join(base, "molewire", "tor", "onions")
	err = os.MkdirAll(parent, 0o700)
	if err != nil{
		return "", err
	}
	return os.MkdirTemp(parent, "hs-*")
}

func onionCheck(t *tor.Tor, port int)error{
	onion := fmt.Sprintf("%s:%d", t.Onion, port)
	chars := []string{"|", "/", "-", "\\"}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func(){
		defer close(finished)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		widest := 0
		for{
			select{
			case <-done:
				fmt.Printf("\r%s\r", strings.Repeat(" ", widest))
				return
			case <-ticker.C:
				line := fmt.Sprintf("hosting the file  %s", chars[i % len(chars)])
				fmt.Printf("\r%s", line)
				if len(line) > widest{
					widest = len(line)
				}
				i++
			}
		}
	}()
	defer func(){
		close(done)
		<-finished
	}()
	for i:= 0; i < burrowAttempts;i++{
		conn, err := t.ConnectOnion(onion)//checking if onion reachable
		if err == nil{
			conn.Close()
			return nil
		}
		time.Sleep(burrowWait)
	}
	return fmt.Errorf("tor could not host file")
}

func sizeConversion(n uint64)string{
	if n >= 1<<20{
		return fmt.Sprintf("%d MB", n/(1<<20))
	}
	if n >= 1<<10{
		return fmt.Sprintf("%d KB", n/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

func triggerSend(args []string)error{
	if len(args) < 1{
		return fmt.Errorf("send needs a file path :D")
	}
	offer, err := transfer.OpenFile(args[0])
	if err != nil{
		return err
	}
	defer offer.Close()
	binary, err := tor.FindingBinary()
	if err != nil{
		return err
	}

	dataDir, err := torDataDir("send")
	if err != nil{
		return err
	}
	hsDir, err := newHSDir()
	if err != nil{
		return err
	}
	defer os.RemoveAll(hsDir)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:0"))
	if err != nil{
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	defer ln.Close()
	fmt.Printf("waking up the mole...\n")
	t, err := tor.Start(binary, dataDir, hsDir, port, port)
	if err != nil{
		return err
	}
	defer t.Stop()
	err = onionCheck(t, port)
	if err != nil{
		return err
	}
	hash := hex.EncodeToString(offer.Hash[:])
	fmt.Printf("Open to serve file now\n")
	fmt.Printf("File name :%s\nFile size :%s\nFile hash :%s\nHost link :%s:%d\nShare this :%s:%d %s\n",offer.Name,sizeConversion(offer.Size),hash,t.Onion,port,t.Onion,port,hash)
	for{
		conn, err := ln.Accept()
		if err != nil{
			return err
		}
		go func(c net.Conn){
			defer c.Close()
			s := offer.Sender(c)
			err := s.Serve()
		
			if !s.MoleAccepted{
				return
			}
			if err != nil{
				log.Printf("burrow collapsed: %v", err)
				return
			}
			log.Printf(" transfer completed successfully | all dig requests were answered")
		}(conn)
	}
}

func triggerGet(args []string)error{
	if len(args) < 3{
		return fmt.Errorf("usage : ./MoleWire get <host:port> <hash> <download path>")
	} 
	hash, err := hex.DecodeString(args[1])
	if err != nil{
		return fmt.Errorf("Not a real hash: %w", err)
	}
	if len(hash) != 32{
		return fmt.Errorf("Hash should be 64 chars got %d", len(args[1]))
	}
	var want [32]byte
	copy(want[:], hash)
	downloadPath := args[2]
	info, err := os.Stat(downloadPath)
	if err != nil{
		return fmt.Errorf("Cant find that folder: %s", downloadPath)
	}
	if !info.IsDir(){
		return fmt.Errorf("%s is a file. need a folder", downloadPath)
	}

	binary, err := tor.FindingBinary()
	if err != nil{
		return err
	}
	dataDir, err := torDataDir("get")
	if err != nil{
		return err
	}
	fmt.Printf("waking the mole give it a sec...\n")
	t, err := tor.Start(binary, dataDir, "", 0, 0)
	if err != nil{
		return err
	}
	defer t.Stop()
	var conn net.Conn
	for attempt := 1; attempt <= connectionAttempts; attempt++{
		conn, err = t.ConnectOnion(args[0])
		if err == nil{
			break
		}
		if attempt == connectionAttempts{
			return err
		}
		fmt.Printf("burrow not open yet digging again\n")
		time.Sleep(connectionDelays)
	}
	defer conn.Close()
	fmt.Printf("connected sending dig requests\n")
	err = transfer.NewReceiver(conn, want, downloadPath).Fetch()
	if err != nil{
		return err
	}
	return nil
}

func usage(){
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, " ./MoleWire send <file path>")
	fmt.Fprintln(os.Stderr, " ./MoleWire get <onion:port> <hash> <download path>")
}

func main(){
	log.SetFlags(0)          
	log.SetPrefix("MoleWire:")  
	args := os.Args
	if len(args) < 2{
		usage()
		os.Exit(1)
	}
	switch args[1]{
	case "send":
		err := triggerSend(args[2:])
		if err != nil{
			log.Fatal(err)
		}
	case "get":
		err := triggerGet(args[2:])
		if err != nil{
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(1)
	}
}
