package main

import (
	// stdlib
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	// fyne framework
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type Settings struct {
	// config.json (for flexible settings)
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

type JSONexport struct {
	Addr      string   `json:"address"`
	Timestamp string   `json:"time"`
	Resp      []string `json:"response"`
}

var (
	AppConfig    Settings       // config.json
	ExportJSON   JSONexport     // .json file with exported data idk
	Wg           sync.WaitGroup // wg
	LogWg        sync.WaitGroup // wg for logs
	LogDisplay   *widget.Entry  // Desktop main dispay
	PingToServer string         // time up to server
	Mu           sync.Mutex     // mutex
	Response     []string       // builds answer
)

func main() {

	LogDisplay = widget.NewMultiLineEntry()
	LogDisplay.Disable()

	loader() // uploading all settings

	myApp := app.New() // app

	myWindow := myApp.NewWindow("Port-checker") // main window name
	settingsWindow := myApp.NewWindow("Scanner settings")

	myWindow.Resize(fyne.NewSize(float32(AppConfig.MyWinW), float32(AppConfig.MyWinH))) // new sizes
	settingsWindow.Resize(fyne.NewSize(float32(AppConfig.SetWinW), float32(AppConfig.SetWinH)))

	exportEntry := widget.NewEntry()

	exportbtn := widget.NewButton("Export", func() {
		go func() {

			exportEntry.SetPlaceHolder("Filename for example: exportfile")

			exportingEntry := fmt.Sprintf("%s.json", exportEntry.Text)

			LogDisplay.SetText("") // resets all data but slice is in safety
			LogDisplay.Append("Exporting in progress ...")

			longData, err := exporting(exportingEntry)
			if err != nil {
				log.Println(err)
			}
			LogDisplay.SetText(longData)
		}()
	})

	btn := widget.NewButton("Check ports", func() { // start button

		var piggy string

		go func() {

			LogDisplay.SetText("Scanning ...")
			Mu.Lock()
			Response = nil
			Mu.Unlock()

			ticker := time.NewTicker(time.Duration(AppConfig.Ticker) * time.Millisecond)
			defer ticker.Stop()

			start := time.Now()
			ch := make(chan int, AppConfig.Threads)

			Wg.Add(AppConfig.Threads)
			for i := 0; i < AppConfig.Threads; i++ {
				go func() {
					defer Wg.Done()
					for p := range ch {
						<-ticker.C
						worker(p, AppConfig.Duration, AppConfig.Address, AppConfig.DeepMode, AppConfig.Protocol)
					}
				}()
			}

			go func() {
				for i := AppConfig.Start; i <= AppConfig.End; i++ {
					ch <- i
				}
				close(ch)
			}()

			Wg.Add(1)
			go func() {
				defer Wg.Done()
				full := fmt.Sprintf("%s:%v", AppConfig.Address, AppConfig.Ping_port)

				startingPoint := time.Now()

				c, err := net.DialTimeout("tcp", full, time.Duration(AppConfig.Duration)*time.Second)
				if err != nil {
					PingToServer = "error: 1050 - bad port or connection timeout"
					return
				}

				PingToServer = time.Since(startingPoint).Round(time.Millisecond).String()
				c.Close()
			}()

			Wg.Wait() // waiting for all go

			Mu.Lock()
			piggy = time.Since(start).String()

			LogDisplay.SetText("")

			for _, response := range Response {
				LogDisplay.Append(response)
			}

			if len(Response) == 0 {
				LogDisplay.Append("All ports are not available\n")
			}

			LogDisplay.Append(fmt.Sprintf("Scanner has made it's work! %s\n", piggy))
			LogDisplay.Append(fmt.Sprintf("Ping: %s\n", PingToServer))
			LogDisplay.Append(fmt.Sprintf("Scanned ports from %d to %d\n", AppConfig.Start, AppConfig.End))
			Mu.Unlock()
		}()
	})

	// config settings

	addressEntry := widget.NewEntry()
	addressEntry.SetText(AppConfig.Address)

	startPort := widget.NewEntry()
	startPort.SetText(strconv.Itoa(AppConfig.Start))

	endPort := widget.NewEntry()
	endPort.SetText(strconv.Itoa(AppConfig.End))

	threads := widget.NewEntry()
	threads.SetText(strconv.Itoa(AppConfig.Threads))

	durationTime := widget.NewEntry()
	durationTime.SetText(strconv.Itoa(AppConfig.Duration))

	tickerTime := widget.NewEntry()
	tickerTime.SetText(strconv.Itoa(AppConfig.Ticker))

	target := widget.NewEntry()
	target.SetText(strconv.Itoa(AppConfig.Ping_port))

	protocol := widget.NewEntry()
	protocol.SetText(AppConfig.Protocol)

	deep := widget.NewEntry()
	deep.SetText(strconv.FormatBool(AppConfig.DeepMode))

	// ends here

	settingsCont := widget.NewForm(
		widget.NewFormItem("Target Address:", addressEntry),
		widget.NewFormItem("From port:", startPort),
		widget.NewFormItem("To port:", endPort),
		widget.NewFormItem("Threads aka workers:", threads),
		widget.NewFormItem("Duration in sec", durationTime),
		widget.NewFormItem("Ticker in ms:", tickerTime),
		widget.NewFormItem("Target port:", target),
		widget.NewFormItem("Network (tcp/udp):", protocol),
		widget.NewFormItem("Mode (true/false):", deep),
	)

	settingsCont.OnSubmit = func() {
		AppConfig.Address = addressEntry.Text

		if deep.Text == "false" {
			AppConfig.DeepMode = false
		} else if deep.Text == "true" {
			AppConfig.DeepMode = true
		} else {
			log.Printf("Error: Mode must be boolean (true/false)\n")
			AppConfig.DeepMode = true
		}

		dur, err := strconv.Atoi(durationTime.Text)
		if err != nil {
			log.Printf("Must be integer\n")
			return
		}
		AppConfig.Duration = dur

		st, err := strconv.Atoi(startPort.Text)
		if err != nil {
			log.Printf("Must be integer\n")
			return
		}
		AppConfig.Start = st

		end, err := strconv.Atoi(endPort.Text)
		if err != nil {
			log.Printf("Must be integer\n")
			return
		}
		AppConfig.End = end

		pPort, err := strconv.Atoi(target.Text)
		if err != nil {
			log.Printf("Must be integer\n")
			return
		}
		AppConfig.Ping_port = pPort

		tick, err := strconv.Atoi(tickerTime.Text)
		if err != nil {
			log.Printf("Must be integer\n")
			return
		}
		AppConfig.Ticker = tick

		thr, err := strconv.Atoi(threads.Text)
		if err != nil {
			log.Printf("Must be integer\n")
			return
		}
		AppConfig.Threads = thr
		AppConfig.Protocol = protocol.Text
		// ending

		error := saveSettings(AppConfig)
		if error != nil {
			log.Fatalln(err)
		}

		loader() // calls loader if all is OK

		settingsWindow.Close()
	}

	settingsWindow.SetContent(settingsCont) // sets content into the window

	settingsBtn := widget.NewButton("Conf", func() {
		settingsWindow.Show()
	})

	layout := container.NewBorder(exportEntry, btn, exportbtn, settingsBtn, LogDisplay)

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
		AppConfig.Ping_port = 80 // cuz rarely blockedorma
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

func worker(port int, duration int, addr string, mode bool, network string) {
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
				Mu.Lock()
				Response = append(Response, newText)
				Mu.Unlock()
			} else {
				return
			}
		} else if port == 80 || port == 443 { // standart website port & standart websockets or VPN (I mean xRay or xHttp/Reality trafics) port
			c := http.Client{ // custom client (because server can be without http servers)
				Timeout: 3 * time.Second,
			}
			response, err := c.Get(fullHttp) // get method (http.Head may be better)
			if err != nil {
				return
			} else if response != nil {
				newText := fmt.Sprintf("http response isn't empty: %d\n", port)
				Mu.Lock()
				Response = append(Response, newText)
				Mu.Unlock()
			}
			newText := fmt.Sprintf("HTTP: %d\n", port)
			Mu.Lock()
			Response = append(Response, newText)
			Mu.Unlock()

			response.Body.Close() // closing resp.
		} else {
			newText := fmt.Sprintf(AppConfig.Protocol+": %d\n", port) // it often can be fake (also with big sites with high average load)

			Mu.Lock()
			Response = append(Response, newText)
			Mu.Unlock()
		}
		c.Close()
	} else if mode == false { // default search without ssh & http checking (can be helpful for localhost/127.0.0.1 scanning or not secure tcp servers)
		conn, err := net.DialTimeout(network, full, time.Duration(duration)*time.Second)
		if err != nil {
			return
		}
		newText := fmt.Sprintf("%d\n", port)
		Mu.Lock()
		Response = append(Response, newText)
		Mu.Unlock()
		conn.Close()
	}
}

func saveSettings(cfg Settings) error {
	data, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		log.Printf("Marshaling error idk!")
		return err
	}

	timeFile := "config.json.tmp"
	err = os.WriteFile(timeFile, data, 0644)
	if err != nil {
		log.Printf("Writing error")
		return err
	}

	return os.Rename(timeFile, "config.json")
}

func exporting(name string) (string, error) {
	f, err := os.Create(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	ExportJSON = JSONexport{
		Addr:      AppConfig.Address,
		Timestamp: time.Now().Format(time.RFC3339),
		Resp:      Response, // copy
	}

	data, err := json.MarshalIndent(ExportJSON, "", "  ")

	err = os.WriteFile(name, data, 0666)
	if err != nil {
		return "", err
	}
	return "Exported successfully", err
}
