package teststore

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"go.keploy.io/server/v2/pkg"
	"go.keploy.io/server/v2/pkg/models"
	"go.keploy.io/server/v2/pkg/platform/yaml"
	"go.keploy.io/server/v2/utils"
	"go.uber.org/zap"
)

func EncodeTestcase(tc models.TestCase, logger *zap.Logger) (*yaml.NetworkTrafficDoc, error) {

	header := pkg.ToHttpHeader(tc.HttpReq.Header)
	curl := pkg.MakeCurlCommand(string(tc.HttpReq.Method), tc.HttpReq.URL, pkg.ToYamlHttpHeader(header), tc.HttpReq.Body)
	doc := &yaml.NetworkTrafficDoc{
		Version: tc.Version,
		Kind:    tc.Kind,
		Name:    tc.Name,
		Curl:    curl,
	}
	// find noisy fields
	m, err := FlattenHttpResponse(pkg.ToHttpHeader(tc.HttpResp.Header), tc.HttpResp.Body)
	if err != nil {
		msg := "error in flattening http response"
		logger.Error(msg, zap.Error(err))
	}
	noise := tc.Noise

	noiseFieldsFound := FindNoisyFields(m, func(k string, vals []string) bool {
		// check if k is date
		for _, v := range vals {
			if pkg.IsTime(v) {
				return true
			}
		}

		// maybe we need to concatenate the values
		return pkg.IsTime(strings.Join(vals, ", "))
	})

	for _, v := range noiseFieldsFound {
		noise[v] = []string{}
	}

	switch tc.Kind {
	case models.HTTP:
		err := doc.Spec.Encode(models.HttpSchema{
			Request:  tc.HttpReq,
			Response: tc.HttpResp,
			Created:  tc.Created,
			Assertions: map[string]interface{}{
				"noise": noise,
			},
		})
		if err != nil {
			logger.Error("failed to encode testcase into a yaml doc", zap.Error(err))
			return nil, err
		}
	default:
		logger.Error("failed to marshal the testcase into yaml due to invalid kind of testcase")
		return nil, errors.New("type of testcases is invalid")
	}
	return doc, nil
}

func FindNoisyFields(m map[string][]string, comparator func(string, []string) bool) []string {
	var noise []string
	for k, v := range m {
		if comparator(k, v) {
			noise = append(noise, k)
		}
	}
	return noise
}

func FlattenHttpResponse(h http.Header, body string) (map[string][]string, error) {
	m := map[string][]string{}
	for k, v := range h {
		m["header."+k] = []string{strings.Join(v, "")}
	}
	err := AddHttpBodyToMap(body, m)
	if err != nil {
		return m, err
	}
	return m, nil
}

func AddHttpBodyToMap(body string, m map[string][]string) error {
	// add body
	if json.Valid([]byte(body)) {
		var result interface{}

		err := json.Unmarshal([]byte(body), &result)
		if err != nil {
			return err
		}
		j := Flatten(result)
		for k, v := range j {
			nk := "body"
			if k != "" {
				nk = nk + "." + k
			}
			m[nk] = v
		}
	} else {
		// add it as raw text
		m["body"] = []string{body}
	}
	return nil
}

// Flatten takes a map and returns a new one where nested maps are replaced
// by dot-delimited keys.
// examples of valid jsons - https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/JSON/parse#examples
func Flatten(j interface{}) map[string][]string {
	if j == nil {
		return map[string][]string{"": {""}}
	}
	o := make(map[string][]string)
	x := reflect.ValueOf(j)
	switch x.Kind() {
	case reflect.Map:
		m, ok := j.(map[string]interface{})
		if !ok {
			return map[string][]string{}
		}
		for k, v := range m {
			nm := Flatten(v)
			for nk, nv := range nm {
				fk := k
				if nk != "" {
					fk = fk + "." + nk
				}
				o[fk] = nv
			}
		}
	case reflect.Bool:
		o[""] = []string{strconv.FormatBool(x.Bool())}
	case reflect.Float64:
		o[""] = []string{strconv.FormatFloat(x.Float(), 'E', -1, 64)}
	case reflect.String:
		o[""] = []string{x.String()}
	case reflect.Slice:
		child, ok := j.([]interface{})
		if !ok {
			return map[string][]string{}
		}
		for _, av := range child {
			nm := Flatten(av)
			for nk, nv := range nm {
				if ov, exists := o[nk]; exists {
					o[nk] = append(ov, nv...)
				} else {
					o[nk] = nv
				}
			}
		}
	default:
		fmt.Println(utils.Emoji, "found invalid value in json", j, x.Kind())
	}
	return o
}

func ContainsMatchingUrl(urlMethods []string, urlStr string, requestUrl string, requestMethod models.Method) (error, bool) {
	urlMatched := false
	parsedURL, err := url.Parse(requestUrl)
	if err != nil {
		return err, false
	}

	// Check for URL path and method
	regex, err := regexp.Compile(urlStr)
	if err != nil {
		return err, false
	}

	urlMatch := regex.MatchString(parsedURL.Path)

	if urlMatch && len(urlStr) != 0 {
		urlMatched = true
	}

	if len(urlMethods) != 0 && urlMatched {
		urlMatched = false
		for _, method := range urlMethods {
			if string(method) == string(requestMethod) {
				urlMatched = true
			}
		}
	}

	return nil, urlMatched
}

func HasBannedHeaders(object map[string]string, bannedHeaders map[string]string) (error, bool) {
	for headerName, headerNameValue := range object {
		for bannedHeaderName, bannedHeaderValue := range bannedHeaders {
			regex, err := regexp.Compile(headerName)
			if err != nil {
				return err, false
			}
			headerNameMatch := regex.MatchString(bannedHeaderName)

			regex, err = regexp.Compile(bannedHeaderValue)
			if err != nil {
				return err, false
			}
			headerValueMatch := regex.MatchString(headerNameValue)
			if headerNameMatch && headerValueMatch {
				return nil, true
			}
		}
	}
	return nil, false
}

func Decode(yamlTestcase *yaml.NetworkTrafficDoc, logger *zap.Logger) (*models.TestCase, error) {
	tc := models.TestCase{
		Version: yamlTestcase.Version,
		Kind:    yamlTestcase.Kind,
		Name:    yamlTestcase.Name,
		Curl:    yamlTestcase.Curl,
	}
	switch tc.Kind {
	case models.HTTP:
		httpSpec := models.HttpSchema{}
		err := yamlTestcase.Spec.Decode(&httpSpec)
		if err != nil {
			logger.Error("failed to unmarshal a yaml doc into the http testcase", zap.Error(err))
			return nil, err
		}
		tc.Created = httpSpec.Created
		tc.HttpReq = httpSpec.Request
		tc.HttpResp = httpSpec.Response
		tc.Noise = map[string][]string{}
		switch reflect.ValueOf(httpSpec.Assertions["noise"]).Kind() {
		case reflect.Map:
			for k, v := range httpSpec.Assertions["noise"].(map[string]interface{}) {
				tc.Noise[k] = []string{}
				for _, val := range v.([]interface{}) {
					tc.Noise[k] = append(tc.Noise[k], val.(string))
				}
			}
		case reflect.Slice:
			for _, v := range httpSpec.Assertions["noise"].([]interface{}) {
				tc.Noise[v.(string)] = []string{}
			}
		}
	// unmarshal its mocks from yaml docs to go struct
	case models.GRPC_EXPORT:
		grpcSpec := models.GrpcSpec{}
		err := yamlTestcase.Spec.Decode(&grpcSpec)
		if err != nil {
			logger.Error(utils.Emoji+"failed to unmarshal a yaml doc into the gRPC testcase", zap.Error(err))
			return nil, err
		}
		tc.GrpcReq = grpcSpec.GrpcReq
		tc.GrpcResp = grpcSpec.GrpcResp
	default:
		logger.Error("failed to unmarshal yaml doc of unknown type", zap.Any("type of yaml doc", tc.Kind))
		return nil, errors.New("yaml doc of unknown type")
	}
	return &tc, nil
}
