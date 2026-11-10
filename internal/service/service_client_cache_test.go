package service

import (
	"net"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/database"
)

const cacheTestSettings = `{"model":"SHSW-25","gen":1,"auth_enabled":false}`

func skipWithoutSockets(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	}
	_ = ln.Close()
}

// The cache is keyed by address. When a device moves, the entry at its old address must go, or a
// client built with the device's old credentials stays reachable under an address another device
// can take.
func TestShellyService_getClient_IPChangeEvictsOldAddress(t *testing.T) {
	skipWithoutSockets(t)
	oldServer := createMockShellyConfigServer()
	defer oldServer.Close()
	newServer := createMockShellyConfigServer()
	defer newServer.Close()
	oldIP := strings.TrimPrefix(oldServer.URL, "http://")
	newIP := strings.TrimPrefix(newServer.URL, "http://")

	service := NewService(createTestDB(t), createTestConfig())
	device := &database.Device{ID: 7, IP: oldIP, Settings: cacheTestSettings}

	if _, err := service.getClient(device); err != nil {
		t.Fatalf("getClient at the old address: %v", err)
	}
	if _, cached := service.clients[oldIP]; !cached {
		t.Fatal("the client was not cached at the old address")
	}

	device.IP = newIP
	if _, err := service.getClient(device); err != nil {
		t.Fatalf("getClient at the new address: %v", err)
	}
	if _, cached := service.clients[oldIP]; cached {
		t.Error("the entry at the device's previous address was kept")
	}
	if _, cached := service.clients[newIP]; !cached {
		t.Error("the client was not cached at the new address")
	}
	if _, owned := service.clientOwner[oldIP]; owned {
		t.Error("the owner record of the previous address was kept")
	}
	if got := service.clientAddr[device.ID]; got != newIP {
		t.Errorf("clientAddr[%d] = %q, want %q", device.ID, got, newIP)
	}
}

// A device that takes an address another device left must get a client of its own, never the
// client built for the previous holder.
func TestShellyService_getClient_AddressTakenByAnotherDevice(t *testing.T) {
	skipWithoutSockets(t)
	server := createMockShellyConfigServer()
	defer server.Close()
	ip := strings.TrimPrefix(server.URL, "http://")

	service := NewService(createTestDB(t), createTestConfig())
	first := &database.Device{ID: 1, IP: ip, Settings: cacheTestSettings}
	second := &database.Device{ID: 2, IP: ip, Settings: cacheTestSettings}

	firstClient, err := service.getClient(first)
	if err != nil {
		t.Fatalf("getClient for the first device: %v", err)
	}
	secondClient, err := service.getClient(second)
	if err != nil {
		t.Fatalf("getClient for the second device: %v", err)
	}
	if secondClient == firstClient {
		t.Fatal("the second device was handed the client built for the first one")
	}
	if got := service.clientOwner[ip]; got != second.ID {
		t.Errorf("clientOwner[%q] = %d, want %d", ip, got, second.ID)
	}
	if _, tracked := service.clientAddr[first.ID]; tracked {
		t.Error("the first device is still recorded at an address it no longer holds")
	}
}

// The same device at the same address keeps its cached client.
func TestShellyService_getClient_SameDeviceReusesClient(t *testing.T) {
	skipWithoutSockets(t)
	server := createMockShellyConfigServer()
	defer server.Close()
	ip := strings.TrimPrefix(server.URL, "http://")

	service := NewService(createTestDB(t), createTestConfig())
	device := &database.Device{ID: 3, IP: ip, Settings: cacheTestSettings}

	first, err := service.getClient(device)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.getClient(device)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("an unchanged device must reuse its cached client")
	}
}

// Clearing the cache must clear the owner records with the clients, or a later lookup trusts a
// record for an entry that is gone.
func TestShellyService_ClearClientCache_ClearsOwnerRecords(t *testing.T) {
	skipWithoutSockets(t)
	server := createMockShellyConfigServer()
	defer server.Close()
	ip := strings.TrimPrefix(server.URL, "http://")

	service := NewService(createTestDB(t), createTestConfig())
	device := &database.Device{ID: 4, IP: ip, Settings: cacheTestSettings}
	if _, err := service.getClient(device); err != nil {
		t.Fatal(err)
	}

	service.ClearClientCache(ip)
	if len(service.clients) != 0 || len(service.clientOwner) != 0 || len(service.clientAddr) != 0 {
		t.Errorf("clear by address left clients=%d owners=%d addrs=%d",
			len(service.clients), len(service.clientOwner), len(service.clientAddr))
	}

	if _, err := service.getClient(device); err != nil {
		t.Fatal(err)
	}
	service.ClearClientCache("")
	if len(service.clients) != 0 || len(service.clientOwner) != 0 || len(service.clientAddr) != 0 {
		t.Errorf("clear all left clients=%d owners=%d addrs=%d",
			len(service.clients), len(service.clientOwner), len(service.clientAddr))
	}
}
