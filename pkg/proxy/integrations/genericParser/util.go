package genericparser

import (
	"encoding/base64"
	"fmt"

	// "fmt"
	"unicode"

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

func fuzzymatch(requestBuffers [][]byte, h *hooks.Hook) (bool, []models.GenericPayload, error) {
	for {
		tcsMocks, err := h.GetTcsMocks()
		if err != nil {
			return false, nil, fmt.Errorf("error while getting tcs mocks %v", err)
		}
		index := -1
		for idx, mock := range tcsMocks {
			if len(mock.Spec.GenericRequests) == len(requestBuffers) {
				matched := true // Flag to track if all requests match

				for requestIndex, reqBuff := range requestBuffers {

					bufStr := string(reqBuff)
					if !IsAsciiPrintable(string(reqBuff)) {
						bufStr = base64.StdEncoding.EncodeToString(reqBuff)
						fmt.Println("not possible")
						fmt.Println(bufStr)
					}

					// Compare the encoded data
					if mock.Spec.GenericRequests[requestIndex].Message[0].Data != bufStr {
						matched = false
						break // Exit the loop if any request doesn't match
					}
				}
				if matched {
					index = idx
					break
				}
			}
		}
		if index != -1 {
			responseMock := make([]models.GenericPayload, len(tcsMocks[index].Spec.GenericResponses))
			copy(responseMock, tcsMocks[index].Spec.GenericResponses)
			isDeleted, err := h.DeleteTcsMock(tcsMocks[index])
			if err != nil {
				return false, nil, fmt.Errorf("error while deleting tcsMock %v", err)
			}
			if !isDeleted {
				continue
			}
			return true, responseMock, nil
		}
		break
	}

	return false, nil, nil
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
