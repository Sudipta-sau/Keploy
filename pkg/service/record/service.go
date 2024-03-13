package record

import (
	"context"

	"go.keploy.io/server/v2/pkg/models"
)

type Instrumentation interface {
	//Setup prepares the environment for the recording
	Setup(ctx context.Context, cmd string, opts models.SetupOptions) (uint64, error)
	//Hook will load hooks and start the proxy server.
	Hook(ctx context.Context, id uint64, opts models.HookOptions) error
	GetIncoming(ctx context.Context, id uint64, opts models.IncomingOptions) (<-chan *models.TestCase, error)
	GetOutgoing(ctx context.Context, id uint64, opts models.OutgoingOptions) (<-chan *models.Mock, error)
	// Run is blocking call and will execute until error
	Run(ctx context.Context, id uint64, opts models.RunOptions) models.AppError
}

type Service interface {
	Start(ctx context.Context) error
	StartMock(ctx context.Context) error
}

type TestDB interface {
	GetAllTestSetIDs(ctx context.Context) ([]string, error)
	InsertTestCase(ctx context.Context, tc *models.TestCase, testSetID string) error
}

type MockDB interface {
	InsertMock(ctx context.Context, mock *models.Mock, testSetID string) error
}

type Telemetry interface {
	RecordedTestSuite(ctx context.Context, testSet string, testsTotal int, mockTotal map[string]int)
	RecordedTestCaseMock(ctx context.Context, mockType string)
	RecordedMocks(ctx context.Context, mockTotal map[string]int)
	RecordedTestAndMocks(ctx context.Context)
}
