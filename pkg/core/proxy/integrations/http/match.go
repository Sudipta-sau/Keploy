package http

import (
	"context"
	"errors"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations/util"
	"net/http"
	"net/url"
	"unicode"

	"github.com/agnivade/levenshtein"
	"github.com/cloudflare/cfssl/log"
	"go.keploy.io/server/v2/pkg/models"
	"go.uber.org/zap"
)

func match(ctx context.Context, logger *zap.Logger, req *http.Request, reqURL *url.URL, requestBuffer []byte, jsonBody bool, mockDb integrations.MockMemDb) (bool, *models.Mock, error) {
	for {
		select {
		case <-ctx.Done():
			return false, nil, nil
		default:
			tcsMocks, err := mockDb.GetFilteredMocks()

			if err != nil {
				logger.Error("failed to get tcs mocks", zap.Error(err))
				return false, nil, errors.New("error while matching the request with the mocks")
			}
			var eligibleMocks []*models.Mock

			for _, mock := range tcsMocks {
				if mock.Kind == models.HTTP {
					isMockBodyJSON := isJSON([]byte(mock.Spec.HttpReq.Body))

					//the body of mock and request aren't of same type
					if isMockBodyJSON != jsonBody {
						continue
					}

					//parse request body url
					parsedURL, err := url.Parse(mock.Spec.HttpReq.URL)
					if err != nil {
						logger.Error("failed to parse mock url", zap.Error(err))
						continue
					}

					//Check if the path matches
					if parsedURL.Path != reqURL.Path {
						//If it is not the same, continue
						continue
					}

					//Check if the method matches
					if mock.Spec.HttpReq.Method != models.Method(req.Method) {
						//If it is not the same, continue
						continue
					}

					// Check if the header keys match
					if !mapsHaveSameKeys(mock.Spec.HttpReq.Header, req.Header) {
						// Different headers, so not a match
						continue
					}

					if !mapsHaveSameKeys(mock.Spec.HttpReq.URLParams, req.URL.Query()) {
						// Different query params, so not a match
						continue
					}
					eligibleMocks = append(eligibleMocks, mock)
				}
			}

			if len(eligibleMocks) == 0 {
				return false, nil, nil
			}

			isMatched, bestMatch := Fuzzymatch(eligibleMocks, requestBuffer)
			if isMatched {
				isDeleted := mockDb.DeleteFilteredMock(bestMatch)
				if !isDeleted {
					continue
				}
			}
			return isMatched, bestMatch, nil
		}
	}

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

func findStringMatch(req string, mockString []string) int {
	minDist := int(^uint(0) >> 1) // Initialize with max int value
	bestMatch := -1
	for idx, req := range mockString {
		if !IsAsciiPrintable(mockString[idx]) {
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

func HttpDecoder(encoded string) ([]byte, error) {
	// decode the string to a buffer.

	data := []byte(encoded)
	return data, nil
}

func findBinaryMatch(configMocks []*models.Mock, reqBuff []byte) int {

	mxSim := -1.0
	mxIdx := -1
	// find the fuzzy hash of the mocks
	for idx, mock := range configMocks {
		encoded, _ := HttpDecoder(mock.Spec.HttpReq.Body)
		k := util.AdaptiveK(len(reqBuff), 3, 8, 5)
		shingles1 := util.CreateShingles(encoded, k)
		shingles2 := util.CreateShingles(reqBuff, k)
		similarity := util.JaccardSimilarity(shingles1, shingles2)

		log.Debugf("Jaccard Similarity:%f\n", similarity)

		if mxSim < similarity {
			mxSim = similarity
			mxIdx = idx
		}
	}
	return mxIdx
}

func IsAsciiPrintable(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func HttpEncoder(buffer []byte) string {
	//Encode the buffer to string
	encoded := string(buffer)
	return encoded
}

func Fuzzymatch(tcsMocks []*models.Mock, reqBuff []byte) (bool, *models.Mock) {
	com := HttpEncoder(reqBuff)
	for _, mock := range tcsMocks {
		encoded, _ := HttpDecoder(mock.Spec.HttpReq.Body)
		if string(encoded) == string(reqBuff) || mock.Spec.HttpReq.Body == com {
			return true, mock
		}
	}
	// convert all the configmocks to string array
	mockString := make([]string, len(tcsMocks))
	for i := 0; i < len(tcsMocks); i++ {
		mockString[i] = string(tcsMocks[i].Spec.HttpReq.Body)
	}
	// find the closest match
	if IsAsciiPrintable(string(reqBuff)) {
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
