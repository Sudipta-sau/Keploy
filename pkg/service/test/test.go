package test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"go.keploy.io/server/pkg"
	"go.keploy.io/server/pkg/hooks"
	"go.keploy.io/server/pkg/models"
	"go.keploy.io/server/pkg/models/spec"
	"go.keploy.io/server/pkg/persistence"
	"go.keploy.io/server/pkg/platform/yaml"
	"go.keploy.io/server/pkg/proxy"
)

type tester struct {
	fileSystem persistence.FileSystem
	logger     *zap.Logger
}

func NewTester(fileSystem persistence.FileSystem, logger *zap.Logger) Tester {
	return &tester{
		fileSystem: fileSystem,
		logger:     logger,
	}
}

func (t *tester) Test(tcsPath, mockPath, testReportPath string, pid uint32) bool {
	models.SetMode(models.MODE_TEST)

	testReportFS := yaml.NewTestReportFS(t.fileSystem, t.logger)
	// fetch the recorded testcases with their mocks
	ys := yaml.NewYamlStore(tcsPath, mockPath, t.fileSystem, t.logger)
	// start the proxies
	ps := proxy.BootProxies(t.logger, proxy.Option{})
	// Initiate the hooks and update the vaccant ProxyPorts map
	loadedHooks := hooks.NewHook(ps.PortList, ys, t.logger)
	if err := loadedHooks.LoadHooks(pid); err != nil {
		return false
	}
	// proxy update its state in the ProxyPorts map
	ps.SetHook(loadedHooks)

	tcs, mocks, err := ys.Read(nil)
	if err != nil {
		return false
	}

	// testReport stores the result of all testruns
	testReport := &models.TestReport{
		Version: models.V1Beta1,
		// Name:    runId,
		Total:  len(tcs),
		Status: string(models.TestRunStatusRunning),
	}

	// starts the testrun
	err = testReportFS.Write(context.Background(), testReportPath, testReport)
	if err != nil {
		t.logger.Error(err.Error())
		return false
	}
	var (
		success = 0
		failure = 0
		status  = models.TestRunStatusPassed
	)

	passed := true

	// sort the testcases in
	sort.Slice(tcs, func(i, j int) bool {
		if tcs[i].Kind == models.HTTP && tcs[j].Kind == models.HTTP {
			iHttpSpec := &spec.HttpSpec{}
			tcs[i].Spec.Decode(iHttpSpec)

			jHttpSpec := &spec.HttpSpec{}
			tcs[j].Spec.Decode(jHttpSpec)
			return iHttpSpec.Created < jHttpSpec.Created
		}
		return true
	})
	for _, tc := range tcs {
		switch tc.Kind {
		case models.HTTP:
			httpSpec := &spec.HttpSpec{}
			err := tc.Spec.Decode(httpSpec)
			if err != nil {
				t.logger.Error("failed to unmarshal yaml doc for simulation of http request", zap.Error(err))
				return false
			}
			started := time.Now().UTC()
			for i, _ := range mocks[tc.Name] {
				loadedHooks.AppendDeps(&mocks[tc.Name][i])
			}
			// loadedHooks.SetDeps(mocks[tc.Name])
			// time.Sleep(1 * time.Second)
			resp, err := pkg.SimulateHttp(tc, httpSpec, t.logger, loadedHooks.GetResp)
			if err != nil {
				t.logger.Info("result", zap.Any("testcase id", tc.Name), zap.Any("passed", "false"))
				continue
			}
			// println("before blocking simulate")

			// resp := loadedHooks.GetResp()
			// println("after blocking simulate")
			testPass, testResult := t.testHttp(tc, httpSpec, resp)
			passed = passed && testPass
			t.logger.Info("result", zap.Any("testcase id", tc.Name), zap.Any("passed", testPass))
			testStatus := models.TestStatusPending
			if testPass {
				testStatus = models.TestStatusPassed
				success++
			} else {
				testStatus = models.TestStatusFailed
				failure++
				status = models.TestRunStatusFailed
			}

			testReportFS.Lock()
			testReportFS.SetResult(testReport.Name, models.TestResult{
				Kind:       models.HTTP,
				Name:       testReport.Name,
				Status:     testStatus,
				Started:    started.Unix(),
				Completed:  time.Now().UTC().Unix(),
				TestCaseID: tc.Name,
				Req: spec.HttpReqYaml{
					Method:     httpSpec.Request.Method,
					ProtoMajor: httpSpec.Request.ProtoMajor,
					ProtoMinor: httpSpec.Request.ProtoMinor,
					URL:        httpSpec.Request.URL,
					URLParams:  httpSpec.Request.URLParams,
					Header:     httpSpec.Request.Header,
					Body:       httpSpec.Request.Body,
				},
				Res: spec.HttpRespYaml{
					StatusCode:    httpSpec.Response.StatusCode,
					Header:        httpSpec.Response.Header,
					Body:          httpSpec.Response.Body,
					StatusMessage: httpSpec.Response.StatusMessage,
					ProtoMajor:    httpSpec.Response.ProtoMajor,
					ProtoMinor:    httpSpec.Response.ProtoMinor,
				},
				// Mocks:        httpSpec.Mocks,
				TestCasePath: tcsPath,
				MockPath:     mockPath,
				Noise:        httpSpec.Assertions["noise"],
				Result:       *testResult,
			})
			testReportFS.Lock()
			testReportFS.Unlock()
			// 		spec := &spec.HttpSpec{}
			// 		err := tc.Spec.Decode(spec)
			// 		if err!=nil {
			// 			t.logger.Error("failed to unmarshal yaml doc for simulation of http request", zap.Error(err))
			// 			return false
			// 		}
			// 		req, err := http.NewRequest(string(spec.Request.Method), "http://localhost"+":"+k.cfg.App.Port+spec.Request.URL, bytes.NewBufferString(spec.Request.Body))
			// 		if err != nil {
			// 			panic(err)
			// 		}
			// 		req.Header = tc.HttpReq.Header
			// 		req.Header.Set("KEPLOY_TEST_ID", tc.ID)
			// 		req.ProtoMajor = tc.HttpReq.ProtoMajor
			// 		req.ProtoMinor = tc.HttpReq.ProtoMinor
			// 		req.Close = true

			// 		// httpresp, err := k.client.Do(req)
			// 		k.client.Do(req)
			// 		if err != nil {
			// 			k.Log.Error("failed sending testcase request to app", zap.Error(err))
			// 			return nil, err
			// 		}
			// 		// defer httpresp.Body.Close()
			// 		println("before blocking simulate")

		}
	}

	// store the result of the testrun as test-report
	testResults, err := testReportFS.GetResults(testReport.Name)
	if err != nil {
		t.logger.Error("failed to fetch test results", zap.Error(err))
		return passed
	}
	testReport.Total = len(testResults)
	testReport.Status = string(status)
	testReport.Tests = testResults
	testReport.Success = success
	testReport.Failure = failure
	err = testReportFS.Write(context.Background(), testReportPath, testReport)
	if err != nil {
		t.logger.Error(err.Error())
		return false
	}

	t.logger.Info("test run completed", zap.Bool("passed overall", passed))

	// stop listening for the eBPF events
	loadedHooks.Stop(false)

	return true
}

func (t *tester) testHttp(tc models.Mock, httpSpec *spec.HttpSpec, actualResponse *spec.HttpRespYaml) (bool, *models.Result) {
	// httpSpec := &spec.HttpSpec{}
	bodyType := models.BodyTypePlain
	if json.Valid([]byte(actualResponse.Body)) {
		bodyType = models.BodyTypeJSON
	}
	pass := true
	hRes := &[]models.HeaderResult{}

	res := &models.Result{
		StatusCode: models.IntResult{
			Normal:   false,
			Expected: httpSpec.Response.StatusCode,
			Actual:   actualResponse.StatusCode,
		},
		BodyResult: []models.BodyResult{{
			Normal:   false,
			Type:     bodyType,
			Expected: httpSpec.Response.Body,
			Actual:   actualResponse.Body,
		}},
	}
	// err := tc.Spec.Decode(httpSpec)
	// if err!=nil {
	// 	t.logger.Error("failed to unmarshal yaml doc for simulation of http request", zap.Error(err))
	// 	return false, res
	// }
	// find noisy fields
	m, err := FlattenHttpResponse(pkg.ToHttpHeader(httpSpec.Response.Header), httpSpec.Response.Body)
	if err != nil {
		msg := "error in flattening http response"
		t.logger.Error(msg, zap.Error(err))
		return false, res
	}
	noise := httpSpec.Assertions["noise"]
	noise = append(noise, FindNoisyFields(m, func(k string, vals []string) bool {
		// check if k is date
		for _, v := range vals {
			if pkg.IsTime(v) {
				return true
			}
		}

		// maybe we need to concatenate the values
		return pkg.IsTime(strings.Join(vals, ", "))
	})...)

	var (
		bodyNoise   []string
		headerNoise = map[string]string{}
	)

	for _, n := range noise {
		a := strings.Split(n, ".")
		if len(a) > 1 && a[0] == "body" {
			x := strings.Join(a[1:], ".")
			bodyNoise = append(bodyNoise, x)
		} else if a[0] == "header" {
			// if len(a) == 2 {
			//  headerNoise[a[1]] = a[1]
			//  continue
			// }
			headerNoise[a[len(a)-1]] = a[len(a)-1]
			// headerNoise[a[0]] = a[0]
		}
	}

	// stores the json body after removing the noise
	// cleanExp, cleanAct := "", ""

	if !Contains(noise, "body") && bodyType == models.BodyTypeJSON {
		_, _, pass, err = Match(httpSpec.Response.Body, actualResponse.Body, bodyNoise, t.logger)
		if err != nil {
			return false, res
		}
	} else {
		if !Contains(noise, "body") && httpSpec.Response.Body != actualResponse.Body {
			pass = false
		}
	}

	res.BodyResult[0].Normal = pass

	if !CompareHeaders(pkg.ToHttpHeader(httpSpec.Response.Header), pkg.ToHttpHeader(actualResponse.Header), hRes, headerNoise) {

		pass = false
	}

	res.HeadersResult = *hRes
	if httpSpec.Response.StatusCode == actualResponse.StatusCode {
		res.StatusCode.Normal = true
	} else {

		pass = false
	}

	// t.logger.Info("", zap.Any("result of test", res))

	return pass, res
}
