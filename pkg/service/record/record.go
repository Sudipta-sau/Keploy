package record

import (
	"context"
	"errors"
	"fmt"

	"go.keploy.io/server/v2/config"
	"go.keploy.io/server/v2/pkg"
	"go.keploy.io/server/v2/pkg/models"
	"go.keploy.io/server/v2/utils"
	"go.uber.org/zap"
)

type recorder struct {
	logger          *zap.Logger
	testDB          TestDB
	mockDB          MockDB
	telemetry       Telemetry
	instrumentation Instrumentation
	config          config.Config
}

func NewRecorder(logger *zap.Logger, testDB TestDB, mockDB MockDB, telemetry Telemetry, instrumentation Instrumentation, config config.Config) Service {
	return &recorder{
		logger:          logger,
		testDB:          testDB,
		mockDB:          mockDB,
		telemetry:       telemetry,
		instrumentation: instrumentation,
		config:          config,
	}
}

func (r *recorder) Start(ctx context.Context) error {
	var runAppError models.AppError
	var appErrChan = make(chan models.AppError)
	var incomingChan <-chan *models.TestCase
	var outgoingChan <-chan *models.Mock
	var incomingErrChan <-chan error
	var outgoingErrChan <-chan error
	var recordErr error
	var appId uint64

	stopReason := "User stopped recording"

	testSetIds, err := r.testDB.GetAllTestSetIds(ctx)
	if err != nil {
		stopReason = "failed to get testSetIds"
		utils.Stop(r.logger, stopReason)
		return fmt.Errorf(stopReason+": %w", err)
	}

	newTestSetId := pkg.NewId(testSetIds, models.TestSetPattern)

	appId, err = r.instrumentation.Setup(ctx, r.config.Command, models.SetupOptions{})
	if err != nil {
		return fmt.Errorf("failed to execute record due to error while setting up the environment: %w", err)
	}

	err = r.instrumentation.Hook(ctx, appId, models.HookOptions{})
	if err != nil {
		return fmt.Errorf("failed to start the hooks and proxy: %w", err)
	}

	incomingChan, incomingErrChan = r.instrumentation.GetIncoming(ctx, appId, models.IncomingOptions{})

	outgoingChan, outgoingErrChan = r.instrumentation.GetOutgoing(ctx, appId, models.OutgoingOptions{})

	go func() {
		runAppError = r.instrumentation.Run(ctx, appId, models.RunOptions{})
		appErrChan <- runAppError
	}()

	loop := true
	for loop {
		select {
		case appErr := <-appErrChan:
			switch appErr.AppErrorType {
			case models.ErrCommandError:
				stopReason = "error in running the user application, hence stopping keploy"
				r.logger.Error(stopReason, zap.Error(appErr))
			case models.ErrUnExpected:
				stopReason = "user application terminated unexpectedly hence stopping keploy"
				r.logger.Warn(stopReason+", please check user application logs if this behaviour is not expected", zap.Error(appErr))
			default:
				stopReason = "unknown error recieved from application, hence stopping keploy"
				r.logger.Error("unknown error recieved from user application, hence stopping keploy", zap.Error(appErr))
			}
			recordErr = errors.New("failed to execute record due to error in running the user application")
			loop = false
		case testCase := <-incomingChan:
			err := r.testDB.InsertTestCase(context.Background(), testCase, newTestSetId)
			if err != nil {
				stopReason = "error while inserting incoming frame into db, hence stopping keploy"
				r.logger.Error(stopReason, zap.Error(err))
				recordErr = errors.New("failed to execute record due to error in inserting incoming frame into db")
				loop = false
			}
		case mock := <-outgoingChan:
			err := r.mockDB.InsertMock(context.Background(), mock, newTestSetId)
			if err != nil {
				stopReason = "error while inserting outgoing frame into db, hence stopping keploy"
				r.logger.Error(stopReason, zap.Error(err))
				recordErr = errors.New("failed to execute record due to error in inserting outgoing frame into db")
				loop = false
			}
		case err := <-incomingErrChan:
			stopReason = "error while fetching incoming frame, hence stopping keploy"
			r.logger.Error(stopReason, zap.Error(err))
			recordErr = errors.New("failed to execute record due to error in fetching incoming frame")
			loop = false
		case err := <-outgoingErrChan:
			stopReason = "error while fetching outgoing frame, hence stopping keploy"
			r.logger.Error(stopReason, zap.Error(err))
			recordErr = errors.New("failed to execute record due to error in fetching outgoing frame")
			loop = false
		case <-ctx.Done():
			return nil
		}
	}
	utils.Stop(r.logger, stopReason)
	return recordErr
}

func (r *recorder) StartMock(ctx context.Context) error {
	var outgoingChan <-chan *models.Mock
	var outgoingErrChan <-chan error
	var stopReason string
	var recordErr error

	appId, err := r.instrumentation.Setup(ctx, r.config.Command, models.SetupOptions{})
	err = r.instrumentation.Hook(ctx, appId, models.HookOptions{})
	if err != nil {
		return fmt.Errorf("failed to execute mock-record due to error while loading hooks and proxy: %w", err)
	}

	go func() {
		outgoingChan, outgoingErrChan = r.instrumentation.GetOutgoing(ctx, appId, models.OutgoingOptions{})
	}()

	loop := true
	for loop {
		select {
		case mock := <-outgoingChan:
			err := r.mockDB.InsertMock(context.Background(), mock, "")
			if err != nil {
				stopReason = "error while inserting outgoing frame into db, hence stopping keploy"
				r.logger.Error(stopReason, zap.Error(err))
				recordErr = errors.New("failed to execute record due to error in inserting outgoing frame into db")
				loop = false
			}
		case err := <-outgoingErrChan:
			stopReason = "error while fetching outgoing frame, hence stopping keploy"
			r.logger.Error(stopReason, zap.Error(err))
			recordErr = errors.New("failed to execute record due to error in fetching outgoing frame")
			loop = false
		case <-ctx.Done():
			return nil
		}
	}
	utils.Stop(r.logger, stopReason)
	return recordErr
}
