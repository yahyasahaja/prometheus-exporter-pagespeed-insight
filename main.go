package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type target struct {
	URL      string
	Strategy string
}

var (
	perfScore = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "psi_performance_score",
		Help: "Performance score from PSI (0-1 scale)",
	}, []string{"site", "strategy"})

	fcp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "psi_first_contentful_paint",
		Help: "First Contentful Paint in milliseconds",
	}, []string{"site", "strategy"})

	lcp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "psi_largest_contentful_paint",
		Help: "Largest Contentful Paint in milliseconds",
	}, []string{"site", "strategy"})

	cls = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "psi_cumulative_layout_shift",
		Help: "Cumulative Layout Shift score",
	}, []string{"site", "strategy"})
)

func fetchPSIData(apiKey string, target target) {
	log.Printf("Fetching PSI data for %s (%s)...", target.URL, target.Strategy)
	url := fmt.Sprintf("https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=%s&strategy=%s&key=%s", target.URL, target.Strategy, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		log.Println("Error fetching PSI:", err)
		return
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Println("Error decoding PSI response:", err)
		return
	}

	result := data["lighthouseResult"].(map[string]interface{})
	categories := result["categories"].(map[string]interface{})
	performance := categories["performance"].(map[string]interface{})

	labels := prometheus.Labels{"site": target.URL, "strategy": target.Strategy}

	if score, ok := performance["score"].(float64); ok {
		perfScore.With(labels).Set(score)
	}

	audits := result["audits"].(map[string]interface{})

	if v, ok := audits["first-contentful-paint"].(map[string]interface{})["numericValue"].(float64); ok {
		fcp.With(labels).Set(v)
	}
	if v, ok := audits["largest-contentful-paint"].(map[string]interface{})["numericValue"].(float64); ok {
		lcp.With(labels).Set(v)
	}
	if v, ok := audits["cumulative-layout-shift"].(map[string]interface{})["numericValue"].(float64); ok {
		cls.With(labels).Set(v)
	}
}

func expandTargets(urls []string) []target {
	strategies := []string{"mobile", "desktop"}
	targets := []target{}
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		for _, s := range strategies {
			targets = append(targets, target{URL: u, Strategy: s})
		}
	}
	return targets
}

func parseMinutes(minArg string) []int {
	parts := strings.Split(minArg, ",")
	minutes := []int{}
	for _, p := range parts {
		if val, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && val >= 0 && val < 60 {
			minutes = append(minutes, val)
		}
	}
	return minutes
}

func main() {
	apiKey := flag.String("apikey", "", "Google PageSpeed Insights API key")
	urlsArg := flag.String("urls", "", "Comma-separated list of URLs to monitor")
	minutesArg := flag.String("minutes", "0,30", "Comma-separated list of minutes in an hour to run fetch")
	flag.Parse()

	if *apiKey == "" || *urlsArg == "" {
		log.Fatal("Both --apikey and --urls must be provided")
	}

	urls := strings.Split(*urlsArg, ",")
	targets := expandTargets(urls)
	fetchMinutes := parseMinutes(*minutesArg)

	prometheus.MustRegister(perfScore, fcp, lcp, cls)

	// Initial fetch
	for _, t := range targets {
		fetchPSIData(*apiKey, t)
		time.Sleep(2 * time.Second)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			minute := now.Minute()
			for _, m := range fetchMinutes {
				if minute == m {
					log.Printf("Minute match %d: fetching...", m)
					for _, t := range targets {
						fetchPSIData(*apiKey, t)
						time.Sleep(2 * time.Second)
					}
					break
				}
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Println("PSI Exporter listening on :2112")
	log.Fatal(http.ListenAndServe(":2112", nil))
}
