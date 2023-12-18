package genericparser

import (
	"encoding/base64"
	// "fmt"
	"unicode"

	"github.com/agnivade/levenshtein"
	"github.com/cloudflare/cfssl/log"
	"go.keploy.io/server/pkg/hooks"
	"go.keploy.io/server/pkg/models"
	"go.keploy.io/server/pkg/proxy/util"
)

func PostgresDecoder(encoded string) ([]byte, error) {
	// decode the base 64 encoded string to buffer ..

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// fmt.Println(hooks.Emoji+"failed to decode the data", err)
		return nil, err
	}
	// println("Decoded data is :", string(data))
	return data, nil
}

func fuzzymatch(tcsMocks []*models.Mock, requestBuffers [][]byte, h *hooks.Hook) (bool, []models.GenericPayload) {
	for idx, mock := range tcsMocks {
		if len(mock.Spec.GenericRequests) == len(requestBuffers) {
			matched := true // Flag to track if all requests match

			for requestIndex, reqBuff := range requestBuffers {
				bufStr := string(reqBuff)
				if !IsAsciiPrintable(string(reqBuff)) {
					bufStr = base64.StdEncoding.EncodeToString(reqBuff)
				}

				encoded := []byte(mock.Spec.GenericRequests[requestIndex].Message[0].Data)
				if !IsAsciiPrintable(mock.Spec.GenericRequests[requestIndex].Message[0].Data) {
					encoded, _ = PostgresDecoder(mock.Spec.GenericRequests[requestIndex].Message[0].Data)
				}

				// Compare the encoded data
				if string(encoded) != string(reqBuff) || mock.Spec.GenericRequests[requestIndex].Message[0].Data != bufStr {
					matched = false
					break // Exit the loop if any request doesn't match
				}
			}

			if matched {
				log.Debug("matched in first loop")
				tcsMocks = append(tcsMocks[:idx], tcsMocks[idx+1:]...)
				h.SetTcsMocks(tcsMocks)
				return true, mock.Spec.GenericResponses
			}
		}
	}

	idx := findBinaryMatch(tcsMocks, requestBuffers, h)
	if idx != -1 {
		log.Debug("matched in binary match")
		bestMatch := tcsMocks[idx].Spec.GenericResponses
		tcsMocks = append(tcsMocks[:idx], tcsMocks[idx+1:]...)
		h.SetTcsMocks(tcsMocks)
		return true, bestMatch
	}

	return false, nil
}

func findBinaryMatch(tcsMocks []*models.Mock, requestBuffers [][]byte, h *hooks.Hook) int {

	mxSim := -1.0
	mxIdx := -1
	for idx, mock := range tcsMocks {
		if len(mock.Spec.GenericRequests) == len(requestBuffers) {
			for requestIndex, reqBuff := range requestBuffers {

				// bufStr := string(reqBuff)
				// if !IsAsciiPrintable(bufStr) {
				_ = base64.StdEncoding.EncodeToString(reqBuff)
				// }
				encoded, _ := PostgresDecoder(mock.Spec.GenericRequests[requestIndex].Message[0].Data)

				k := util.AdaptiveK(len(reqBuff), 3, 8, 5)
				shingles1 := util.CreateShingles(encoded, k)
				shingles2 := util.CreateShingles(reqBuff, k)
				similarity := util.JaccardSimilarity(shingles1, shingles2)
				log.Debugf(hooks.Emoji, "Jaccard Similarity:%f\n", similarity)

				if mxSim < similarity {
					mxSim = similarity
					mxIdx = idx
				}
			}
		}
	}
	return mxIdx
}

// checks if s is ascii and printable, aka doesn't include tab, backspace, etc.
func IsAsciiPrintable(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || (!unicode.IsPrint(r) && r != '\r' && r != '\n') {
			return false
		}
	}
	return true
}

func findStringMatch(req []string, mockString []string) int {
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
