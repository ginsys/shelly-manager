package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

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
	"github.com/ginsys/shelly-manager/internal/plugins/sync/opnsense"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/registry"
	"github.com/ginsys/shelly-manager/internal/provisioning"
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

		devices, err := shellyService.DiscoverDevices(network)
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

			// Check if device already exists by MAC
			_, err := dbManager.GetDeviceByMAC(device.MAC)
			if err != nil && err == gorm.ErrRecordNotFound {
				if addErr := dbManager.AddDevice(&device); addErr != nil {
					log.Printf("Failed to add device %s: %v", device.IP, addErr)
				} else {
					fmt.Printf("✓ Added to database\n")
				}
			} else if err == nil {
				// Update existing device with new IP if changed
				existingDevice, _ := dbManager.GetDeviceByMAC(device.MAC)
				if existingDevice.IP != device.IP {
					existingDevice.IP = device.IP
					existingDevice.LastSeen = time.Now()
					if err := dbManager.UpdateDevice(existingDevice); err != nil {
						fmt.Printf("✗ Failed to update device: %v\n", err)
					} else {
						fmt.Printf("✓ Updated IP address in database\n")
					}
				} else {
					fmt.Printf("• Already in database\n")
				}
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
		ip := args[0]
		name := "Unknown Device"
		if len(args) > 1 {
			name = args[1]
		}

		// Try to discover the specific device first
		devices, err := shellyService.DiscoverDevices(ip + "/32")
		if err != nil || len(devices) == 0 {
			log.Fatal("Could not discover device at", ip)
		}

		device := devices[0]
		if name != "Unknown Device" {
			device.Name = name
		}

		if err := dbManager.AddDevice(&device); err != nil {
			log.Fatal("Failed to add device:", err)
		}

		fmt.Printf("Added device: %s (%s) at %s\n", device.Name, device.Type, device.IP)
	},
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

	// Wire sync handlers for export/import functionality
	syncHandlers := api.NewSyncHandlers(syncEngine, logger)
	apiHandler.ExportHandlers = syncHandlers
	apiHandler.ImportHandlers = api.NewImportHandlers(syncEngine, logger)

	// Build security config from application config
	secCfg := middleware.DefaultSecurityConfig()
	if cfg != nil {
		secCfg.UseProxyHeaders = cfg.Security.UseProxyHeaders
		secCfg.TrustedProxies = cfg.Security.TrustedProxies
		secCfg.CORSAllowedOrigins = cfg.Security.CORS.AllowedOrigins
		if len(cfg.Security.CORS.AllowedMethods) > 0 {
			secCfg.CORSAllowedMethods = cfg.Security.CORS.AllowedMethods
		}
		if len(cfg.Security.CORS.AllowedHeaders) > 0 {
			secCfg.CORSAllowedHeaders = cfg.Security.CORS.AllowedHeaders
		}
		if cfg.Security.CORS.MaxAge > 0 {
			secCfg.CORSMaxAge = cfg.Security.CORS.MaxAge
		}
	}
	// Setup routes with middleware using configured security
	router := api.SetupRoutesWithSecurity(apiHandler, logger, secCfg, middleware.DefaultValidationConfig())

	// Start WebSocket hub if metrics are enabled
	if metricsHandler != nil {
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
	fmt.Printf("Dashboard: http://%s/dashboard.html\n", address)
	fmt.Printf("API base URL: http://%s/api/v1\n", address)

	if err := http.ListenAndServe(address, router); err != nil {
		logger.WithFields(map[string]any{
			"address":   address,
			"error":     err.Error(),
			"component": "server",
		}).Error("Server failed to start")
		log.Fatal("Server failed to start:", err)
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
		log.Fatal("Failed to load config:", err)
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

	// Initialize database with logger using provider abstraction
	dbManager, err = database.NewManagerFromConfigWithLogger(cfg, logger)
	if err != nil {
		logger.WithFields(map[string]any{
			"db_path":   cfg.Database.Path,
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

	// Register sync plugins directly with the sync engine using the old interface
	syncPlugins := []sync.SyncPlugin{
		backup.NewPlugin(),
		gitops.NewPlugin(),
		opnsense.NewPlugin(),
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

	// Register backup plugin with database manager for enhanced functionality
	if err := pluginRegistry.RegisterPluginWithDatabaseManager(dbManager); err != nil {
		logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "plugins",
		}).Warn("Failed to register backup plugin with database manager")
	}

	logger.WithFields(map[string]any{
		"db_path":   cfg.Database.Path,
		"networks":  cfg.Discovery.Networks,
		"component": "app",
	}).Info("Shelly Manager initialized")

	fmt.Printf("Shelly Manager initialized\n")
	fmt.Printf("Database: %s\n", cfg.Database.Path)
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
