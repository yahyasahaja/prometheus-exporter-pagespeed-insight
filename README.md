# Prometheus PageSpeed Insights Exporter

A Prometheus exporter that fetches and exposes Google PageSpeed Insights (PSI) metrics for monitoring website performance.

## Features

- 🔄 Automatic periodic fetching of PSI data at configurable intervals
- 📊 Exposes Prometheus metrics for performance scores and Core Web Vitals
- 📱 Supports both mobile and desktop strategies
- 🎯 Multiple URL monitoring with comma-separated list
- 🔌 HTTP endpoint for manual PSI execution
- ⚡ Exponential backoff retry mechanism for API calls

## Prerequisites

- Go 1.23.0 or higher
- Google PageSpeed Insights API key ([Get one here](https://developers.google.com/speed/docs/insights/v5/get-started))

## Installation

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd prometheus-exporter-pagespeed-insight

# Build the binary
make build

# Or build for Linux
make build-linux
```

### Using Makefile

```bash
# Build for current platform
make build

# Build for Linux (amd64)
make build-linux

# Install to /usr/local/bin
make install

# Clean build artifacts
make clean
```

### Manual Build

```bash
go build -o psi_exporter main.go
```

## Usage

### Basic Usage

```bash
./psi_exporter \
  --apikey YOUR_API_KEY \
  --urls https://example.com,https://another-site.com
```

### Full Options

```bash
./psi_exporter \
  --apikey YOUR_API_KEY \
  --urls https://example.com,https://another-site.com \
  --minutes 0,15,30,45 \
  --port 2112 \
  --initial
```

### Command-Line Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `--apikey` | ✅ Yes | - | Google PageSpeed Insights API key |
| `--urls` | ✅ Yes | - | Comma-separated list of URLs to monitor |
| `--minutes` | ❌ No | `0,30` | Comma-separated list of minutes (0-59) in an hour to run fetch |
| `--port` | ❌ No | `2112` | Port to run the exporter on |
| `--initial` | ❌ No | `false` | Fetch initial data on startup |

### Examples

**Monitor a single website:**
```bash
./psi_exporter --apikey YOUR_API_KEY --urls https://example.com
```

**Monitor multiple websites with custom schedule:**
```bash
./psi_exporter \
  --apikey YOUR_API_KEY \
  --urls https://example.com,https://test.com,https://demo.com \
  --minutes 0,15,30,45 \
  --initial
```

**Run on custom port:**
```bash
./psi_exporter \
  --apikey YOUR_API_KEY \
  --urls https://example.com \
  --port 9090
```

## How It Works

1. The exporter automatically expands each URL to monitor both `mobile` and `desktop` strategies
2. At the specified minutes of each hour, it fetches PSI data for all configured URLs
3. Metrics are exposed in Prometheus format at `/metrics` endpoint
4. The exporter includes retry logic with exponential backoff (up to 5 retries)

## API Endpoints

### `/metrics`

Prometheus metrics endpoint. Returns all collected PSI metrics in Prometheus format.

**Example:**
```bash
curl http://localhost:2112/metrics
```

### `/execute`

Manually trigger a PSI fetch for a specific URL and strategy.

**Parameters:**
- `url` (required): The URL to test
- `strategy` (required): Either `mobile` or `desktop`

**Example:**
```bash
curl "http://localhost:2112/execute?url=https://example.com&strategy=mobile"
```

## Exported Metrics

The exporter exposes the following Prometheus metrics:

| Metric Name | Type | Description | Labels |
|------------|------|-------------|--------|
| `psi_performance_score` | Gauge | Performance score from PSI (0-1 scale) | `site`, `strategy` |
| `psi_first_contentful_paint` | Gauge | First Contentful Paint in milliseconds | `site`, `strategy` |
| `psi_largest_contentful_paint` | Gauge | Largest Contentful Paint in milliseconds | `site`, `strategy` |
| `psi_cumulative_layout_shift` | Gauge | Cumulative Layout Shift score | `site`, `strategy` |
| `psi_total_blocking_time` | Gauge | Total Blocking Time in milliseconds | `site`, `strategy` |

### Metric Labels

- `site`: The URL being monitored
- `strategy`: Either `mobile` or `desktop`

### Example Metrics Output

```
psi_performance_score{site="https://example.com",strategy="mobile"} 0.85
psi_first_contentful_paint{site="https://example.com",strategy="mobile"} 1200.5
psi_largest_contentful_paint{site="https://example.com",strategy="mobile"} 2500.0
psi_cumulative_layout_shift{site="https://example.com",strategy="mobile"} 0.05
psi_total_blocking_time{site="https://example.com",strategy="mobile"} 150.2
```

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'psi-exporter'
    static_configs:
      - targets: ['localhost:2112']
```

## Docker (Optional)

You can containerize the exporter:

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o psi_exporter main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/psi_exporter .
EXPOSE 2112
CMD ["./psi_exporter", "--apikey", "${API_KEY}", "--urls", "${URLS}"]
```

## Error Handling

The exporter implements exponential backoff retry mechanism:
- Maximum 5 retries per fetch
- Initial delay: 2 seconds
- Delay doubles after each retry (2s, 4s, 8s, 16s, 32s)
- Logs errors for failed fetches after all retries are exhausted

## Development

### Project Structure

```
.
├── main.go           # Main application code
├── go.mod            # Go module definition
├── go.sum            # Go module checksums
├── Makefile          # Build automation
└── README.md         # This file
```

### Dependencies

- `github.com/prometheus/client_golang` - Prometheus Go client library

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]

## Author

[Add author information here]

