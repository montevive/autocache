package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"autocache/internal/config"
	"autocache/internal/server"

	"github.com/sirupsen/logrus"
)

const (
	// Version information
	Version   = "1.0.0"
	BuildTime = "development"
	GitCommit = "unknown"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := cfg.SetupLogger()

	// Print startup banner
	printStartupBanner(logger)

	// Print configuration
	cfg.PrintConfig(logger)

	// Create handler
	handler := server.NewAutocacheHandler(cfg, logger)

	// Setup routes
	mux := handler.SetupRoutes()

	// Wrap with panic recovery middleware (innermost - catches all panics)
	panicRecoveredMux := handler.PanicRecoveryMiddleware(mux)

	// Wrap with logging middleware (outermost - logs all requests)
	loggedMux := handler.LogMiddleware(panicRecoveredMux)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      loggedMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // Long timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.WithFields(logrus.Fields{
			"address": httpServer.Addr,
			"version": Version,
		}).Info("Starting autocache proxy server")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Print ready message
	logger.WithFields(logrus.Fields{
		"address": httpServer.Addr,
		"health":  fmt.Sprintf("http://%s/health", httpServer.Addr),
		"metrics": fmt.Sprintf("http://%s/metrics", httpServer.Addr),
	}).Info("Autocache proxy server is ready")

	// Wait for interrupt signal to gracefully shutdown
	waitForShutdown(httpServer, logger)
}

// printStartupBanner prints the startup banner
func printStartupBanner(logger *logrus.Logger) {
	banner := `
     _         _                     _
    / \  _   _| |_ ___   ___ __ _  ___| |__   ___
   / _ \| | | | __/ _ \ / __/ _` + "`" + ` |/ __| '_ \ / _ \
  / ___ \ |_| | || (_) | (_| (_| | (__| | | |  __/
 /_/   \_\__,_|\__\___/ \___\__,_|\___|_| |_|\___|

 Intelligent Anthropic API Cache Proxy
`

	logger.Info(banner)

	logger.WithFields(logrus.Fields{
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}).Info("Autocache starting up")
}

// waitForShutdown waits for interrupt signal and gracefully shuts down the server
func waitForShutdown(httpServer *http.Server, logger *logrus.Logger) {
	// Create a channel to receive OS signals
	quit := make(chan os.Signal, 1)

	// Register the channel to receive specific signals
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	sig := <-quit
	logger.WithField("signal", sig.String()).Info("Received shutdown signal")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	logger.Info("Shutting down server gracefully...")

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}

// printUsage prints usage information
func printUsage() {
	fmt.Printf(`Autocache - Intelligent Anthropic API Cache Proxy v%s

USAGE:
    autocache [FLAGS]

FLAGS:
    -h, --help     Show this help message
    -v, --version  Show version information

ENVIRONMENT VARIABLES:
    PORT                     Server port (default: 8080)
    HOST                     Server host (default: 0.0.0.0)
    ANTHROPIC_API_KEY        Your Anthropic API key
    ANTHROPIC_API_URL        Anthropic API URL (default: https://api.anthropic.com)
    CACHE_STRATEGY           Cache strategy: conservative|moderate|aggressive (default: moderate)
    LOG_LEVEL                Log level: trace|debug|info|warn|error (default: info)
    LOG_JSON                 Use JSON logging: true|false (default: false)
    ENABLE_METRICS           Enable metrics endpoint: true|false (default: true)
    ENABLE_DETAILED_ROI      Enable detailed ROI calculation: true|false (default: true)
    MAX_CACHE_BREAKPOINTS    Maximum cache breakpoints: 1-4 (default: 4)
    TOKEN_MULTIPLIER         Token count multiplier for caching threshold (default: 1.0)

EXAMPLES:
    # Start with default configuration
    autocache

    # Start with custom port and aggressive caching
    PORT=3000 CACHE_STRATEGY=aggressive autocache

    # Start with debug logging
    LOG_LEVEL=debug autocache

ENDPOINTS:
    POST /v1/messages    Main API endpoint (drop-in replacement for Anthropic API)
    GET  /health         Health check endpoint
    GET  /metrics        Metrics and configuration endpoint

CACHE HEADERS:
    The proxy adds these headers to responses with cache information:

    X-Autocache-Injected        true|false - Whether cache was injected
    X-Autocache-Total-Tokens    Total tokens in request
    X-Autocache-Cached-Tokens   Tokens that were cached
    X-Autocache-Cache-Ratio     Ratio of cached tokens (0.0-1.0)
    X-Autocache-Strategy        Cache strategy used
    X-Autocache-Model           Model that was used
    X-Autocache-ROI-FirstCost   Cost of first request with cache writes
    X-Autocache-ROI-Savings     Savings per subsequent request
    X-Autocache-ROI-BreakEven   Number of requests to break even
    X-Autocache-ROI-Percent     Percentage savings at scale
    X-Autocache-Breakpoints     Cache breakpoints: position:tokens:ttl,position:tokens:ttl
    X-Autocache-Savings-10req   Total savings after 10 requests
    X-Autocache-Savings-100req  Total savings after 100 requests

BYPASS HEADERS:
    Add these headers to requests to bypass caching:

    X-Autocache-Bypass: true    Skip cache injection entirely
    X-Autocache-Disable: true   Skip cache injection entirely

For more information, visit: https://github.com/yourusername/autocache
`, Version)
}

// handleFlags handles command line flags
func handleFlags() {
	args := os.Args[1:]

	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "-v", "--version":
			fmt.Printf("autocache version %s (built %s, commit %s)\n", Version, BuildTime, GitCommit)
			os.Exit(0)
		default:
			if arg[0] == '-' {
				fmt.Printf("Unknown flag: %s\n", arg)
				fmt.Println("Use --help for usage information")
				os.Exit(1)
			}
		}
	}
}

func init() {
	// Only handle flags when not running tests
	if !isRunningTests() {
		handleFlags()
	}

	// Set build info from build-time variables if available
	if buildTime := os.Getenv("BUILD_TIME"); buildTime != "" {
		// This would be set at build time
	}
	if gitCommit := os.Getenv("GIT_COMMIT"); gitCommit != "" {
		// This would be set at build time
	}
}

// isRunningTests checks if we're running in test mode
func isRunningTests() bool {
	for _, arg := range os.Args {
		if strings.Contains(arg, "test") || strings.HasSuffix(arg, ".test") {
			return true
		}
	}
	return false
}