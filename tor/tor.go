package tor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var noTor = errors.New("Could not find tor try to install it first")
const bootstrapTimeout = 90 * time.Second // wait time for tor circuit
const onionTimeout = 20 * time.Second //wait time for tor writing the hostname file 
type Tor struct{
	cmd       *exec.Cmd
	dataDir   string
	SocksAddr string	
	Onion     string
	stopOnce  sync.Once
}

const EnvBinary = "MOLEWIRE_TOR"

func FindingBinary()(string, error){
	set := os.Getenv(EnvBinary)
	if set != ""{
		info, err := os.Stat(set)
		if err != nil{
			return "", fmt.Errorf("tor: %s points at %s: %w", EnvBinary, set, err)
		}
		if info.IsDir(){
			return "", fmt.Errorf("tor: %s points at a directory: %s", EnvBinary, set)
		}
		return set, nil
	}

	path, err := exec.LookPath("tor")
	if err == nil{
		return path, nil
	}
	// C:\Users\username\Desktop\Tor Browser\Browser\TorBrowser\Tor\tor.exe
	home, _ := os.UserHomeDir()
	tor := []string{"Browser", "TorBrowser", "Tor", "tor.exe"}
	torPaths := []string{
		filepath.Join(append([]string{home, "Desktop", "Tor Browser"}, tor...)...),
		filepath.Join(append([]string{home, "OneDrive", "Desktop", "Tor Browser"}, tor...)...),
		filepath.Join(append([]string{os.Getenv("LOCALAPPDATA"), "Tor Browser"}, tor...)...),
		filepath.Join(append([]string{`C:\Program Files`, "Tor Browser"}, tor...)...),
		"/usr/bin/tor",
		"/usr/local/bin/tor",
		"/opt/homebrew/bin/tor",
		filepath.Join(home, "tor-browser", "Browser", "TorBrowser", "Tor", "tor"),
	}
	for _, g := range torPaths{
		info, err := os.Stat(g)
		if err == nil && !info.IsDir(){
			return g, nil
		}
	}

	return "", noTor
}

func Start(binary string, dataDir string,hsDir string, virtualPort int, localPort int)(*Tor, error){
	socksPort, err := freePort()
	if err != nil{
		return nil, err
	}
	err = os.MkdirAll(dataDir, 0o700)
	if err != nil{
		return nil, err
	}
	torrcPath := filepath.Join(dataDir, "torrc")
	err = writeTorrc(torrcPath, dataDir, hsDir, socksPort, virtualPort, localPort)
	if err != nil{
		return nil, err
	}
	cmd := exec.Command(binary, "-f", torrcPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil{
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	err = cmd.Start()
	if err != nil{
		return nil, fmt.Errorf("tor could not start %s: %w", binary, err)
	}
	t := &Tor{
		cmd:       cmd,
		dataDir:   dataDir,
		SocksAddr: fmt.Sprintf("127.0.0.1:%d", socksPort),
	}
	t.killOnSignal()
	err = t.bootstrap(stdout)
	if err != nil{
		t.Stop()
		return nil, err
	}
	if localPort != 0{
		onion, err := onionAddress(hsDir)
		if err != nil{
			t.Stop()
			return nil, err
		}
		t.Onion = onion
	}

	return t, nil
}

func (t *Tor)Stop()error{
	t.stopOnce.Do(func(){
		if t.cmd == nil || t.cmd.Process == nil{
			return
		}
		t.cmd.Process.Kill()
		t.cmd.Wait()
	})
	return nil
}

func (t *Tor)killOnSignal(){
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func(){
		<-ch
		t.Stop()
		os.Exit(1)
	}()
}

func (t *Tor)bootstrap(stdout io.ReadCloser)error{
	circuitError := make(chan error, 1)
	go func(){
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan(){
			line := scanner.Text()
			if strings.Contains(line,"Bootstrapped 100%"){
				circuitError <- nil
				return
			}
		}
		circuitError <- fmt.Errorf("tor exited mid bootstrappping")
	}()
	select{
	case err := <-circuitError:
		return err
	case <-time.After(bootstrapTimeout):
		return fmt.Errorf("tor gave up after %v", bootstrapTimeout)
	}
}

//tor generating hidden service hostname file
func onionAddress(hsDir string)(string, error){
	path := filepath.Join(hsDir, "hostname")
	waittime := time.Now().Add(onionTimeout)
	for time.Now().Before(waittime){
		link , err := os.ReadFile(path)
		if err == nil{
			name := strings.TrimSpace(string(link))
			if name != ""{
				return name, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("no onion link at %s after %v", path, onionTimeout)
}

//torrc build configs
func writeTorrc(path string, dataDir string, hsDir string, socksPort int, virtualPort int, localPort int)error{
	var b strings.Builder
	fmt.Fprintf(&b, "SocksPort %d\n", socksPort)
	fmt.Fprintf(&b, "DataDirectory %q\n", filepath.ToSlash(dataDir))
	fmt.Fprintf(&b, "Log notice stdout\n")
	
	if localPort != 0{	//prevent receiver
		fmt.Fprintf(&b, "HiddenServiceDir %q\n", filepath.ToSlash(hsDir))
		fmt.Fprintf(&b, "HiddenServicePort %d 127.0.0.1:%d\n",virtualPort,localPort)
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func freePort()(int, error){
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil{
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
