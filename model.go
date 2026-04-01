package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type Settings struct {
	Address   string `json:"address"`
	Duration  int    `json:"duration_time_seconds"`
	Start     int    `json:"start_port"`
	End       int    `json:"end_port"`
	Threads   int    `json:"threads"`
	DeepMode  bool   `json:"deep_scanning"`
	Ping_port int    `json:"target_ping_port"`
	Ticker    int    `json:"ticker_time_milliseconds"`
	Protocol  string `json:"protocol"`
	MyWinW    int    `json:"main_window_width"`
	MyWinH    int    `json:"main_window_height"`
	SetWinW   int    `json:"settings_window_width"`
	SetWinH   int    `json:"settings_window_height"`
}

var (
	AppConfig    Settings
	ScanWg       sync.WaitGroup
	PingWg       sync.WaitGroup
	LogDisplay   *widget.Entry
	PingToServer string
)

func main() {

	LogDisplay = widget.NewMultiLineEntry()

	loader() // uploading all settings

	ch := make(chan int, AppConfig.Threads)

	ticker := time.NewTicker(time.Duration(AppConfig.Ticker))
	defer ticker.Stop()

	myApp := app.New() // app

	myWindow := myApp.NewWindow("Go Window With Fyne") // main window name
	settingsWindow := myApp.NewWindow("Scanner settings")

	myWindow.Resize(fyne.NewSize(float32(AppConfig.MyWinW), float32(AppConfig.MyWinH))) // new size
	settingsWindow.Resize(fyne.NewSize(float32(AppConfig.SetWinW), float32(AppConfig.SetWinH)))

	label := widget.NewLabel("Settings")

	btn := widget.NewButton("Start Scan", func() {
		go func() {
			go func() {
				for i := 0; i < AppConfig.Threads; i++ {
					ScanWg.Add(1)
					go func() {
						defer ScanWg.Done()
						for p := range ch {
							<-ticker.C
							worker(p, AppConfig.Duration, AppConfig.Address, AppConfig.DeepMode, AppConfig.Protocol)
						}
					}()
				}
			}()

			go func() {
				for i := AppConfig.Start; i <= AppConfig.End; i++ {
					ch <- i
				}
				close(ch)
			}()

			PingWg.Add(1)
			go func() {
				defer PingWg.Done()
				full := fmt.Sprintf("%s"+":"+"%v", AppConfig.Address, AppConfig.Ping_port)
				startingPoint := time.Now()
				c, err := net.DialTimeout(AppConfig.Address, full, time.Duration(AppConfig.Duration)*time.Second)
				if err != nil {
					PingToServer = "error: bad port(closed or incorrect one)"
					return
				}
				PingToServer = time.Since(startingPoint).Round(time.Millisecond).String()
				c.Close()
			}()

			ScanWg.Wait()
			PingWg.Wait()

			newText := fmt.Sprintf("ping is: %s\n", PingToServer)
			LogDisplay.Append(newText)
			newText = fmt.Sprintf("scanned ports from %d to %d\n", AppConfig.Start, AppConfig.End)
			LogDisplay.Append(newText)
		}()
	})

	settingsCont := container.NewBorder(label, nil, nil, nil, nil)
	settingsWindow.SetContent(settingsCont)

	settingsBtn := widget.NewButton("Conf", func() {
		settingsWindow.Show()
	})

	layout := container.NewBorder(nil, btn, nil, settingsBtn, LogDisplay)

	myWindow.SetContent(layout)
	myWindow.ShowAndRun()
}

func loader() {
	bytes, err := os.ReadFile("config.json")
	if err != nil {
		log.Println("So we are using default settings as config file invalid or programm can't read it:", err)
		// start config
		AppConfig.Address = "127.0.0.1"
		AppConfig.DeepMode = false
		AppConfig.Duration = 1
		AppConfig.Start = 1
		AppConfig.End = 1024     // all main ports
		AppConfig.Ping_port = 80 // cuz rarely blocked
		AppConfig.Ticker = 100
		AppConfig.Threads = 100 // about 100 * 2kb ram is normal and fast
		AppConfig.Protocol = "tcp"
		// windows control default
		AppConfig.MyWinH = 400
		AppConfig.MyWinW = 500
		AppConfig.SetWinH = 400
		AppConfig.SetWinW = 500
		// end config
		return
	}

	err = json.Unmarshal(bytes, &AppConfig)
	if err != nil {
		log.Println("Error while parsing config!")
		log.Fatalln(err)
	}
}

func worker(port, duration int, addr string, mode bool, network string) {
	full := fmt.Sprintf("%s"+":"+"%v", addr, port)
	fullHttp := fmt.Sprintf("http://"+"%s", full)
	if mode == true { // deep search
		c, err := net.DialTimeout(network, full, time.Duration(duration)*time.Second) // testing for classical tcp protocol
		if err != nil {
			return
		}
		if port == 22 { // ssh access port
			c.SetReadDeadline(time.Now().Add(2 * time.Second)) // time for reading
			buf := make([]byte, 100)                           // buffer for testing request
			n, err := c.Read(buf)                              // n is bytes or idk what else
			if err == nil && n > 0 {
				newText := fmt.Sprintf("SSH: %d\n", port)
				LogDisplay.Append(newText)
			} else {
				return
			}
		} else if port == 80 || port == 443 { // standart website port & standart websockets or VPN (I mean xRay or xHttp/Reality trafics) port
			response, err := http.Get(fullHttp) // getting header (main info)
			if err != nil {
				return
			} else if response != nil {
				newText := fmt.Sprintf("http response isn't empty: %d\n", port)
				LogDisplay.Append(newText)
			}
			newText := fmt.Sprintf("HTTP: %d\n", port)
			LogDisplay.Append(newText)
			response.Body.Close()
		} else {
			newText := fmt.Sprintf(AppConfig.Protocol+": %d\n", port)
			LogDisplay.Append(newText) // it often can be fake (also with big sites with high average load)
		}
		c.Close()
	} else if mode == false { // default search without ssh & http checking (can be helpful for localhost/127.0.0.1 scanning or not secure tcp servers)
		conn, err := net.DialTimeout(network, full, time.Duration(duration)*time.Second)
		if err != nil {
			return
		}
		newText := fmt.Sprintf("%d\n", port)
		LogDisplay.Append(newText)
		conn.Close()
	}
}
