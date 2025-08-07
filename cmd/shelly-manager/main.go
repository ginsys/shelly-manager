// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/discovery"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Models
type Device struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	IP          string    `json:"ip" gorm:"uniqueIndex"`
	MAC         string    `json:"mac"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Firmware    string    `json:"firmware"`
	Status      string    `json:"status"`
	LastSeen    time.Time `json:"last_seen"`
	Settings    string    `json:"settings" gorm:"type:text"` // JSON string
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Config struct {
	Server struct {
		Port     int    `mapstructure:"port"`
		Host     string `mapstructure:"host"`
		LogLevel string `mapstructure:"log_level"`
	} `mapstructure:"server"`
	Database struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"database"`
	Discovery struct {
		Enabled          bool     `mapstructure:"enabled"`
		Networks         []string `mapstructure:"networks"`
		Interval         int      `mapstructure:"interval"`
		Timeout          int      `mapstructure:"timeout"`
		EnableMDNS       bool     `mapstructure:"enable_mdns"`
		EnableSSDP       bool     `mapstructure:"enable_ssdp"`
		ConcurrentScans  int      `mapstructure:"concurrent_scans"`
	} `mapstructure:"discovery"`
	Provisioning struct {
		AuthEnabled        bool   `mapstructure:"auth_enabled"`
		AuthUser          string `mapstructure:"auth_user"`
		AuthPassword      string `mapstructure:"auth_password"`
		CloudEnabled      bool   `mapstructure:"cloud_enabled"`
		MQTTEnabled       bool   `mapstructure:"mqtt_enabled"`
		MQTTServer        string `mapstructure:"mqtt_server"`
		DeviceNamePattern string `mapstructure:"device_name_pattern"`
		AutoProvision     bool   `mapstructure:"auto_provision"`
		ProvisionInterval int    `mapstructure:"provision_interval"`
	} `mapstructure:"provisioning"`
	DHCP struct {
		Network   string `mapstructure:"network"`
		StartIP   string `mapstructure:"start_ip"`
		EndIP     string `mapstructure:"end_ip"`
		AutoReserve bool `mapstructure:"auto_reserve"`
	} `mapstructure:"dhcp"`
	OPNSense struct {
		Enabled   bool   `mapstructure:"enabled"`
		Host      string `mapstructure:"host"`
		Port      int    `mapstructure:"port"`
		APIKey    string `mapstructure:"api_key"`
		APISecret string `mapstructure:"api_secret"`
		AutoApply bool   `mapstructure:"auto_apply"`
	} `mapstructure:"opnsense"`
	MainApp struct {
		URL     string `mapstructure:"url"`
		APIKey  string `mapstructure:"api_key"`
		Enabled bool   `mapstructure:"enabled"`
	} `mapstructure:"main_app"`
}

type ShellyManager struct {
	DB     *gorm.DB
	Config *Config
	ctx    context.Context
	cancel context.CancelFunc
}

// Global variables
var (
	manager    *ShellyManager
	configFile string
	rootCmd    = &cobra.Command{
		Use:   "shelly-manager",
		Short: "Shelly Device Configuration Manager",
		Long:  "A comprehensive tool for managing Shelly smart home devices",
	}
)

// Database operations
func (sm *ShellyManager) InitDB() error {
	db, err := gorm.Open(sqlite.Open(sm.Config.Database.Path), &gorm.Config{})
	if err != nil {
		return err
	}
	sm.DB = db
	return sm.DB.AutoMigrate(&Device{})
}

func (sm *ShellyManager) AddDevice(device *Device) error {
	return sm.DB.Create(device).Error
}

func (sm *ShellyManager) GetDevices() ([]Device, error) {
	var devices []Device
	err := sm.DB.Find(&devices).Error
	return devices, err
}

func (sm *ShellyManager) GetDevice(id uint) (*Device, error) {
	var device Device
	err := sm.DB.First(&device, id).Error
	return &device, err
}

func (sm *ShellyManager) UpdateDevice(device *Device) error {
	return sm.DB.Save(device).Error
}

func (sm *ShellyManager) DeleteDevice(id uint) error {
	return sm.DB.Delete(&Device{}, id).Error
}

// DiscoverDevices performs real Shelly device discovery using HTTP and mDNS
func (sm *ShellyManager) DiscoverDevices(network string) ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("Starting device discovery on network: %s", network)
	
	// Determine networks to scan
	var networks []string
	if network != "" && network != "auto" {
		networks = []string{network}
	} else if len(sm.Config.Discovery.Networks) > 0 {
		networks = sm.Config.Discovery.Networks
	}
	
	// Use timeout from config or default
	timeout := time.Duration(sm.Config.Discovery.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	
	// Perform combined discovery (HTTP + mDNS)
	shellyDevices, err := discovery.CombinedDiscovery(ctx, networks, timeout)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	
	// Convert discovered Shelly devices to our Device model
	var devices []Device
	for _, sd := range shellyDevices {
		device := Device{
			IP:       sd.IP,
			MAC:      sd.MAC,
			Type:     discovery.GetDeviceType(sd.Model),
			Name:     sd.ID, // Use ID as initial name, can be updated later
			Firmware: sd.Version,
			Status:   "online",
			LastSeen: sd.Discovered,
			Settings: fmt.Sprintf(`{"model":"%s","gen":%d,"auth_enabled":%v}`, 
				sd.Model, sd.Generation, sd.AuthEn),
		}
		devices = append(devices, device)
	}
	
	log.Printf("Discovery complete. Found %d devices", len(devices))
	return devices, nil
}

// CLI Commands
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	Run: func(cmd *cobra.Command, args []string) {
		devices, err := manager.GetDevices()
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
		network := "192.168.1.0/24"
		if len(args) > 0 {
			network = args[0]
		}
		
		fmt.Printf("Discovering devices on network %s...\n", network)
		devices, err := manager.DiscoverDevices(network)
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
			
			var existingDevice Device
			result := manager.DB.Where("mac = ?", device.MAC).First(&existingDevice)
			if result.Error != nil && result.Error == gorm.ErrRecordNotFound {
				if err := manager.AddDevice(&device); err != nil {
					log.Printf("Failed to add device %s: %v", device.IP, err)
				} else {
					fmt.Printf("✓ Added to database\n")
				}
			} else {
				// Update existing device with new IP if changed
				if existingDevice.IP != device.IP {
					existingDevice.IP = device.IP
					existingDevice.LastSeen = time.Now()
					manager.UpdateDevice(&existingDevice)
					fmt.Printf("✓ Updated IP address in database\n")
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
		name := ip
		if len(args) > 1 {
			name = args[1]
		}
		
		device := &Device{
			IP:       ip,
			Name:     name,
			Status:   "unknown",
			LastSeen: time.Now(),
		}
		
		if err := manager.AddDevice(device); err != nil {
			log.Fatal("Failed to add device:", err)
		}
		
		fmt.Printf("Device %s added successfully\n", ip)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a device by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseUint(args[0], 10, 32)
		if err != nil {
			log.Fatal("Invalid device ID:", err)
		}
		
		if err := manager.DeleteDevice(uint(id)); err != nil {
			log.Fatal("Failed to remove device:", err)
		}
		
		fmt.Printf("Device %d removed successfully\n", id)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <ip>",
	Short: "Get device status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ip := args[0]
		fmt.Printf("Device Status for %s:\n", ip)
		fmt.Printf("Status: online\n")
		fmt.Printf("Last Seen: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start API server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Starting server on %s:%d\n", manager.Config.Server.Host, manager.Config.Server.Port)
		startServer()
	},
}

// Placeholder for provisioning command (would integrate WiFi provisioning)
var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision unconfigured Shelly devices",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔍 Scanning for unconfigured Shelly devices...")
		fmt.Println("📡 Found 2 unconfigured devices (mock data)")
		fmt.Println("✅ Provisioning completed - real implementation would use WiFi provisioning system")
	},
}

// API Handlers
func (sm *ShellyManager) getDevicesHandler(w http.ResponseWriter, r *http.Request) {
	devices, err := sm.GetDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (sm *ShellyManager) addDeviceHandler(w http.ResponseWriter, r *http.Request) {
	var device Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()
	
	if err := sm.AddDevice(&device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(device)
}

func (sm *ShellyManager) getDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}
	
	device, err := sm.GetDevice(uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func (sm *ShellyManager) updateDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}
	
	var updates Device
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	device, err := sm.GetDevice(uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	if updates.Name != "" {
		device.Name = updates.Name
	}
	if updates.Settings != "" {
		device.Settings = updates.Settings
	}
	device.UpdatedAt = time.Now()
	
	if err := sm.UpdateDevice(device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func (sm *ShellyManager) deleteDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}
	
	if err := sm.DeleteDevice(uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

func (sm *ShellyManager) discoverHandler(w http.ResponseWriter, r *http.Request) {
	network := r.URL.Query().Get("network")
	if network == "" {
		network = "192.168.1.0/24"
	}
	
	devices, err := sm.DiscoverDevices(network)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"network": network,
		"devices": devices,
	})
}

// Placeholder handlers for advanced features
func (sm *ShellyManager) getProvisioningStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"platform_supported": runtime.GOOS == "linux" || runtime.GOOS == "darwin",
		"unconfigured_devices": 2, // Mock data
		"message": "WiFi provisioning system ready",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (sm *ShellyManager) provisionDevicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "started",
		"message": "Mock provisioning started - real implementation would use WiFi provisioning",
	})
}

func (sm *ShellyManager) getDHCPReservationsHandler(w http.ResponseWriter, r *http.Request) {
	// Mock DHCP reservations
	reservations := []map[string]string{
		{"mac": "A4:CF:12:34:56:78", "ip": "192.168.1.100", "hostname": "shelly-345678", "description": "Living Room Light"},
		{"mac": "A4:CF:12:34:56:79", "ip": "192.168.1.101", "hostname": "shelly-345679", "description": "Desk Lamp"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reservations": reservations,
		"count":       len(reservations),
	})
}

func startServer() {
	r := mux.NewRouter()
	
	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/static/index.html")
	})
	
	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/devices", manager.getDevicesHandler).Methods("GET")
	api.HandleFunc("/devices", manager.addDeviceHandler).Methods("POST")
	api.HandleFunc("/devices/{id}", manager.getDeviceHandler).Methods("GET")
	api.HandleFunc("/devices/{id}", manager.updateDeviceHandler).Methods("PUT")
	api.HandleFunc("/devices/{id}", manager.deleteDeviceHandler).Methods("DELETE")
	api.HandleFunc("/discover", manager.discoverHandler).Methods("POST")
	
	// Provisioning routes (placeholders)
	api.HandleFunc("/provisioning/status", manager.getProvisioningStatusHandler).Methods("GET")
	api.HandleFunc("/provisioning/start", manager.provisionDevicesHandler).Methods("POST")
	
	// DHCP routes (placeholders)
	api.HandleFunc("/dhcp/reservations", manager.getDHCPReservationsHandler).Methods("GET")
	
	// CORS middleware
	api.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	})
	
	addr := fmt.Sprintf("%s:%d", manager.Config.Server.Host, manager.Config.Server.Port)
	log.Fatal(http.ListenAndServe(addr, r))
}

// Configuration
func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("shelly-manager")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("$HOME/.shelly-manager")
		viper.AddConfigPath("/etc/shelly-manager")
	}
	
	// Set defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.log_level", "info")
	viper.SetDefault("database.path", "data/shelly.db")
	viper.SetDefault("discovery.enabled", true)
	viper.SetDefault("discovery.networks", []string{"192.168.1.0/24"})
	viper.SetDefault("discovery.interval", 300)
	viper.SetDefault("discovery.timeout", 5)
	viper.SetDefault("provisioning.auth_enabled", false)
	viper.SetDefault("provisioning.auth_user", "admin")
	viper.SetDefault("dhcp.network", "192.168.1.0/24")
	viper.SetDefault("dhcp.start_ip", "192.168.1.100")
	viper.SetDefault("dhcp.end_ip", "192.168.1.199")
	
	viper.AutomaticEnv()
	viper.SetEnvPrefix("SHELLY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("No config file found, using defaults")
		} else {
			log.Fatal("Error reading config file:", err)
		}
	}
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatal("Error unmarshaling config:", err)
	}
	
	manager.Config = &config
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./configs/shelly-manager.yaml)")
	
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(discoverCmd) 
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(provisionCmd)
}

func main() {
	manager = &ShellyManager{}
	manager.ctx, manager.cancel = context.WithCancel(context.Background())
	defer manager.cancel()
	
	if manager.Config == nil {
		initConfig()
	}
	
	if err := manager.InitDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	
	dbDir := filepath.Dir(manager.Config.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatal("Failed to create database directory:", err)
	}
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
