package regression

import (
	"errors"
	"net/http"
	"strings"

	proto "go.keploy.io/server/grpc/regression"
	"go.keploy.io/server/pkg/models"
)

// TestCaseReq
type TestCaseReq struct {
	Captured     int64               `json:"captured" bson:"captured"`
	AppID        string              `json:"app_id" bson:"app_id"`
	URI          string              `json:"uri" bson:"uri"`
	HttpReq      models.HttpReq      `json:"http_req" bson:"http_req"`
	HttpResp     models.HttpResp     `json:"http_resp" bson:"http_resp"`
	Deps         []models.Dependency `json:"deps" bson:"deps"`
	TestCasePath string              `json:"test_case_path" bson:"test_case_path"`
	MockPath     string              `json:"mock_path" bson:"mock_path"`
	Mocks        []*proto.Mock       `json:"mocks" bson:"mocks"`
	Type         string              `json:"type" bson:"type"`
}

// GrpcTestCaseReq
type GrpcTestCaseReq struct {
	Captured    int64               `json:"captured" bson:"captured"`
	AppID       string              `json:"app_id" bson:"app_id"`
	GrpcRequest string              `json:"grpc_request" bson:"grpc_request"`
	Method      string              `json:"method" bson:"method"`
	Response    string              `json:"response" bson:"response"`
	Deps        []models.Dependency `json:"deps" bson:"deps"`
	Type        string              `json:"type" bson:"type"`
}

func (req *TestCaseReq) Bind(r *http.Request) error {
	if req.Captured == 0 {
		return errors.New("captured timestamp cant be empty")
	}

	if req.AppID == "" {
		return errors.New("app id needs to be declared")
	}

	if strings.Contains(req.TestCasePath, "../") || strings.Contains(req.MockPath, "../") || strings.HasPrefix(req.TestCasePath, "/etc/passwd") || strings.HasPrefix(req.MockPath, "/etc/passwd") {
		return errors.New("file path should be absolute")
	}
	return nil
}

func (req *GrpcTestCaseReq) Bind(r *http.Request) error {
	if req.Captured == 0 {
		return errors.New("captured timestamp cant be empty")
	}

	if req.AppID == "" {
		return errors.New("app id needs to be declared")
	}

	return nil
}

type TestReq struct {
	ID           string          `json:"id" bson:"_id"`
	AppID        string          `json:"app_id" bson:"app_id"`
	RunID        string          `json:"run_id" bson:"run_id"`
	Resp         models.HttpResp `json:"resp" bson:"resp"`
	TestCasePath string          `json:"test_case_path" bson:"test_case_path"`
	MockPath     string          `json:"mock_path" bson:"mock_path"`
}

type GrpcTestReq struct {
	ID           string `json:"id" bson:"_id"`
	AppID        string `json:"app_id" bson:"app_id"`
	RunID        string `json:"run_id" bson:"run_id"`
	Resp         string `json:"resp" bson:"resp"`
	TestCasePath string `json:"test_case_path" bson:"test_case_path"`
	MockPath     string `json:"mock_path" bson:"mock_path"`
}

func (req *TestReq) Bind(r *http.Request) error {
	if req.ID == "" {
		return errors.New("id is required")
	}

	if req.AppID == "" {
		return errors.New("app id is required")
	}

	if strings.Contains(req.TestCasePath, "../") || strings.Contains(req.MockPath, "../") || strings.HasPrefix(req.TestCasePath, "/etc/passwd") || strings.HasPrefix(req.MockPath, "/etc/passwd") {
		return errors.New("file path should be absolute")
	}
	return nil
}

func (req *GrpcTestReq) Bind(r *http.Request) error {
	if req.ID == "" {
		return errors.New("id is required")
	}

	if req.AppID == "" {
		return errors.New("app id is required")
	}

	if strings.Contains(req.TestCasePath, "../") || strings.Contains(req.MockPath, "../") || strings.HasPrefix(req.TestCasePath, "/etc/passwd") || strings.HasPrefix(req.MockPath, "/etc/passwd") {
		return errors.New("file path should be absolute")
	}
	return nil
}
