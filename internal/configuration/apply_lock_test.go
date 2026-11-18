package configuration

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The cross-replica apply lock is tested against real PostgreSQL and MySQL servers, two service
// instances on one database, as two manager replicas are. The cases skip unless the server is named
// in the environment, the same variables the provider migration job sets.

type lockServer struct {
	name string
	// open is a new handle on the test database with its own connection pool, as one replica has.
	open func(t *testing.T) *gorm.DB
	// endSessions ends every session on the test database but the caller's own, as a restarted
	// server or a cut connection does.
	endSessions func(t *testing.T)
}

func lockServers(t *testing.T) []lockServer {
	t.Helper()
	var servers []lockServer
	if s := postgresLockServer(t); s != nil {
		servers = append(servers, *s)
	}
	if s := mysqlLockServer(t); s != nil {
		servers = append(servers, *s)
	}
	if len(servers) == 0 {
		t.Skip("the apply lock needs POSTGRES_TEST_HOST or MYSQL_TEST_HOST")
	}
	return servers
}

func lockEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newLockDatabaseName() string {
	return fmt.Sprintf("sm_apply_lock_%d", time.Now().UnixNano())
}

func openQuiet(t *testing.T, dialector gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Idle connections stay in the pool (the default keeps two and closes the rest), so a lock that
	// leaked onto a returned connection would still be held and a case that frees it by accident
	// would not pass for the wrong reason.
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func postgresLockServer(t *testing.T) *lockServer {
	host := os.Getenv("POSTGRES_TEST_HOST")
	if host == "" {
		return nil
	}
	port := lockEnvOrDefault("POSTGRES_TEST_PORT", "5432")
	user := lockEnvOrDefault("POSTGRES_TEST_USER", "postgres")
	password := lockEnvOrDefault("POSTGRES_TEST_PASSWORD", "postgres")
	dsn := func(database string) string {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, database)
	}
	database := newLockDatabaseName()
	admin := openQuiet(t, postgres.Open(dsn("postgres")))
	require.NoError(t, admin.Exec("CREATE DATABASE "+database).Error)
	t.Cleanup(func() { _ = admin.Exec("DROP DATABASE IF EXISTS " + database + " WITH (FORCE)").Error })
	return &lockServer{
		name: "postgres",
		open: func(t *testing.T) *gorm.DB { return openQuiet(t, postgres.Open(dsn(database))) },
		endSessions: func(t *testing.T) {
			require.NoError(t, admin.Exec(
				"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = ? AND pid <> pg_backend_pid()", database).Error)
		},
	}
}

func mysqlLockServer(t *testing.T) *lockServer {
	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		return nil
	}
	port := lockEnvOrDefault("MYSQL_TEST_PORT", "3306")
	user := lockEnvOrDefault("MYSQL_TEST_USER", "root")
	password := lockEnvOrDefault("MYSQL_TEST_PASSWORD", "root")
	dsn := func(database string) string {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, database)
	}
	database := newLockDatabaseName()
	admin := openQuiet(t, mysql.Open(dsn("")))
	require.NoError(t, admin.Exec("CREATE DATABASE "+database).Error)
	t.Cleanup(func() { _ = admin.Exec("DROP DATABASE IF EXISTS " + database).Error })
	return &lockServer{
		name: "mysql",
		open: func(t *testing.T) *gorm.DB { return openQuiet(t, mysql.Open(dsn(database))) },
		endSessions: func(t *testing.T) {
			var ids []int64
			require.NoError(t, admin.Raw("SELECT id FROM information_schema.processlist WHERE db = ?", database).Scan(&ids).Error)
			for _, id := range ids {
				// A session that ended meanwhile is what the test wants anyway.
				_ = admin.Exec(fmt.Sprintf("KILL %d", id)).Error
			}
		},
	}
}

func desiredNameJSON(t *testing.T, name string) string {
	t.Helper()
	raw, err := json.Marshal(&DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr(name)}},
	})
	require.NoError(t, err)
	return string(raw)
}

// lockFixture is the test database with device 1 stored, and one service per replica.
type lockFixture struct {
	server lockServer
	setup  *gorm.DB
}

func newLockFixture(t *testing.T, server lockServer) *lockFixture {
	t.Helper()
	setup := server.open(t)
	require.NoError(t, setup.AutoMigrate(&DbDevice{}))
	require.NoError(t, setup.Create(&DbDevice{ID: 1, MAC: testDeviceMAC, DesiredConfig: desiredNameJSON(t, "old")}).Error)
	return &lockFixture{server: server, setup: setup}
}

func (f *lockFixture) replica(t *testing.T) (*ConfigurationService, *GormConfigRepository) {
	t.Helper()
	repo := NewGormConfigRepository(f.server.open(t), nil)
	return NewConfigurationService(repo, nil, nil), repo
}

func (f *lockFixture) setDesired(t *testing.T, desired string) {
	t.Helper()
	require.NoError(t, f.setup.Model(&DbDevice{}).Where("id = ?", 1).
		Updates(map[string]any{"desired_config": desired, "config_applied": false}).Error)
}

func (f *lockFixture) stored(t *testing.T) DbDevice {
	t.Helper()
	// A fresh handle: a case that ends sessions on the server leaves dead connections in the others' pools.
	var d DbDevice
	require.NoError(t, f.server.open(t).First(&d, 1).Error)
	return d
}

func shortLockWait(t *testing.T, wait time.Duration) {
	t.Helper()
	oldWait, oldPoll := applyLockWait, applyLockPoll
	applyLockWait, applyLockPoll = wait, 20*time.Millisecond
	t.Cleanup(func() { applyLockWait, applyLockPoll = oldWait, oldPoll })
}

// parkedDevice is one fake device both replicas write to. The first write parks until release is
// closed, which holds the apply that made it inside its span. names is the name after each write.
type parkedDevice struct {
	*fakeGen1Device
	mu      sync.Mutex
	names   []string
	parked  chan struct{}
	release chan struct{}
	first   atomic.Bool
}

func newParkedDevice() *parkedDevice {
	p := &parkedDevice{fakeGen1Device: newFakeGen1Device(), parked: make(chan struct{}), release: make(chan struct{})}
	p.onWrite = func() {
		p.mu.Lock()
		p.names = append(p.names, fmt.Sprint(p.settings["name"]))
		p.mu.Unlock()
	}
	return p
}

// SetConfig parks the first writer before its write reaches the device, as a replica that stalls
// after its snapshot does; a second writer goes through meanwhile.
func (p *parkedDevice) SetConfig(ctx context.Context, cfg map[string]interface{}) error {
	if p.first.CompareAndSwap(false, true) {
		close(p.parked)
		<-p.release
	}
	return p.fakeGen1Device.SetConfig(ctx, cfg)
}

func (p *parkedDevice) writtenNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.names...)
}

func (p *parkedDevice) currentName() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprint(p.settings["name"])
}

type applyOutcome struct {
	result *ApplyVerifyResult
	err    error
}

func applyAsync(svc *ConfigurationService, dev ShellyClient) <-chan applyOutcome {
	out := make(chan applyOutcome, 1)
	go func() {
		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
		out <- applyOutcome{result, err}
	}()
	return out
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func waitOutcome(t *testing.T, ch <-chan applyOutcome, what string) applyOutcome {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return applyOutcome{}
	}
}

// The failure of ginsys/shelly-manager#425: replica A takes its snapshot and stops inside the write,
// the desired configuration changes, replica B applies the new one, and then A writes its old
// snapshot to the device. With the lock, B waits for A, so the device ends with the new
// configuration and the flag is true over it.
func TestApplyLockProviders_TheSecondReplicaWaitsAndTheDeviceEndsWithTheNewConfiguration(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, 20*time.Second)
			f := newLockFixture(t, server)
			svcA, _ := f.replica(t)
			svcB, _ := f.replica(t)
			dev := newParkedDevice()

			aDone := applyAsync(svcA, dev)
			waitClosed(t, dev.parked, "replica A to park inside its write")
			f.setDesired(t, desiredNameJSON(t, "new"))
			bDone := applyAsync(svcB, dev)

			var b applyOutcome
			bEarly := false
			select {
			case b = <-bDone: // without the lock B runs now, ahead of A
				bEarly = true
			case <-time.After(700 * time.Millisecond):
			}
			close(dev.release)
			a := waitOutcome(t, aDone, "replica A")
			require.NoError(t, a.err)
			if !bEarly {
				b = waitOutcome(t, bDone, "replica B")
			}
			require.NoError(t, b.err)

			names := dev.writtenNames()
			require.NotEmpty(t, names)
			assert.Equal(t, "old", names[0], "replica A wrote first")
			assert.Equal(t, "new", names[len(names)-1], "the last write to the device is the new configuration: %v", names)
			assert.Equal(t, "new", dev.currentName())
			assert.True(t, f.stored(t).ConfigApplied, "the flag is true, and the device holds what the flag says")
		})
	}
}

// A second replica that cannot get the lock within the wait is refused, and sends nothing.
func TestApplyLockProviders_TheSecondReplicaIsRefusedAfterTheWait(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, 300*time.Millisecond)
			f := newLockFixture(t, server)
			svcA, _ := f.replica(t)
			svcB, _ := f.replica(t)
			devA := newParkedDevice()
			devB := newFakeGen1Device()

			aDone := applyAsync(svcA, devA)
			waitClosed(t, devA.parked, "replica A to park inside its write")
			before := f.stored(t)

			b := waitOutcome(t, applyAsync(svcB, devB), "replica B")

			require.ErrorIs(t, b.err, ErrApplyInProgress)
			assert.Zero(t, devB.setCalls, "the refused apply wrote nothing to the device")
			assert.Zero(t, devB.reboots)
			after := f.stored(t)
			assert.Equal(t, before.ConfigApplied, after.ConfigApplied, "the refusal changed no flag")
			assert.Equal(t, before.PendingReboot, after.PendingReboot)

			close(devA.release)
			require.NoError(t, waitOutcome(t, aDone, "replica A").err)
		})
	}
}

// The pending-reboot flag has the same race (the second case of ginsys/shelly-manager#425): while
// replica A is between writing a reboot-needing value and clearing the flag after the reboot, replica
// B must not run, or its record of a newer owed reboot is erased by A's clear.
func TestApplyLockProviders_TheSecondReplicaCannotTouchThePendingRebootFlagInsideAnApply(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, 300*time.Millisecond)
			f := newLockFixture(t, server)
			raw, err := json.Marshal(mqttAndSNTP())
			require.NoError(t, err)
			f.setDesired(t, string(raw))
			svcA, _ := f.replica(t)
			svcB, _ := f.replica(t)

			rebootParked, rebootRelease := make(chan struct{}), make(chan struct{})
			var parkedOnce atomic.Bool
			devA := &rebootingDevice{fakeGen1Device: newDeviceWithMQTT(), onReboot: func() {
				if parkedOnce.CompareAndSwap(false, true) {
					close(rebootParked)
					<-rebootRelease
				}
			}}
			devB := newDeviceWithMQTT()

			aDone := applyAsync(svcA, devA)
			waitClosed(t, rebootParked, "replica A to park inside its reboot")
			assert.True(t, f.stored(t).PendingReboot, "A recorded the owed reboot before writing")

			b := waitOutcome(t, applyAsync(svcB, devB), "replica B")

			require.ErrorIs(t, b.err, ErrApplyInProgress)
			assert.Zero(t, devB.setCalls, "B wrote nothing")
			assert.Zero(t, devB.reboots, "B rebooted nothing")
			assert.True(t, f.stored(t).PendingReboot, "the flag is still A's")

			close(rebootRelease)
			require.NoError(t, waitOutcome(t, aDone, "replica A").err)
			assert.False(t, f.stored(t).PendingReboot, "cleared once, by the apply that rebooted")
			assert.Equal(t, 1, devA.reboots+devB.reboots, "one reboot in all")
		})
	}
}

// rebootingDevice runs a hook when the apply reboots it.
type rebootingDevice struct {
	*fakeGen1Device
	onReboot func()
}

func (d *rebootingDevice) Reboot(ctx context.Context) error {
	if d.onReboot != nil {
		d.onReboot()
	}
	return d.fakeGen1Device.Reboot(ctx)
}

// A lock lost during the apply (the server ended the session) lets another replica in. The device
// writes cannot be undone, but the apply must not claim success: the flag stays false and the
// error says the lock was lost.
func TestApplyLockProviders_ALockLostDuringTheApplyIsNotReportedAsSuccess(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			f := newLockFixture(t, server)
			svcA, _ := f.replica(t)
			dev := newParkedDevice()

			aDone := applyAsync(svcA, dev)
			waitClosed(t, dev.parked, "replica A to park inside its write")
			server.endSessions(t)
			close(dev.release)

			a := waitOutcome(t, aDone, "replica A")

			require.ErrorIs(t, a.err, ErrApplyLockLost)
			assert.False(t, f.stored(t).ConfigApplied, "success was not recorded")
		})
	}
}

// Taking the lock, running ordinary queries through the pool, and releasing it leaves nothing held:
// the unlock ran on the connection that took the lock, not on another one from the pool.
func TestApplyLockProviders_TheLockIsReleasedOnTheConnectionThatTookIt(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, time.Second)
			f := newLockFixture(t, server)
			_, repoA := f.replica(t)
			_, repoB := f.replica(t)

			lock, err := repoA.LockDeviceApply(context.Background(), 1)
			require.NoError(t, err)
			require.NotNil(t, lock)
			var wg sync.WaitGroup
			for i := 0; i < 6; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, queryErr := repoA.GetDevice(1)
					assert.NoError(t, queryErr)
				}()
			}
			wg.Wait()
			lock.Unlock()

			started := time.Now()
			second, err := repoB.LockDeviceApply(context.Background(), 1)
			require.NoError(t, err, "the lock was still held after Unlock")
			require.NotNil(t, second)
			second.Unlock()
			assert.Less(t, time.Since(started), 500*time.Millisecond, "the second replica got it at once")
		})
	}
}

// A request that was cancelled still releases the lock it held.
func TestApplyLockProviders_ACancelledRequestStillReleasesTheLock(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, time.Second)
			f := newLockFixture(t, server)
			_, repoA := f.replica(t)
			_, repoB := f.replica(t)

			ctx, cancel := context.WithCancel(context.Background())
			lock, err := repoA.LockDeviceApply(ctx, 1)
			require.NoError(t, err)
			require.NotNil(t, lock)
			cancel()
			lock.Unlock()

			second, err := repoB.LockDeviceApply(context.Background(), 1)
			require.NoError(t, err)
			require.NotNil(t, second)
			second.Unlock()
		})
	}
}

// Release on a clean disconnect: a replica that closes the connection holding the lock frees it at
// once. An abrupt loss of the replica (a pod that vanishes with no TCP reset) is not tested: the
// server then frees the lock when it notices the dead client, which the docs name as a limit.
func TestApplyLockProviders_ACleanDisconnectReleasesTheLock(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, time.Second)
			f := newLockFixture(t, server)
			_, repoA := f.replica(t)
			_, repoB := f.replica(t)

			lock, err := repoA.LockDeviceApply(context.Background(), 1)
			require.NoError(t, err)
			require.NotNil(t, lock)
			// Close the physical connection, as a replica that exits does; Raw with ErrBadConn
			// makes database/sql close it instead of pooling it.
			held, ok := lock.(*sessionLock)
			require.True(t, ok)
			_ = held.conn.Raw(func(any) error { return driver.ErrBadConn })
			_ = held.conn.Close()

			second, err := repoB.LockDeviceApply(context.Background(), 1)
			require.NoError(t, err, "the lock outlived its connection")
			require.NotNil(t, second)
			second.Unlock()
		})
	}
}

// Another device's lock is independent.
func TestApplyLockProviders_TheLockIsPerDevice(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, 200*time.Millisecond)
			f := newLockFixture(t, server)
			_, repoA := f.replica(t)
			_, repoB := f.replica(t)

			one, err := repoA.LockDeviceApply(context.Background(), 1)
			require.NoError(t, err)
			require.NotNil(t, one)
			defer one.Unlock()

			other, err := repoB.LockDeviceApply(context.Background(), 2)
			require.NoError(t, err)
			require.NotNil(t, other)
			other.Unlock()

			_, err = repoB.LockDeviceApply(context.Background(), 1)
			require.ErrorIs(t, err, ErrApplyInProgress)
		})
	}
}

// SQLite runs in one process: the repository offers a lock that needs no database session.
func TestApplyLock_SQLiteNeedsNoDatabaseLock(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	repo := NewGormConfigRepository(db, nil)

	lock, err := repo.LockDeviceApply(context.Background(), 1)

	require.NoError(t, err)
	assert.Nil(t, lock)
}

// The server builds its configuration service from NewService. If that wiring wrapped the
// repository so the lock was no longer offered, the type assertion would fail without a sound and a
// PostgreSQL deployment would serialize applies inside one process only.
func TestApplyLock_TheServersOwnWiringOffersTheLock(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svc := NewService(db, nil)

	_, offered := svc.ConfigurationSvc.repo.(applyLocker)
	assert.True(t, offered, "the repository the server uses must offer the cross-replica apply lock")
}

// A repository with no lock leaves the apply serialized inside the process, as before.
func TestApplyLock_ARepositoryWithoutALockStillSerializesInProcess(t *testing.T) {
	svc, _ := newServiceWithDesiredName(t, "tv-office")

	lock, err := svc.lockApplyAcrossReplicas(context.Background(), 1)

	require.NoError(t, err)
	assert.NoError(t, lock.StillHeld(context.Background()))
	lock.Unlock()
}

// A device id the two-int4 PostgreSQL key cannot hold is refused, never truncated onto another device.
func TestApplyLockProviders_ADeviceIDBeyondInt4IsRefused(t *testing.T) {
	for _, server := range lockServers(t) {
		if server.name != "postgres" {
			continue
		}
		t.Run(server.name, func(t *testing.T) {
			f := newLockFixture(t, server)
			_, repo := f.replica(t)

			_, err := repo.LockDeviceApply(context.Background(), 1<<31)

			require.Error(t, err)
			assert.False(t, strings.Contains(err.Error(), "in progress"))
		})
	}
}
