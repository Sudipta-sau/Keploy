package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/agnivade/levenshtein"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations/util"
	"go.keploy.io/server/v2/pkg/models"
	"go.keploy.io/server/v2/utils"
	"go.uber.org/zap"
)

func match(ctx context.Context, logger *zap.Logger, matchParams *matchParams, mockDb integrations.MockMemDb) (bool, *models.Mock, error) {
	for {
		select {
		case <-ctx.Done():
			return false, nil, ctx.Err()
		default:
			tcsMocks, err := mockDb.GetFilteredMocks()

			if err != nil {
				utils.LogError(logger, err, "failed to get tcs mocks")
				return false, nil, errors.New("error while matching the request with the mocks")
			}
			var eligibleMocks []*models.Mock

			for _, mock := range tcsMocks {
				if ctx.Err() != nil {
					return false, nil, ctx.Err()
				}
				if mock.Kind == models.HTTP {
					isMockBodyJSON := isJSON([]byte(mock.Spec.HTTPReq.Body))

					//the body of mock and request aren't of same type
					if isMockBodyJSON != matchParams.reqBodyIsJSON {
						logger.Debug("The body of mock and request aren't of same type")
						continue
					}

					//parse request body url
					parsedURL, err := url.Parse(mock.Spec.HTTPReq.URL)
					if err != nil {
						utils.LogError(logger, err, "failed to parse mock url")
						continue
					}

					//Check if the path matches
					if parsedURL.Path != matchParams.req.URL.Path {
						//If it is not the same, continue
						logger.Debug("The url path of mock and request aren't the same")
						continue
					}

					//Check if the method matches
					if mock.Spec.HTTPReq.Method != models.Method(matchParams.req.Method) {
						//If it is not the same, continue
						logger.Debug("The method of mock and request aren't the same")
						continue
					}

					// Check if the header keys match
					if !mapsHaveSameKeys(mock.Spec.HTTPReq.Header, matchParams.req.Header) {
						// Different headers, so not a match
						logger.Debug("The header keys of mock and request aren't the same")
						continue
					}

					if !mapsHaveSameKeys(mock.Spec.HTTPReq.URLParams, matchParams.req.URL.Query()) {
						// Different query params, so not a match
						logger.Debug("The query params of mock and request aren't the same")
						continue
					}
					eligibleMocks = append(eligibleMocks, mock)
				}
			}

			if len(eligibleMocks) == 0 {
				return false, nil, nil
			}

			// If the body is JSON we do a schema match.
			if matchParams.reqBodyIsJSON {
				logger.Debug("Performing schema match")
				for _, mock := range eligibleMocks {
					if ctx.Err() != nil {
						return false, nil, ctx.Err()
					}
					var mockData map[string]interface{}
					var reqData map[string]interface{}
					err := json.Unmarshal([]byte(mock.Spec.HTTPReq.Body), &mockData)
					if err != nil {
						utils.LogError(logger, err, "Failed to unmarshal the mock request body")
						break
					}
					err = json.Unmarshal(matchParams.reqBuf, &reqData)
					if err != nil {
						utils.LogError(logger, err, "failed to unmarshal the request body")
						break
					}

					if schemaMatch(mockData, reqData) {
						isDeleted := mockDb.DeleteFilteredMock(mock)
						if isDeleted {
							return true, mock, nil
						}
						logger.Debug("found match but did not delete it")
					}
				}
			}
			logger.Debug("Performing fuzzy match")
			isMatched, bestMatch := fuzzyMatch(eligibleMocks, matchParams.reqBuf)
			if isMatched {
				isDeleted := mockDb.DeleteFilteredMock(bestMatch)
				if !isDeleted {
					logger.Debug("found match but did not delete it, so ignoring")
					continue
				}
			}
			return isMatched, bestMatch, nil
		}
	}

}

func schemaMatch(mockData map[string]interface{}, reqData map[string]interface{}) bool {
	for key := range mockData {
		_, exists := reqData[key]
		if !exists {
			return false
		}
	}
	return true
}

func mapsHaveSameKeys(map1 map[string]string, map2 map[string][]string) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key := range map1 {
		if _, exists := map2[key]; !exists {
			return false
		}
	}

	for key := range map2 {
		if _, exists := map1[key]; !exists {
			return false
		}
	}

	return true
}

func findStringMatch(_ string, mockString []string) int {
	minDist := int(^uint(0) >> 1) // Initialize with max int value
	bestMatch := -1
	for idx, req := range mockString {
		if !util.IsASCIIPrintable(mockString[idx]) {
			continue
		}

		dist := levenshtein.ComputeDistance(req, mockString[idx])
		if dist == 0 {
			return 0
		}

		if dist < minDist {
			minDist = dist
			bestMatch = idx
		}
	}
	return bestMatch
}

// TODO: generalize the function to work with any type of integration
func findBinaryMatch(mocks []*models.Mock, reqBuff []byte) int {

	mxSim := -1.0
	mxIdx := -1
	// find the fuzzy hash of the mocks
	for idx, mock := range mocks {
		encoded, _ := decode(mock.Spec.HTTPReq.Body)
		k := util.AdaptiveK(len(reqBuff), 3, 8, 5)
		shingles1 := util.CreateShingles(encoded, k)
		shingles2 := util.CreateShingles(reqBuff, k)
		similarity := util.JaccardSimilarity(shingles1, shingles2)

		// log.Debugf("Jaccard Similarity:%f\n", similarity)

		if mxSim < similarity {
			mxSim = similarity
			mxIdx = idx
		}
	}
	return mxIdx
}

func encode(buffer []byte) string {
	//Encode the buffer to string
	encoded := string(buffer)
	return encoded
}
func decode(encoded string) ([]byte, error) {
	// decode the string to a buffer.
	data := []byte(encoded)
	return data, nil
}

func fuzzyMatch(tcsMocks []*models.Mock, reqBuff []byte) (bool, *models.Mock) {
	com := encode(reqBuff)
	for _, mock := range tcsMocks {
		encoded, _ := decode(mock.Spec.HTTPReq.Body)
		if string(encoded) == string(reqBuff) || mock.Spec.HTTPReq.Body == com {
			return true, mock
		}
	}
	// convert all the configmocks to string array
	mockString := make([]string, len(tcsMocks))
	for i := 0; i < len(tcsMocks); i++ {
		mockString[i] = tcsMocks[i].Spec.HTTPReq.Body
	}
	// find the closest match
	if util.IsASCIIPrintable(string(reqBuff)) {
		idx := findStringMatch(string(reqBuff), mockString)
		if idx != -1 {
			return true, tcsMocks[idx]
		}
	}
	idx := findBinaryMatch(tcsMocks, reqBuff)
	if idx != -1 {
		return true, tcsMocks[idx]
	}
	return false, &models.Mock{}
}
