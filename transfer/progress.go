package transfer

import (
	"fmt"
	"sync/atomic"
	"time"
)
const progressSpinner = 100 * time.Millisecond

func startProgress(action string, done string, fileSize uint64, counter *atomic.Uint64)func(){
	stop := make(chan struct{})
	finished := make(chan struct{})
	go func(){
		defer close(finished)
		ticker := time.NewTicker(progressSpinner)
		defer ticker.Stop()
		chars := []string{"|", "/", "-", "\\"}
		i := 0
		lastBytes := uint64(0)
		lastTime := time.Now()
		began := time.Now()
		speed := 0.0
		for{
			select{
			case <-stop:
				if done == ""{
					fmt.Printf("\r%-60s\r", "")
					return
				}
				avg := 0.0
				spent := time.Since(began).Seconds()
				if spent > 0{
					avg = float64(counter.Load()) / 1024 / spent
				}
				fmt.Printf("\r%-60s\n", fmt.Sprintf("%s %.0f kb/s avg", done, avg))
				return
			case <-ticker.C:
				got := counter.Load()
				lastestTime := time.Since(lastTime)
				if lastestTime >= time.Second{
					speed = float64(got - lastBytes) / 1024 / lastestTime.Seconds()
					lastBytes = got
					lastTime = time.Now()
				}
				percent := uint64(0)
				if fileSize > 0{
					percent = got * 100 / fileSize
				}
				line := fmt.Sprintf("%s %d%% %.0f kb/s %s", action, percent, speed, chars[i])
				fmt.Printf("\r%-60s", line)
				i = (i + 1) % len(chars)
			}
		}
	}()
	return func(){
		close(stop)
		<-finished
	}
}
