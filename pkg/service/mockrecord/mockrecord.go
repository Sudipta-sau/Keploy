package mockrecord

import (
	"sync"

	"go.keploy.io/server/pkg"
	"go.keploy.io/server/pkg/hooks"
	"go.keploy.io/server/pkg/models"
	"go.keploy.io/server/pkg/platform/yaml"
	"go.keploy.io/server/pkg/proxy"
	"go.keploy.io/server/pkg/platform/telemetry"
	"go.keploy.io/server/pkg/platform/fs"
	"go.uber.org/zap"
)

var Emoji = "\U0001F430" + " Keploy:"

type mockRecorder struct {
	logger *zap.Logger
	mutex  sync.Mutex
}

func NewMockRecorder(logger *zap.Logger) MockRecorder {
	return &mockRecorder{
		logger: logger,
		mutex:  sync.Mutex{},
	}
}

func (s *mockRecorder) MockRecord(path string, pid uint32, mockName string) {

	models.SetMode(models.MODE_RECORD)
	ys := yaml.NewYamlStore(path, path, "", mockName, s.logger)

	//Initiate the telemetry.
	store := fs.NewTeleFS()
	tele := telemetry.NewTelemetry(true, false, store, s.logger, "", nil)

	routineId := pkg.GenerateRandomID()

	testsTotal := 0
	// Initiate the hooks
	loadedHooks := hooks.NewHook(ys, routineId, s.logger)
	if err := loadedHooks.LoadHooks("", "", pid, &testsTotal); err != nil {
		return
	}

	mocksTotal := make(map[string]int)
	// start the proxy
	ps := proxy.BootProxy(s.logger, proxy.Option{}, "", "", pid, "", []uint{}, loadedHooks, mocksTotal)

	// proxy update its state in the ProxyPorts map
	// ps.SetHook(loadedHooks)

	// Sending Proxy Ip & Port to the ebpf program
	if err := loadedHooks.SendProxyInfo(ps.IP4, ps.Port, ps.IP6); err != nil {
		return
	}

	// Shutdown other resources
	loadedHooks.Stop(false)
	//Call the telemetry events.
	tele.RecordedMock(mocksTotal)
	tele.RecordedTest(path, testsTotal)
	ps.StopProxyServer()
}
