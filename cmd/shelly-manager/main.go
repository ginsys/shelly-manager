package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ginsys/shelly-manager/internal/api"
	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/plugins"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/backup"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/jsonexport"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/opnsense"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/registry"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/sma"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/yamlexport"
	"github.com/ginsys/shelly-manager/internal/provisioning"
	"github.com/ginsys/shelly-manager/internal/security/secrets"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// Global variables
var (
	shellyService       *service.ShellyService
	dbManager           *database.Manager
	provisioningManager *provisioning.ProvisioningManager
	notificationHandler *notification.Handler
	metricsService      *metrics.Service
	metricsCollector    *metrics.Collector
	metricsHandler      *metrics.Handler
	syncEngine          *sync.SyncEngine
	basePluginRegistry  *plugins.Registry
	pluginRegistry      *registry.PluginRegistry
	cfg                 *config.Config
	logger              *logging.Logger
	configFile          string
)

// Root command
var rootCmd = &cobra.Command{
	Use:   "shelly-manager",
	Short: "Manage Shelly IoT devices",
	Long: `A comprehensive tool for discovering, configuring, and managing 
Shelly smart home devices on your network.`,
}

// CLI Commands
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	Run: func(cmd *cobra.Command, args []string) {
		devices, err := dbManager.GetDevices()
		if err != nil {
			log.Fatal("Error fetching devices:", err)
		}

		fmt.Printf("%-5s %-15s %-18s %-12s %-20s %-10s\n",
			"ID", "IP", "MAC", "Type", "Name", "Status")
		fmt.Println(strings.Repeat("-", 80))

		for _, device := range devices {
			fmt.Printf("%-5d %-15s %-18s %-12s %-20s %-10s\n",
				device.ID, device.IP, device.MAC, device.Type, device.Name, device.Status)
		}
	},
}

var discoverCmd = &cobra.Command{
	Use:   "discover [network]",
	Short: "Discover devices on network",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var network string
		if len(args) > 0 {
			network = args[0]
		} else {
			// Use "auto" to trigger config-based discovery
			network = "auto"
		}

		if network == "auto" && len(cfg.Discovery.Networks) > 0 {
			fmt.Printf("Discovering devices on configured networks: %v\n", cfg.Discovery.Networks)
		} else if network != "auto" {
			fmt.Printf("Discovering devices on network %s...\n", network)
		}

		devices, err := shellyService.DiscoverDevicesReport(network)
		if err != nil {
			log.Fatal("Discovery failed:", err)
		}

		fmt.Printf("\nFound %d devices:\n", len(devices))
		fmt.Println(strings.Repeat("-", 80))

		for _, device := range devices {
			fmt.Printf("IP: %-15s  MAC: %s\n", device.IP, device.MAC)
			fmt.Printf("Type: %-20s  Firmware: %s\n", device.Type, device.Firmware)
			fmt.Printf("Name: %s\n", device.Name)

			// Parse settings to show more info
			var settings map[string]interface{}
			if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil {
				if model, ok := settings["model"].(string); ok {
					fmt.Printf("Model: %s", model)
					if gen, ok := settings["gen"].(float64); ok {
						fmt.Printf(" (Gen %d)", int(gen))
					}
					if auth, ok := settings["auth_enabled"].(bool); ok && auth {
						fmt.Printf(" [Auth Required]")
					}
					fmt.Println()
				}
			}

			// The discovery has stored the device already, an address included, and it reports
			// whether this pass created the row, so this command writes nothing more
			// (ginsys/shelly-manager#572, ginsys/shelly-manager#585).
			if device.Added {
				fmt.Printf("✓ Added to database\n")
			} else {
				fmt.Printf("• Already in database\n")
			}
			fmt.Println(strings.Repeat("-", 80))
		}
	},
}

var addCmd = &cobra.Command{
	Use:   "add <ip> [name]",
	Short: "Add a device by IP address",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if err := runAdd(os.Stdout, args[0], name); err != nil {
			log.Fatal(err)
		}
	},
}

// runAdd discovers the device at ip and reports it. name is empty when none was given.
func runAdd(w io.Writer, ip, name string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("%q is not an IP address", ip)
	}

	// The probe stores the device under the name, so there is nothing left to create or save here.
	// A device that is known keeps the name it has.
	found, err := shellyService.DiscoverHost(ip, name)
	if err != nil {
		return fmt.Errorf("could not discover device at %s: %w", ip, err)
	}
	if found == nil {
		return fmt.Errorf("could not discover device at %s", ip)
	}
	device := *found

	if !device.Added {
		_, err = fmt.Fprintf(w, "Device already known: %s (%s) at %s\n", device.Name, device.Type, device.IP)
		return err
	}

	_, err = fmt.Fprintf(w, "Added device: %s (%s) at %s\n", device.Name, device.Type, device.IP)
	return err
}

var scanAPCmd = &cobra.Command{
	Use:   "scan-ap",
	Short: "Scan for Shelly devices in AP mode",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Println("Scanning for Shelly devices in AP mode...")

		devices, err := provisioningManager.DiscoverUnprovisionedDevices(ctx)
		if err != nil {
			fmt.Printf("Warning: Failed to scan for unprovisioned devices: %v\n", err)
			fmt.Println("No unprovisioned Shelly devices found in AP mode")
			return
		}

		if len(devices) == 0 {
			fmt.Println("No unprovisioned Shelly devices found in AP mode")
			return
		}

		fmt.Printf("\nFound %d unprovisioned devices:\n", len(devices))
		fmt.Println(strings.Repeat("-", 80))

		for _, device := range devices {
			fmt.Printf("MAC: %-18s  SSID: %s\n", device.MAC, device.SSID)
			fmt.Printf("Model: %-15s  Generation: %d\n", device.Model, device.Generation)
			fmt.Printf("IP: %-15s  Signal: %d%%\n", device.IP, device.Signal)
			fmt.Printf("Discovered: %s\n", device.Discovered.Format("2006-01-02 15:04:05"))
			fmt.Println(strings.Repeat("-", 80))
		}
	},
}

var provisionCmd = &cobra.Command{
	Use:   "provision <ssid> [password]",
	Short: "Provision unconfigured devices to join WiFi network",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		targetSSID := args[0]
		targetPassword := ""
		if len(args) > 1 {
			targetPassword = args[1]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		fmt.Printf("Searching for unprovisioned Shelly devices...\n")

		devices, err := provisioningManager.DiscoverUnprovisionedDevices(ctx)
		if err != nil {
			fmt.Printf("Error: Failed to discover unprovisioned devices: %v\n", err)
			return
		}

		if len(devices) == 0 {
			fmt.Println("No unprovisioned Shelly devices found")
			return
		}

		fmt.Printf("Found %d unprovisioned devices. Starting provisioning...\n", len(devices))

		// Get additional configuration from flags
		deviceName, _ := cmd.Flags().GetString("name")
		enableAuth, _ := cmd.Flags().GetBool("enable-auth")
		authUser, _ := cmd.Flags().GetString("auth-user")
		authPassword, _ := cmd.Flags().GetString("auth-password")
		enableCloud, _ := cmd.Flags().GetBool("enable-cloud")
		enableMQTT, _ := cmd.Flags().GetBool("enable-mqtt")
		mqttServer, _ := cmd.Flags().GetString("mqtt-server")
		timeout, _ := cmd.Flags().GetInt("timeout")

		successCount := 0
		failCount := 0

		for i, device := range devices {
			fmt.Printf("\n[%d/%d] Provisioning device: %s (%s)\n",
				i+1, len(devices), device.SSID, device.Model)

			// Create provisioning request
			request := provisioning.ProvisioningRequest{
				SSID:         targetSSID,
				Password:     targetPassword,
				DeviceName:   deviceName,
				EnableAuth:   enableAuth,
				AuthUser:     authUser,
				AuthPassword: authPassword,
				EnableCloud:  enableCloud,
				EnableMQTT:   enableMQTT,
				MQTTServer:   mqttServer,
				Timeout:      timeout,
			}

			// If no device name specified, generate one
			if request.DeviceName == "" {
				request.DeviceName = fmt.Sprintf("Shelly-%s", device.MAC[len(device.MAC)-6:])
			}

			result, err := provisioningManager.ProvisionDevice(ctx, device, request)
			if err != nil {
				fmt.Printf("❌ Provisioning failed: %v\n", err)
				if result != nil {
					fmt.Printf("   Steps completed: %d/%d\n",
						countSuccessfulSteps(result.Steps), len(result.Steps))
				}
				failCount++
				continue
			}

			fmt.Printf("✅ Provisioning completed successfully!\n")
			fmt.Printf("   Device Name: %s\n", result.DeviceName)
			fmt.Printf("   New IP: %s\n", result.DeviceIP)
			fmt.Printf("   Duration: %s\n", result.Duration.String())
			fmt.Printf("   Steps: %d/%d successful\n",
				countSuccessfulSteps(result.Steps), len(result.Steps))

			successCount++
		}

		fmt.Printf("\nProvisioning Summary:\n")
		fmt.Printf("✅ Successful: %d\n", successCount)
		fmt.Printf("❌ Failed: %d\n", failCount)
		fmt.Printf("📊 Total: %d\n", len(devices))
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the HTTP API server",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

// startServer starts the HTTP API server
func startServer() {
	// Create API handler with service and logger
	apiHandler := api.NewHandlerWithLogger(dbManager, shellyService, notificationHandler, metricsHandler, logger)

	// Initialize the admin key (if configured). The export, import and metrics handlers read this same
	// key once ShareAdminKey has run below, so a rotation switches all of them at once.
	if cfg != nil && cfg.Security.AdminAPIKey != "" {
		apiHandler.SetAdminAPIKey(cfg.Security.AdminAPIKey)
	}
	// The agent key opens the provisioner agent routes next to the admin key. Only the API handler
	// takes it: export, import and metrics have no agent routes.
	if cfg != nil && cfg.Security.AgentAPIKey != "" {
		apiHandler.SetAgentAPIKey(cfg.Security.AgentAPIKey)
	}
	if cfg != nil {
		apiHandler.SetAgentEnrolment(cfg.Security.AgentEnrolment)
	}
	// The status route reports this, so the UI can say a stored schedule is not running here.
	apiHandler.SetDriftSchedulerEnabled(cfg != nil && cfg.Drift.SchedulerEnabled)

	// Wire integration (7.2.d): emit notifications from configuration drift detection
	if notificationHandler != nil && apiHandler.ConfigService != nil {
		apiHandler.ConfigService.SetDriftNotifier(func(ctx context.Context, deviceID uint, deviceName string, differenceCount int) {
			msg := fmt.Sprintf("%d configuration differences detected", differenceCount)
			_ = notificationHandler.NotifyEvent(ctx, &notification.NotificationEvent{
				Type:       "drift_detected",
				AlertLevel: notification.AlertLevelWarning,
				DeviceID:   &deviceID,
				DeviceName: deviceName,
				Title:      "Configuration drift detected",
				Message:    msg,
				Timestamp:  time.Now(),
				Categories: []string{"configuration", "drift"},
				Metadata:   map[string]interface{}{"difference_count": differenceCount},
			})
		})
	}

	// Run the stored drift schedules when the operator turned the scheduler on
	scheduler, err := startScheduler(context.Background(), cfg, dbManager.GetDB(), apiHandler.ConfigService, logger)
	if err != nil {
		logger.WithFields(map[string]any{"error": err.Error(), "component": "drift"}).Error("Drift scheduler failed to start")
		log.Fatal("Drift scheduler failed to start:", err)
	}

	// Wire sync handlers for export/import functionality
	syncHandlers := api.NewSyncHandlers(syncEngine, logger)
	// Downloads are served only from the export root (the default one when none is configured)
	if cfg != nil {
		syncHandlers.SetExportBaseDir(cfg.ExportRoot())
	}
	apiHandler.ExportHandlers = syncHandlers
	importHandlers := api.NewImportHandlers(syncEngine, logger)
	// The backup check opens only backups under the same directory
	if cfg != nil && cfg.Export.OutputDirectory != "" {
		importHandlers.SetExportBaseDir(cfg.Export.OutputDirectory)
	}
	// A restore replaces the SQLite file at the next start; other databases answer 501
	if cfg != nil {
		importHandlers.SetRestoreDatabase(cfg.GetDatabaseConfig())
	}
	apiHandler.ImportHandlers = importHandlers
	// Protect the export, import and metrics endpoints with the one admin key
	apiHandler.ShareAdminKey()

	// Build security config from application config
	secCfg := securityConfigFrom(cfg)
	// Setup validation config based on main configuration
	valCfg := middleware.DefaultValidationConfig()
	if cfg != nil {
		valCfg.TestMode = cfg.Security.ValidationTestMode
	}

	// Setup routes with middleware using configured security
	router := api.SetupRoutesWithSecurity(apiHandler, logger, secCfg, valCfg)

	// Dev convenience: expose app-config.js to inject API base and (optionally) admin key.
	// This is enabled only when SHELLY_DEV_EXPOSE_ADMIN_KEY is set.
	router.HandleFunc("/app-config.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

		// Always set default API base to /api/v1 unless overridden by client
		_, _ = fmt.Fprintln(w, "window.__API_BASE__ = window.__API_BASE__ || '/api/v1';")

		if v := os.Getenv("SHELLY_DEV_EXPOSE_ADMIN_KEY"); strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
			// The live key, so a rotation shows here too (also one rotated from an empty start-up key)
			if key := apiHandler.AdminAPIKey(); key != "" {
				// Serialize as JSON string to safely escape characters
				if b, err := json.Marshal(key); err == nil {
					_, _ = fmt.Fprintf(w, "window.__ADMIN_KEY__ = %s;\n", string(b))
				}
			}
		}
	}).Methods("GET")

	// Serve UI
	// 1) If a built SPA exists (ui/dist), serve it at "/" with SPA fallback
	// 2) Otherwise, provide a minimal landing page and expose raw files under /ui/ for reference
	if _, err := os.Stat("./ui/dist/index.html"); err == nil {
		distDir := http.Dir("./ui/dist")
		fileServer := http.FileServer(distDir)

		// Serve static assets from dist (e.g., /assets/*, /favicon.ico)
		router.PathPrefix("/assets/").Handler(http.StripPrefix("/", fileServer))
		router.Handle("/favicon.ico", http.StripPrefix("/", fileServer))
		router.Handle("/robots.txt", http.StripPrefix("/", fileServer))

		// SPA fallback: for any non-API path, serve index.html
		router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/metrics") || p == "/app-config.js" {
				// Let other routes handle these (should have matched earlier)
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, "./ui/dist/index.html")
		}))
	} else {
		// Root landing page (no built UI available)
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Shelly Manager</title>
<style>body{font-family:system-ui,Segoe UI,Roboto,Helvetica,Arial,sans-serif;padding:24px;color:#0f172a}a{color:#2563eb;text-decoration:none}a:hover{text-decoration:underline}.note{color:#475569}</style>
</head><body>
<h1>Shelly Manager</h1>
<p>API is running. Explore <a href="/api/v1/version">/api/v1/version</a> or see docs under <code>docs/</code> in the repo. When an admin key is configured, the other API routes need a key, except /api/v1/metrics/prometheus.</p>
<p class="note">UI note: Run the SPA with <code>make ui-dev</code> (Vite) or build with <code>make ui-build</code> to serve <code>ui/dist</code> from the Go server.</p>
</body></html>`)
		}).Methods("GET")

		// Serve raw UI files (uncompiled) under /ui/ for development reference
		if _, err := os.Stat("./ui"); err == nil {
			fs := http.FileServer(http.Dir("./ui"))
			router.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/", fs))
		}
	}

	// Start WebSocket hub if metrics are enabled
	if metricsHandler != nil {
		// The WebSocket reads the same admin key through apiHandler.ShareAdminKey above
		wsHub := metricsHandler.GetWebSocketHub()
		if wsHub != nil {
			logger.WithFields(map[string]any{
				"component": "websocket",
			}).Info("Starting WebSocket hub for real-time metrics")

			// Start WebSocket hub in background
			go func() {
				ctx := context.Background()
				wsHub.Run(ctx)
			}()
		}
	}

	// Start background cleanup process for discovered devices
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // Cleanup every hour
		defer ticker.Stop()

		logger.WithFields(map[string]any{
			"component": "cleanup",
		}).Info("Starting discovered devices cleanup scheduler")

		for range ticker.C {
			if deleted, err := dbManager.CleanupExpiredDiscoveredDevices(); err != nil {
				logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "cleanup",
				}).Warn("Failed to cleanup expired discovered devices")
			} else if deleted > 0 {
				logger.WithFields(map[string]any{
					"deleted":   deleted,
					"component": "cleanup",
				}).Info("Scheduled cleanup completed for discovered devices")
			}
		}
	}()

	// Start server
	address := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.LogAppStart("1.0.0", address)

	fmt.Printf("Starting server on %s\n", address)
	fmt.Printf("Web interface: http://%s\n", address)
	// Note: Legacy dashboard removed. New SPA is served from Vite (dev) or ui/dist (prod).
	fmt.Printf("API base URL: http://%s/api/v1\n", address)

	// A signal ends the server cleanly only when the scheduler runs, so that the run in flight stops
	// between devices; with the scheduler off nothing here changes. A device read already started is
	// not interrupted (its client timeout bounds it, and Stop waits 15 seconds at most), so a run can
	// still be left running; the first poll after its recorded start is more than ten minutes old
	// marks it interrupted.
	serveCtx := context.Background()
	if scheduler != nil {
		var stopSignals context.CancelFunc
		serveCtx, stopSignals = signal.NotifyContext(serveCtx, os.Interrupt, syscall.SIGTERM)
		defer stopSignals()
	}
	stopScheduler := func() {
		if scheduler == nil {
			return
		}
		if err := scheduler.Stop(); err != nil {
			logger.WithFields(map[string]any{"error": err.Error(), "component": "drift"}).Warn("Drift scheduler did not stop cleanly")
		}
	}
	if err := serve(serveCtx, newHTTPServer(address, router), stopScheduler, shutdownTimeout); err != nil {
		logger.WithFields(map[string]any{
			"address":   address,
			"error":     err.Error(),
			"component": "server",
		}).Error("Server failed")
		log.Fatal("Server failed:", err)
	}
}

// The server's own limits, for a client that never finishes its request header or that holds an
// idle keep-alive connection. The router's request deadline starts only after the header is read.
// ReadTimeout and WriteTimeout stay unset on purpose: each is one limit for a whole request, so a
// value short enough to matter would cut a slow upload (up to 10 MiB), a download or the 5 minute
// bulk-apply answer. On a connection that already carried a request the idle limit comes first: the
// 10 seconds start once the first bytes of the next request have arrived.
const (
	serverReadHeaderTimeout = 10 * time.Second
	serverIdleTimeout       = 120 * time.Second
)

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return newHTTPServerWith(address, handler, serverReadHeaderTimeout, serverIdleTimeout)
}

func newHTTPServerWith(address string, handler http.Handler, readHeader, idle time.Duration) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeader,
		IdleTimeout:       idle,
	}
}

// countSuccessfulSteps counts successful provisioning steps
func countSuccessfulSteps(steps []provisioning.ProvisioningStep) int {
	count := 0
	for _, step := range steps {
		if step.Status == "success" {
			count++
		}
	}
	return count
}

// Initialize configuration, database, and services
func initApp() {
	var err error

	// Load configuration
	cfg, err = config.Load(configFile)
	if err != nil {
		log.Fatal("Failed to load config: ", err)
	}

	// Apply secret overrides (env and *_FILE). A *_FILE that cannot be read stops the start: with the
	// key left empty the guarded routes would be open.
	if err = secrets.ApplyToConfig(cfg); err != nil {
		log.Fatalf("Failed to read secrets: %v", err)
	}
	if err = cfg.ValidateAgentEnrolment(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Blocking on with the monitor off, or a zero block length or threshold, would leave an operator
	// believing a protection is working: stop here, naming the key.
	if err = cfg.ValidateSecurityMonitoring(); err != nil {
		log.Fatalf("Invalid security configuration: %v", err)
	}

	// Initialize logger from config
	logger, err = logging.New(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	// Set as default logger
	logging.SetDefault(logger)

	// Log configuration load
	logger.LogConfigLoad(configFile, nil)

	// Initialize database with logger using provider abstraction. Only the server's own start applies a
	// staged restore: a short-lived command may run while the server owns the database file.
	if sub, _, findErr := rootCmd.Find(os.Args[1:]); findErr == nil && sub.Name() == "server" {
		dbManager, err = database.NewManagerFromConfigWithLogger(cfg, logger)
	} else {
		dbManager, err = database.NewManagerKeepingRestore(cfg.GetDatabaseConfig(), logger)
	}
	if err != nil {
		logger.WithFields(map[string]any{
			"database":  cfg.DatabaseTarget(),
			"error":     err.Error(),
			"component": "database",
		}).Error("Failed to initialize database")
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize service with logger
	shellyService = service.NewServiceWithLogger(dbManager, cfg, logger)

	// Initialize notification service
	emailConfig := notification.EmailSMTPConfig{
		Host:     cfg.Notifications.Email.SMTPHost,
		Port:     cfg.Notifications.Email.SMTPPort,
		Username: cfg.Notifications.Email.SMTPUser,
		Password: cfg.Notifications.Email.SMTPPassword,
		From:     cfg.Notifications.Email.FromAddress,
		TLS:      cfg.Notifications.Email.TLS,
	}
	notificationService := notification.NewService(dbManager.GetDB(), logger, emailConfig)
	notificationHandler = notification.NewHandler(notificationService, logger)

	// Initialize metrics service if enabled
	if cfg.Metrics.Enabled {
		metricsService = metrics.NewService(dbManager.GetDB(), logger, nil)
		metricsHandler = metrics.NewHandler(metricsService, logger)

		// Start metrics collector if enabled
		if cfg.Metrics.CollectionInterval > 0 {
			collectionInterval := time.Duration(cfg.Metrics.CollectionInterval) * time.Second
			metricsCollector = metrics.NewCollector(metricsService, logger, collectionInterval)

			// Start collector in background
			go func() {
				ctx := context.Background()
				if err := metricsCollector.Start(ctx); err != nil {
					logger.WithFields(map[string]any{
						"error":     err.Error(),
						"component": "metrics",
					}).Error("Failed to start metrics collector")
				}
			}()
		}

		// Wire integration (7.2.d): emit notifications from metrics test alerts
		if metricsHandler != nil && notificationHandler != nil {
			metricsHandler.SetNotifier(func(ctx context.Context, alertType, severity, message string) {
				// Map severity to notification.AlertLevel
				level := notification.AlertLevelInfo
				switch severity {
				case "critical":
					level = notification.AlertLevelCritical
				case "warning", "high", "medium":
					level = notification.AlertLevelWarning
				}
				_ = notificationHandler.NotifyEvent(ctx, &notification.NotificationEvent{
					Type:       alertType,
					AlertLevel: level,
					Title:      fmt.Sprintf("Metrics alert: %s", alertType),
					Message:    message,
					Timestamp:  time.Now(),
					Categories: []string{"metrics", "alert"},
				})
			})
		}

		// (moved to startServer where apiHandler is available)

		logger.WithFields(map[string]any{
			"prometheus_enabled":  cfg.Metrics.PrometheusEnabled,
			"collection_interval": cfg.Metrics.CollectionInterval,
			"component":           "metrics",
		}).Info("Metrics service initialized")
	}

	// Initialize provisioning manager
	provisioningManager = provisioning.NewProvisioningManager(cfg, logger)

	// Create platform-specific network interface
	netInterface := provisioning.CreateNetworkInterface(logger)
	provisioningManager.SetNetworkInterface(netInterface)

	// Create Shelly device provisioner
	shellyProvisioner := provisioning.NewShellyProvisioner(logger, netInterface)
	provisioningManager.SetDeviceProvisioner(shellyProvisioner)

	// Initialize base plugin registry
	basePluginRegistry = plugins.NewRegistry(logger)

	// Initialize sync-specific plugin registry
	pluginRegistry = registry.NewPluginRegistry(basePluginRegistry, logger)

	// Register all sync plugins
	if err := pluginRegistry.RegisterAllPlugins(); err != nil {
		logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "plugins",
		}).Warn("Some sync plugins failed to register")
		// Continue startup even if some plugins fail to register
	} else {
		logger.WithFields(map[string]any{
			"component": "plugins",
		}).Info("All sync plugins registered successfully")
	}

	// Initialize sync engine and register plugins with it
	syncEngine = sync.NewSyncEngine(dbManager, logger)

	// Configure base directories for path traversal protection (if set)
	if cfg.Sync.ImportBaseDir != "" {
		syncEngine.SetImportBaseDir(cfg.Sync.ImportBaseDir)
		logger.WithFields(map[string]any{
			"import_base_dir": cfg.Sync.ImportBaseDir,
			"component":       "sync_engine",
		}).Info("Import base directory configured for path validation")
	}
	// One export root for plugins and downloads: sync.export_base_dir, else export.output_directory, else
	// sync.DefaultExportRoot. It is created here, so the path checks run against a directory that exists.
	if err := sync.EnsureExportRoot(cfg.ExportRoot()); err != nil {
		log.Fatal("Export directory cannot be created: ", err)
	}
	if root := cfg.ExportRoot(); root != "" {
		syncEngine.SetExportBaseDir(root)
		logger.WithFields(map[string]any{
			"export_base_dir": root,
			"component":       "sync_engine",
		}).Info("Export base directory configured for path validation")
	}

	// Register sync plugins directly with the sync engine using the old interface
	syncPlugins := []sync.SyncPlugin{
		backup.NewPlugin(),
		gitops.NewPlugin(),
		opnsense.NewPlugin(),
		sma.NewPlugin(),
		jsonexport.NewPlugin(),
		yamlexport.NewPlugin(),
	}

	for _, plugin := range syncPlugins {
		if err := syncEngine.RegisterPlugin(plugin); err != nil {
			logger.WithFields(map[string]any{
				"plugin":    plugin.Info().Name,
				"error":     err.Error(),
				"component": "sync_engine",
			}).Warn("Failed to register plugin with sync engine")
		} else {
			logger.WithFields(map[string]any{
				"plugin":    plugin.Info().Name,
				"component": "sync_engine",
			}).Info("Plugin registered with sync engine")
		}
	}

	// Inject database manager into the backup plugin registered in the sync engine
	if p, err := syncEngine.GetPlugin("backup"); err == nil {
		if bp, ok := p.(*backup.BackupPlugin); ok {
			// Use the existing adapter to satisfy backup.DatabaseManagerInterface
			bp.SetDatabaseManager(&registry.DatabaseManagerAdapter{Manager: dbManager})
			logger.WithFields(map[string]any{"component": "sync_engine", "plugin": "backup"}).Info("Database manager injected into backup plugin")
		} else {
			logger.WithFields(map[string]any{"component": "sync_engine", "plugin": "backup"}).Warn("Registered backup plugin is not *backup.BackupPlugin; cannot inject DB manager")
		}
	} else if err != nil {
		logger.WithFields(map[string]any{"component": "sync_engine", "plugin": "backup", "error": err.Error()}).Warn("Backup plugin not found for DB manager injection")
	}

	// Register backup plugin with database manager for enhanced functionality
	if err := pluginRegistry.RegisterPluginWithDatabaseManager(dbManager); err != nil {
		logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "plugins",
		}).Warn("Failed to register backup plugin with database manager")
	}

	logger.WithFields(map[string]any{
		"database":  cfg.DatabaseTarget(),
		"networks":  cfg.Discovery.Networks,
		"component": "app",
	}).Info("Shelly Manager initialized")

	fmt.Printf("Shelly Manager initialized\n")
	fmt.Printf("Database: %s\n", cfg.DatabaseTarget())
	if len(cfg.Discovery.Networks) > 0 {
		fmt.Printf("Discovery networks: %v\n", cfg.Discovery.Networks)
	}
}

func init() {
	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "",
		"config file (default is ./configs/shelly-manager.yaml)")

	// Add provisioning command flags
	provisionCmd.Flags().String("name", "", "Device name (auto-generated if not specified)")
	provisionCmd.Flags().Bool("enable-auth", false, "Enable device authentication")
	provisionCmd.Flags().String("auth-user", "admin", "Authentication username")
	provisionCmd.Flags().String("auth-password", "", "Authentication password")
	provisionCmd.Flags().Bool("enable-cloud", false, "Enable cloud connectivity")
	provisionCmd.Flags().Bool("enable-mqtt", false, "Enable MQTT")
	provisionCmd.Flags().String("mqtt-server", "", "MQTT server address")
	provisionCmd.Flags().Int("timeout", 300, "Provisioning timeout in seconds")

	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(scanAPCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(serverCmd)
}

func main() {
	// Initialize before running commands
	cobra.OnInitialize(initApp)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
