package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

type Config struct {
	address        string
	pollInterval   time.Duration
	reportInterval time.Duration
}

// Describe monitoring statistics "class"

type gauge float64
type counter int64

type MonitorStats struct {
	stats     map[string]gauge
	randVal   gauge
	pollCount counter
}

func (monitor *MonitorStats) UpdateMetrics() {
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	monitor.stats["Alloc"] = gauge(rtm.Alloc)
	monitor.stats["BuckHashSys"] = gauge(rtm.BuckHashSys)
	monitor.stats["Frees"] = gauge(rtm.Frees)
	monitor.stats["GCCPUFraction"] = gauge(rtm.GCCPUFraction)
	monitor.stats["GCSys"] = gauge(rtm.GCSys)
	monitor.stats["HeapAlloc"] = gauge(rtm.HeapAlloc)
	monitor.stats["HeapIdle"] = gauge(rtm.HeapIdle)
	monitor.stats["HeapInuse"] = gauge(rtm.HeapInuse)
	monitor.stats["HeapObjects"] = gauge(rtm.HeapObjects)
	monitor.stats["HeapReleased"] = gauge(rtm.HeapReleased)
	monitor.stats["HeapSys"] = gauge(rtm.HeapSys)
	monitor.stats["LastGC"] = gauge(rtm.LastGC)
	monitor.stats["Lookups"] = gauge(rtm.Lookups)
	monitor.stats["MCacheInuse"] = gauge(rtm.MCacheInuse)
	monitor.stats["MCacheSys"] = gauge(rtm.MCacheSys)
	monitor.stats["MSpanInuse"] = gauge(rtm.MSpanInuse)
	monitor.stats["MSpanSys"] = gauge(rtm.MSpanSys)
	monitor.stats["Mallocs"] = gauge(rtm.Mallocs)
	monitor.stats["NextGC"] = gauge(rtm.NextGC)
	monitor.stats["NumForcedGC"] = gauge(rtm.NumForcedGC)
	monitor.stats["NumGC"] = gauge(rtm.NumGC)
	monitor.stats["OtherSys"] = gauge(rtm.OtherSys)
	monitor.stats["PauseTotalNs"] = gauge(rtm.PauseTotalNs)
	monitor.stats["StackInuse"] = gauge(rtm.StackInuse)
	monitor.stats["StackSys"] = gauge(rtm.StackSys)
	monitor.stats["Sys"] = gauge(rtm.Sys)
	monitor.stats["TotalAlloc"] = gauge(rtm.TotalAlloc)
	monitor.pollCount++
	monitor.randVal = gauge(GetRandomValue())
}

func GetRandomValue() float64 {
	return rand.New(rand.NewSource(time.Now().Unix())).Float64()
}

// Now describe agent "class"

type Agent struct {
	client         *http.Client
	data           *MonitorStats
	address        string
	pollInterval   time.Duration
	reportInterval time.Duration
}

func (ag *Agent) Create(cfg *Config) {
	ag.address = cfg.address
	ag.pollInterval = cfg.pollInterval
	ag.reportInterval = cfg.reportInterval
	ag.data = &MonitorStats{}
	ag.client = &http.Client{}
}

func (ag *Agent) Send(ctx context.Context) {
	var url string
	for key, value := range ag.data.stats {
		//http://<АДРЕС_СЕРВЕРА>/update/<ТИП_МЕТРИКИ>/<ИМЯ_МЕТРИКИ>/<ЗНАЧЕНИЕ_МЕТРИКИ>
		url = fmt.Sprintf("http://%s/update/gauge/%s/%v", ag.address, key, value)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		req.Header.Set("Content-Type", "text/plain")
		resp, _ := ag.client.Do(req)
		defer resp.Body.Close()
	}
	//send pollcount
	url = fmt.Sprintf("http://%s/update/counter/%s/%v", ag.address, "PollCount", ag.data.pollCount)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := ag.client.Do(req)
	defer resp.Body.Close()
	url = fmt.Sprintf("http://%s/update/gauge/%s/%v", ag.address, "RandomValue", ag.data.randVal)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	req.Header.Set("Content-Type", "text/plain")
	resp, _ = ag.client.Do(req)
	defer resp.Body.Close()

}

func (ag *Agent) Run(ctx context.Context) {
	updateTicker := time.NewTicker(ag.pollInterval)
	senderTicker := time.NewTicker(ag.reportInterval)
	go func(ctx context.Context, ticker *time.Ticker) {
		for {
			select {
			case <-ticker.C:
				ag.data.UpdateMetrics()
			case <-ctx.Done():
				return
			}
		}
	}(ctx, updateTicker)
	go func(ctx context.Context, ticker *time.Ticker) {
		for {
			select {
			case <-ticker.C:
				ag.Send(ctx)
			case <-ctx.Done():
				return
			}
		}
	}(ctx, senderTicker)
}

func main() {
	cfg := &Config{
		address:        "127.0.0.1:8080",
		pollInterval:   2 * time.Second,
		reportInterval: 10 * time.Second,
	}
	// logic: define agent, get context for agent execution.
	// Start goroutine that waits stop signal
	agent := &Agent{}
	agent.Create(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
		<-signalCh
		cancel()
	}()
	agent.Run(ctx)
}
