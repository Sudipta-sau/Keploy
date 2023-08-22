package httpparser

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.keploy.io/server/pkg"
	"go.keploy.io/server/pkg/hooks"
	"go.keploy.io/server/pkg/models"
	"go.keploy.io/server/pkg/proxy/util"
	"go.uber.org/zap"
)

var Emoji = "\U0001F430" + " Keploy:"

// IsOutgoingHTTP function determines if the outgoing network call is HTTP by comparing the
// message format with that of an HTTP text message.
func IsOutgoingHTTP(buffer []byte) bool {
	return bytes.HasPrefix(buffer[:], []byte("HTTP/")) ||
		bytes.HasPrefix(buffer[:], []byte("GET ")) ||
		bytes.HasPrefix(buffer[:], []byte("POST ")) ||
		bytes.HasPrefix(buffer[:], []byte("PUT ")) ||
		bytes.HasPrefix(buffer[:], []byte("PATCH ")) ||
		bytes.HasPrefix(buffer[:], []byte("DELETE ")) ||
		bytes.HasPrefix(buffer[:], []byte("OPTIONS ")) ||
		bytes.HasPrefix(buffer[:], []byte("HEAD "))
}

func ProcessOutgoingHttp(request []byte, clientConn, destConn net.Conn, h *hooks.Hook, logger *zap.Logger) {
	switch models.GetMode() {
	case models.MODE_RECORD:
		// *deps = append(*deps, encodeOutgoingHttp(request,  clientConn,  destConn, logger))
		h.AppendMocks(encodeOutgoingHttp(request, clientConn, destConn, logger))
		// h.TestCaseDB.WriteMock(encodeOutgoingHttp(request, clientConn, destConn, logger))
	case models.MODE_TEST:
		decodeOutgoingHttp(request, clientConn, destConn, h, logger)
	default:
		logger.Info(Emoji+"Invalid mode detected while intercepting outgoing http call", zap.Any("mode", models.GetMode()))
	}

}

// Handled chunked requests when content-length is given.
func contentLengthRequest(finalReq *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, contentLength int) {
	for contentLength > 0 {
				clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		requestChunked, err := util.ReadBytes(clientConn)
		if err != nil {
			if err == io.EOF {
				logger.Error(Emoji+"connection closed by the user client", zap.Error(err))
				break
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				logger.Info(Emoji+"Stopped getting data from the connection", zap.Error(err))
				break
			} else {
				logger.Error(Emoji+"failed to read the response message from the destination server", zap.Error(err))
				return
			}
		}
		*finalReq = append(*finalReq, requestChunked...)
		contentLength -= len(requestChunked)
		_, err = destConn.Write(requestChunked)
		if err != nil {
			logger.Error(Emoji+"failed to write request message to the destination server", zap.Error(err))
			return
		}
	}
}

// Handled chunked requests when transfer-encoding is given.
func chunkedRequest(finalReq *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, transferEncodingHeader string) {
	if transferEncodingHeader == "chunked" {
		for {
						clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			requestChunked, err := util.ReadBytes(clientConn)
			if err != nil {
				if err == io.EOF {
					logger.Error(Emoji+"connection closed by the user client", zap.Error(err))
					break
				} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					break
				} else {
					logger.Error(Emoji+"failed to read the response message from the destination server", zap.Error(err))
					return
				}
			}
			*finalReq = append(*finalReq, requestChunked...)
			_, err = destConn.Write(requestChunked)
			if err != nil {
				logger.Error(Emoji+"failed to write request message to the destination server", zap.Error(err))
				return
			}
			if string(requestChunked) == "0\r\n\r\n" {
				break
			}
		}
	}
}

// Handled chunked responses when content-length is given.
func contentLengthResponse(finalResp *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, contentLength int) {
	for contentLength > 0 {
		//Set deadline of 5 seconds
		destConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		resp, err := util.ReadBytes(destConn)
		if err != nil {
			//Check if the connection closed.
			if err == io.EOF {
				logger.Error(Emoji+"connection closed by the destination server", zap.Error(err))
				break
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				logger.Info(Emoji+"Stopped getting data from the connection", zap.Error(err))
				break
			} else {
				logger.Error(Emoji+"failed to read the response message from the destination server", zap.Error(err))
				return
			}
		}
		*finalResp = append(*finalResp, resp...)
		contentLength -= len(resp)
		// write the response message to the user client
		_, err = clientConn.Write(resp)
		if err != nil {
			logger.Error(Emoji+"failed to write response message to the user client", zap.Error(err))
			return
		}
	}
}

// Handled chunked responses when transfer-encoding is given.
func chunkedResponse(finalResp *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, transferEncodingHeader string) {
	//If the transfer-encoding header is chunked
	if transferEncodingHeader == "chunked" {
		for {
						//Set deadline of 5 seconds
			destConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			resp, err := util.ReadBytes(destConn)
			if err != nil {
				//Check if the connection closed.
				if err == io.EOF {
					logger.Error(Emoji+"connection closed by the destination server", zap.Error(err))
					break
				} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					//Check if the deadline is reached.
					logger.Info(Emoji + "Stopped getting buffer from the destination server")
					break
				} else {
					logger.Error(Emoji+"failed to read the response message from the destination server", zap.Error(err))
					return
				}
			}
			*finalResp = append(*finalResp, resp...)
			// write the response message to the user client
			_, err = clientConn.Write(resp)
			if err != nil {
				logger.Error(Emoji+"failed to write response message to the user client", zap.Error(err))
				return
			}
			if string(resp) == "0\r\n\r\n" {
				break
			}
		}
	}
}

func handleChunkedRequests(finalReq *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, request []byte) {
	lines := strings.Split(string(request), "\n")
	var contentLengthHeader string
	var transferEncodingHeader string
	for _, line := range lines {
		if strings.HasPrefix(line, "Content-Length:") {
			contentLengthHeader = strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			break
		} else if strings.HasPrefix(line, "Transfer-Encoding:") {
			transferEncodingHeader = strings.TrimSpace(strings.TrimPrefix(line, "Transfer-Encoding:"))
			break
		}
	}
	//Handle chunked requests
	if contentLengthHeader != "" {
		contentLength, err := strconv.Atoi(contentLengthHeader)
		if err != nil {
			logger.Error(Emoji+"failed to get the content-length header", zap.Error(err))
			return
		}
		//Get the length of the body in the request.
		bodyLength := len(request) - strings.Index(string(request), "\r\n\r\n") - 4
		contentLength -= bodyLength
		if contentLength > 0 {
			contentLengthRequest(finalReq, clientConn, destConn, logger, contentLength)
		}
	} else if transferEncodingHeader != "" {
		chunkedRequest(finalReq, clientConn, destConn, logger, transferEncodingHeader)
	}
}

func handleChunkedResponses(finalResp *[]byte, clientConn, destConn net.Conn, logger *zap.Logger, resp []byte) {
		//Getting the content-length or the transfer-encoding header
		var contentLengthHeader, transferEncodingHeader string
		lines := strings.Split(string(resp), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Content-Length:") {
				contentLengthHeader = strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				break
			} else if strings.HasPrefix(line, "Transfer-Encoding:") {
				transferEncodingHeader = strings.TrimSpace(strings.TrimPrefix(line, "Transfer-Encoding:"))
				break
			}
		}
		//Handle chunked responses
		if contentLengthHeader != "" {
			contentLength, err := strconv.Atoi(contentLengthHeader)
			if err != nil {
				logger.Error(Emoji+"failed to get the content-length header", zap.Error(err))
				return
			}
			bodyLength := len(resp) - strings.Index(string(resp), "\r\n\r\n") - 4
			contentLength -= bodyLength
			if contentLength > 0 {
				contentLengthResponse(finalResp, clientConn, destConn, logger, contentLength)
			}
		} else if transferEncodingHeader != "" {
			chunkedResponse(finalResp, clientConn, destConn, logger, transferEncodingHeader)
		}
}

//Checks if the response is gzipped
func checkIfGzipped(check io.ReadCloser) (bool, *bufio.Reader) {
	bufReader := bufio.NewReader(check)
	peekedBytes, err := bufReader.Peek(2)
	if err != nil && err != io.EOF {
		fmt.Println("Error peeking:", err)
		return false, nil
	}
	if len(peekedBytes) < 2 {
		return false, nil
	}
	if peekedBytes[0] == 0x1f && peekedBytes[1] == 0x8b {
		return true, bufReader
	} else {
		return false, nil
	}
}

//Decodes the mocks in test mode so that they can be sent to the user application.
func decodeOutgoingHttp(request []byte, clienConn, destConn net.Conn, h *hooks.Hook, logger *zap.Logger) {
	// if len(deps) == 0 {

	if h.GetDepsSize() == 0 {
		// logger.Error("failed to mock the output for unrecorded outgoing http call")
		return
	}

	// var httpSpec spec.HttpSpec
	// err := deps[0].Spec.Decode(&httpSpec)
	// if err != nil {
	// 	logger.Error("failed to decode the yaml spec for the outgoing http call")
	// 	return
	// }
	// httpSpec := deps[0]
	stub := h.FetchDep(0)
	// fmt.Println("http mock in test: ", stub)

	statusLine := fmt.Sprintf("HTTP/%d.%d %d %s\r\n", stub.Spec.HttpReq.ProtoMajor, stub.Spec.HttpReq.ProtoMinor, stub.Spec.HttpResp.StatusCode, http.StatusText(int(stub.Spec.HttpResp.StatusCode)))

	// Fetching the response headers
	header := pkg.ToHttpHeader(stub.Spec.HttpResp.Header)
	var headers string
	for key, values := range header {
		for _, value := range values {
			headerLine := fmt.Sprintf("%s: %s\r\n", key, value)
			headers += headerLine
		}
	}
	body := stub.Spec.HttpResp.Body
	var responseString string

	//Check if the gzip encoding is present in the header
	if header["Content-Encoding"] != nil && header["Content-Encoding"][0] == "gzip" {
		var compressedBuffer bytes.Buffer
		gw := gzip.NewWriter(&compressedBuffer)
		_, err := gw.Write([]byte(body))
		if err != nil {
			logger.Error(Emoji+"failed to compress the response body", zap.Error(err))
			return
		}
		err = gw.Close()
		if err != nil {
			logger.Error(Emoji+"failed to close the gzip writer", zap.Error(err))
			return
		}
		responseString = statusLine + headers + "\r\n" + compressedBuffer.String()
	}else{
		responseString = statusLine + headers + "\r\n" + body
	}
	_, err := clienConn.Write([]byte(responseString))
	if err != nil {
		logger.Error(Emoji+"failed to write the mock output to the user application", zap.Error(err))
		return
	}
	// pop the mocked output from the dependency queue
	// deps = deps[1:]
	h.PopFront()
}

// encodeOutgoingHttp function parses the HTTP request and response text messages to capture outgoing network calls as mocks.
func encodeOutgoingHttp(request []byte, clientConn, destConn net.Conn, logger *zap.Logger) *models.Mock {
	defer destConn.Close()
	var resp []byte
	var finalResp []byte
	var finalReq []byte
	var err error
	// write the request message to the actual destination server
	_, err = destConn.Write(request)
	if err != nil {
		logger.Error(Emoji+"failed to write request message to the destination server", zap.Error(err))
		return nil
	}
	finalReq = append(finalReq, request...)

	//Handle chunked requests
	handleChunkedRequests(&finalReq, clientConn, destConn, logger, request)

	// read the response from the actual server
	resp, err = util.ReadBytes(destConn)
	if err != nil {
		logger.Error(Emoji+"failed to read the response message from the destination server", zap.Error(err))
		return nil
	}
	// write the response message to the user client
	_, err = clientConn.Write(resp)
	if err != nil {
		logger.Error(Emoji+"failed to write response message to the user client", zap.Error(err))
		return nil
	}
	finalResp = append(finalResp, resp...)
	handleChunkedResponses(&finalResp, clientConn, destConn, logger, resp)
	var req *http.Request
	// converts the request message buffer to http request
	req, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(finalReq)))
	if err != nil {
		logger.Error(Emoji+"failed to parse the http request message", zap.Error(err))
		return nil
	}
	var reqBody []byte
	if req.Body != nil { // Read
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			// TODO right way to log errors
			logger.Error(Emoji+"failed to read the http request body", zap.Error(err))
			return nil
		}
	}
	// converts the response message buffer to http response
	respParsed, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(finalResp)), req)
	if err != nil {
		logger.Error(Emoji+"failed to parse the http response message", zap.Error(err))
		return nil
	}
	var respBody []byte
	if respParsed.Body != nil { // Read
		if respParsed.Header.Get("Content-Encoding") == "gzip" {
			check := respParsed.Body
			ok, reader := checkIfGzipped(check)
			fmt.Println("ok: ", ok)
			if ok {
				gzipReader, err := gzip.NewReader(reader)
				if err != nil {
					logger.Error(Emoji+"failed to create a gzip reader", zap.Error(err))
					return nil
				}
				respParsed.Body = gzipReader
			}
		}
		respBody, err = io.ReadAll(respParsed.Body)
		if err != nil {
			logger.Error(Emoji+"failed to read the the http repsonse body", zap.Error(err))
			return nil
		}
	}
	// store the request and responses as mocks
	meta := map[string]string{
		"name":      "Http",
		"type":      models.HttpClient,
		"operation": req.Method,
	}
	// httpMock := &models.Mock{
	// 	Version: models.V1Beta2,
	// 	Name:    "",
	// 	Kind:    models.HTTP,
	// }

	// // encode the message into yaml
	// err = httpMock.Spec.Encode(&spec.HttpSpec{
	// 		Metadata: meta,
	// 		Request: spec.HttpReqYaml{
	// 			Method:     spec.Method(req.Method),
	// 			ProtoMajor: req.ProtoMajor,
	// 			ProtoMinor: req.ProtoMinor,
	// 			URL:        req.URL.String(),
	// 			Header:     pkg.ToYamlHttpHeader(req.Header),
	// 			Body:       string(reqBody),
	// 			URLParams: pkg.UrlParams(req),
	// 		},
	// 		Response: spec.HttpRespYaml{
	// 			StatusCode: resp.StatusCode,
	// 			Header:     pkg.ToYamlHttpHeader(resp.Header),
	// 			Body: string(respBody),
	// 		},
	// })
	// if err != nil {
	// 	logger.Error("failed to encode the http messsage into the yaml")
	// 	return nil
	// }

	// return httpMock

	return &models.Mock{
		Version: models.V1Beta2,
		Name:    "mocks",
		Kind:    models.HTTP,
		Spec: models.MockSpec{
			Metadata: meta,
			HttpReq: &models.HttpReq{
				Method:     models.Method(req.Method),
				ProtoMajor: req.ProtoMajor,
				ProtoMinor: req.ProtoMinor,
				URL:        req.URL.String(),
				Header:     pkg.ToYamlHttpHeader(req.Header),
				Body:       string(reqBody),
				URLParams:  pkg.UrlParams(req),
			},
			HttpResp: &models.HttpResp{
				StatusCode: respParsed.StatusCode,
				Header:     pkg.ToYamlHttpHeader(respParsed.Header),
				Body:       string(respBody),
			},
			Created: time.Now().Unix(),
		},
	}

	// if val, ok := Deps[string(port)]; ok {
	// keploy.Deps = append(keploy.Deps, httpMock)
}
