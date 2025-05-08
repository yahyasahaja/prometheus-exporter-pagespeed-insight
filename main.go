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

	tbt = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "psi_total_blocking_time",
		Help: "Total Blocking Time in milliseconds",
	}, []string{"site", "strategy"})
)

func fetchPSIData(apiKey string, target target) {
	log.Printf("Fetching PSI data for %s (%s)...", target.URL, target.Strategy)
	url := fmt.Sprintf("https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=%s&strategy=%s&key=%s", target.URL, target.Strategy, apiKey)

	// Exponential backoff parameters
	maxRetries := 5
	delay := 2 * time.Second

	for retries := 0; retries < maxRetries; retries++ {
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Error fetching PSI: %v", err)
			time.Sleep(delay)
			delay *= 2 // Increase delay for next retry
			continue
		}
		defer resp.Body.Close()

		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Printf("Error decoding PSI response: %v", err)
			time.Sleep(delay)
			delay *= 2
			continue
		}

		// Check if the expected fields are available in the response
		result, ok := data["lighthouseResult"].(map[string]interface{})
		if !ok {
			log.Println("Invalid response structure: missing 'lighthouseResult'", data)
			time.Sleep(delay)
			delay *= 2
			continue
		}

		categories, ok := result["categories"].(map[string]interface{})
		if !ok {
			log.Println("Invalid response structure: missing 'categories'")
			time.Sleep(delay)
			delay *= 2
			continue
		}

		performance, ok := categories["performance"].(map[string]interface{})
		if !ok {
			log.Println("Invalid response structure: missing 'performance' category")
			time.Sleep(delay)
			delay *= 2
			continue
		}

		// Extract performance score and other data
		labels := prometheus.Labels{"site": target.URL, "strategy": target.Strategy}

		if score, ok := performance["score"].(float64); ok {
			perfScore.With(labels).Set(score)
		}

		audits, ok := result["audits"].(map[string]interface{})
		if ok {
			// Extract FCP, LCP, CLS, TBT
			if v, ok := audits["first-contentful-paint"].(map[string]interface{})["numericValue"].(float64); ok {
				fcp.With(labels).Set(v)
			}
			if v, ok := audits["largest-contentful-paint"].(map[string]interface{})["numericValue"].(float64); ok {
				lcp.With(labels).Set(v)
			}
			if v, ok := audits["cumulative-layout-shift"].(map[string]interface{})["numericValue"].(float64); ok {
				cls.With(labels).Set(v)
			}
			if v, ok := audits["total-blocking-time"].(map[string]interface{})["numericValue"].(float64); ok {
				tbt.With(labels).Set(v)
			}
		}

		// If we reached here, the response was valid and processed successfully
		return
	}

	// After all retries, log the failure
	log.Printf("Failed to fetch data for %s after %d retries.", target.URL, maxRetries)
}

// New endpoint to execute PSI for a given URL and strategy
func executePSI(w http.ResponseWriter, r *http.Request, apiKey string) {
	url := r.URL.Query().Get("url")
	strategy := r.URL.Query().Get("strategy")

	if url == "" || strategy == "" {
		http.Error(w, "Missing URL or strategy", http.StatusBadRequest)
		return
	}

	// Call fetchPSIData for the provided URL and strategy
	target := target{URL: url, Strategy: strategy}
	fetchPSIData(apiKey, target)

	// Prepare the response
	response := map[string]interface{}{
		"performance_score": perfScore,
		"cls":               cls,
		"fcp":               fcp,
		"lcp":               lcp,
		"tbt":               tbt,
		"rawData":           r,
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	if len(parts) == 0 {
		log.Println("Warning: No minutes specified, no fetch will occur.")
	}
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
	port := flag.String("port", "2112", "Port to run the exporter on")
	withInitialFetch := flag.Bool("initial", false, "Fetch initial data")
	flag.Parse()

	if *apiKey == "" || *urlsArg == "" {
		log.Fatal("Both --apikey and --urls must be provided")
	}

	urls := strings.Split(*urlsArg, ",")
	targets := expandTargets(urls)
	fetchMinutes := parseMinutes(*minutesArg)

	prometheus.MustRegister(perfScore, fcp, lcp, cls, tbt)

	// Initial fetch
	go func() {
		if *withInitialFetch {
			for _, t := range targets {
				fetchPSIData(*apiKey, t)
				time.Sleep(2 * time.Second)
			}
		}
	}()

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

	// Add /execute endpoint for manual fetch
	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		executePSI(w, r, *apiKey)
	})

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("PSI Exporter listening on :%s", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", *port), nil))
}
