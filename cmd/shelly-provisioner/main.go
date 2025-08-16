package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/provisioning"
)

// Global variables
var (
	provisioningManager *provisioning.ProvisioningManager
	shellyProvisioner   *provisioning.ShellyProvisioner
	netInterface        provisioning.NetworkInterface
	cfg                 *config.Config
	logger              *logging.Logger
	configFile          string
	apiURL              string
	apiKey              string
)

// Root command
var rootCmd = &cobra.Command{
	Use:   "shelly-provisioner",
	Short: "Shelly device provisioning agent",
	Long: `A specialized provisioning agent for discovering and configuring 
Shelly smart home devices in AP mode and connecting them to WiFi networks.

This agent is designed to run on systems with WiFi interfaces and can
operate as a standalone tool or as an agent connected to the main
shelly-manager API server.`,
}

// Agent command - run as a service connected to main API
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run as provisioning agent connected to main API",
	Long: `Run as a provisioning agent that connects to the main shelly-manager
API server and polls for provisioning tasks. This mode is intended for
deployment on WiFi-capable hosts that can manage device provisioning
while the main API server runs in a container environment.`,
	Run: func(cmd *cobra.Command, args []string) {
		runAgent()
	},
}

// Scan AP command - scan for devices in AP mode
var scanAPCmd = &cobra.Command{
	Use:   "scan-ap",
	Short: "Scan for Shelly devices in AP mode",
	Long: `Scan for unprovisioned Shelly devices broadcasting WiFi networks
in AP mode. This command identifies devices that need provisioning.`,
	Run: func(cmd *cobra.Command, args []string) {
		scanForAPDevices()
	},
}

// Provision command - provision specific devices
var provisionCmd = &cobra.Command{
	Use:   "provision <ssid> [password]",
	Short: "Provision discovered devices to join WiFi network",
	Long: `Provision unprovisioned Shelly devices to join a specific WiFi network.
Devices must be in AP mode and accessible via WiFi interface.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		targetSSID := args[0]
		targetPassword := ""
		if len(args) > 1 {
			targetPassword = args[1]
		}
		provisionDevices(cmd, targetSSID, targetPassword)
	},
}

// Status command - check agent health and connectivity
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check agent status and connectivity",
	Long: `Check the provisioning agent status, including network interface
availability, API server connectivity (if configured), and system health.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkStatus()
	},
}

// runAgent runs the provisioning agent service
func runAgent() {
	if apiURL == "" {
		log.Fatal("API URL required for agent mode. Use --api-url flag or set via config.")
	}

	logger.WithFields(map[string]any{
		"api_url":   apiURL,
		"component": "agent",
	}).Info("Starting provisioning agent")

	fmt.Printf("Starting provisioning agent...\n")
	fmt.Printf("API Server: %s\n", apiURL)
	fmt.Printf("Agent ID: %s\n", generateAgentID())

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.WithFields(map[string]any{
			"component": "agent",
		}).Info("Received shutdown signal, stopping agent")
		fmt.Println("\nReceived shutdown signal, stopping agent...")
		cancel()
	}()

	// Main agent loop
	pollInterval := 30 * time.Second
	if cfg.Provisioning.ProvisionInterval > 0 {
		pollInterval = time.Duration(cfg.Provisioning.ProvisionInterval) * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fmt.Printf("Agent polling every %v\n", pollInterval)
	logger.WithFields(map[string]any{
		"poll_interval": pollInterval,
		"component":     "agent",
	}).Info("Agent started successfully")

	// Initial registration attempt
	if err := registerWithAPI(); err != nil {
		logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "agent",
		}).Warn("Failed to register with API server")
		fmt.Printf("Warning: Failed to register with API server: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			logger.WithFields(map[string]any{
				"component": "agent",
			}).Info("Agent shutdown complete")
			fmt.Println("Agent shutdown complete")
			return
		case <-ticker.C:
			if err := pollForTasks(ctx); err != nil {
				logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "agent",
				}).Warn("Failed to poll for tasks")
			}
		}
	}
}

// scanForAPDevices scans for devices in AP mode
func scanForAPDevices() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Scanning for Shelly devices in AP mode...")
	logger.WithFields(map[string]any{
		"component": "scan",
	}).Info("Starting AP mode device scan")

	devices, err := shellyProvisioner.DiscoverUnprovisionedDevices(ctx)
	if err != nil {
		logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "scan",
		}).Error("Failed to scan for AP devices")
		fmt.Printf("Error: Failed to scan for devices: %v\n", err)
		return
	}

	if len(devices) == 0 {
		fmt.Println("No unprovisioned Shelly devices found in AP mode")
		logger.WithFields(map[string]any{
			"devices_found": 0,
			"component":     "scan",
		}).Info("Scan completed - no devices found")
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

	logger.WithFields(map[string]any{
		"devices_found": len(devices),
		"component":     "scan",
	}).Info("Scan completed successfully")
}

// provisionDevices provisions discovered devices
func provisionDevices(cmd *cobra.Command, targetSSID, targetPassword string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger.WithFields(map[string]any{
		"target_ssid": targetSSID,
		"component":   "provision",
	}).Info("Starting device provisioning")

	fmt.Printf("Searching for unprovisioned Shelly devices...\n")

	devices, err := shellyProvisioner.DiscoverUnprovisionedDevices(ctx)
	if err != nil {
		logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "provision",
		}).Error("Failed to discover devices for provisioning")
		fmt.Printf("Error: Failed to discover devices: %v\n", err)
		return
	}

	if len(devices) == 0 {
		fmt.Println("No unprovisioned Shelly devices found")
		logger.WithFields(map[string]any{
			"devices_found": 0,
			"component":     "provision",
		}).Info("No devices found for provisioning")
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
			logger.WithFields(map[string]any{
				"device_mac": device.MAC,
				"error":      err.Error(),
				"component":  "provision",
			}).Error("Device provisioning failed")
			fmt.Printf("❌ Provisioning failed: %v\n", err)
			if result != nil {
				fmt.Printf("   Steps completed: %d/%d\n",
					countSuccessfulSteps(result.Steps), len(result.Steps))
			}
			failCount++
			continue
		}

		logger.WithFields(map[string]any{
			"device_mac":  device.MAC,
			"device_name": result.DeviceName,
			"device_ip":   result.DeviceIP,
			"duration":    result.Duration,
			"component":   "provision",
		}).Info("Device provisioning completed successfully")

		fmt.Printf("✅ Provisioning completed successfully!\n")
		fmt.Printf("   Device Name: %s\n", result.DeviceName)
		fmt.Printf("   New IP: %s\n", result.DeviceIP)
		fmt.Printf("   Duration: %s\n", result.Duration.String())
		fmt.Printf("   Steps: %d/%d successful\n",
			countSuccessfulSteps(result.Steps), len(result.Steps))

		successCount++
	}

	logger.WithFields(map[string]any{
		"successful": successCount,
		"failed":     failCount,
		"total":      len(devices),
		"component":  "provision",
	}).Info("Provisioning operation completed")

	fmt.Printf("\nProvisioning Summary:\n")
	fmt.Printf("✅ Successful: %d\n", successCount)
	fmt.Printf("❌ Failed: %d\n", failCount)
	fmt.Printf("📊 Total: %d\n", len(devices))
}

// checkStatus checks agent status and connectivity
func checkStatus() {
	fmt.Println("Shelly Provisioner Status")
	fmt.Println(strings.Repeat("=", 50))

	// Check network interface availability
	if netInterface != nil {
		fmt.Printf("✅ Network Interface: Available\n")

		// Test WiFi scanning capability
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if networks, err := netInterface.GetAvailableNetworks(ctx); err != nil {
			fmt.Printf("❌ WiFi Scanning: Failed (%v)\n", err)
			logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "status",
			}).Warn("WiFi scanning test failed")
		} else {
			fmt.Printf("✅ WiFi Scanning: Working (%d networks found)\n", len(networks))
		}
	} else {
		fmt.Printf("❌ Network Interface: Not available\n")
	}

	// Check API connectivity if configured
	if apiURL != "" {
		fmt.Printf("📡 API Server: %s\n", apiURL)
		if err := testAPIConnectivity(); err != nil {
			fmt.Printf("❌ API Connectivity: Failed (%v)\n", err)
			logger.WithFields(map[string]any{
				"api_url":   apiURL,
				"error":     err.Error(),
				"component": "status",
			}).Warn("API connectivity test failed")
		} else {
			fmt.Printf("✅ API Connectivity: Working\n")
		}
	} else {
		fmt.Printf("⚠️  API Server: Not configured (standalone mode)\n")
	}

	// Show configuration summary
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Config File: %s\n", getConfigFilePath())
	fmt.Printf("  Agent ID: %s\n", generateAgentID())

	if cfg != nil {
		fmt.Printf("  Poll Interval: %ds\n", cfg.Provisioning.ProvisionInterval)
		fmt.Printf("  Auto Provision: %t\n", cfg.Provisioning.AutoProvision)
	}

	logger.WithFields(map[string]any{
		"component": "status",
	}).Info("Status check completed")
}

// Helper functions
func countSuccessfulSteps(steps []provisioning.ProvisioningStep) int {
	count := 0
	for _, step := range steps {
		if step.Status == "success" {
			count++
		}
	}
	return count
}

func generateAgentID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("provisioner-%s-%d", hostname, os.Getpid())
}

func getConfigFilePath() string {
	if configFile != "" {
		return configFile
	}
	return "default (shelly-provisioner.yaml)"
}

func registerWithAPI() error {
	// TODO: Implement API registration
	logger.WithFields(map[string]any{
		"component": "agent",
	}).Info("API registration not yet implemented")
	return nil
}

func pollForTasks(ctx context.Context) error {
	// TODO: Implement task polling from API
	logger.WithFields(map[string]any{
		"component": "agent",
	}).Debug("Task polling not yet implemented")
	return nil
}

func testAPIConnectivity() error {
	// TODO: Implement API connectivity test
	return nil
}

// Initialize configuration and services
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

	// Initialize provisioning manager
	provisioningManager = provisioning.NewProvisioningManager(cfg, logger)

	// Create platform-specific network interface
	netInterface = provisioning.CreateNetworkInterface(logger)
	provisioningManager.SetNetworkInterface(netInterface)

	// Create Shelly device provisioner
	shellyProvisioner = provisioning.NewShellyProvisioner(logger, netInterface)
	provisioningManager.SetDeviceProvisioner(shellyProvisioner)

	logger.WithFields(map[string]any{
		"agent_id":  generateAgentID(),
		"component": "app",
	}).Info("Shelly Provisioner initialized")

	fmt.Printf("Shelly Provisioner initialized\n")
	fmt.Printf("Agent ID: %s\n", generateAgentID())
	if netInterface != nil {
		fmt.Printf("Network interface: Available\n")
	}
}

func init() {
	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "",
		"config file (default is shelly-provisioner.yaml)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "",
		"main API server URL (required for agent mode)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "",
		"API authentication key")

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
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(scanAPCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	// Initialize before running commands
	cobra.OnInitialize(initApp)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
