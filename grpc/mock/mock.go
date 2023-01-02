package mock

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	proto "go.keploy.io/server/grpc/regression"
	"go.keploy.io/server/grpc/utils"
	"go.keploy.io/server/pkg/models"
)

func Encode(doc *proto.Mock) (models.Mock, error) {
	res := models.Mock{
		Version: models.Version(doc.Version),
		Kind:    models.Kind(doc.Kind),
		Name:    doc.Name,
	}
	switch doc.Kind {
	case string(models.HTTP):
		spec := models.HttpSpec{
			Metadata: doc.Spec.Metadata,
			Request: models.MockHttpReq{
				Method:     models.Method(doc.Spec.Req.Method),
				ProtoMajor: int(doc.Spec.Req.ProtoMajor),
				ProtoMinor: int(doc.Spec.Req.ProtoMinor),
				URL:        doc.Spec.Req.URL,
				Header:     ToMockHeader(utils.GetHttpHeader(doc.Spec.Req.Header)),
				Body:       doc.Spec.Req.Body,
			},
			Response: models.MockHttpResp{
				StatusCode:    int(doc.Spec.Res.StatusCode),
				Header:        ToMockHeader(utils.GetHttpHeader(doc.Spec.Res.Header)),
				Body:          doc.Spec.Res.Body,
				StatusMessage: doc.Spec.Res.StatusMessage,
				ProtoMajor:    int(doc.Spec.Res.ProtoMajor),
				ProtoMinor:    int(doc.Spec.Res.ProtoMinor),
			},
			Objects:    ToModelObjects(doc.Spec.Objects),
			Mocks:      doc.Spec.Mocks,
			Assertions: utils.GetHttpHeader(doc.Spec.Assertions),
			Created:    doc.Spec.Created,
		}

		err := res.Spec.Encode(&spec)
		if err != nil {
			return res, fmt.Errorf("failed to encode http spec for mock with name: %s.  error: %s", doc.Name, err.Error())
		}

	case string(models.SQL):
		spec := models.SQlSpec{
			Type:     models.SqlOutputType(doc.Spec.Type),
			Metadata: doc.Spec.Metadata,
			Int:      int(doc.Spec.Int),
			Err:      doc.Spec.Err,
		}
		if doc.Spec.Table != nil {
			spec.Table = models.Table{
				Cols: ToModelCols(doc.Spec.Table.Cols),
				Rows: doc.Spec.Table.Rows,
			}
		}
		err := res.Spec.Encode(&spec)
		if err != nil {
			return res, fmt.Errorf("failed to encode sql spec for mock with name: %s.  error: %s", doc.Name, err.Error())
		}

	case string(models.GENERIC):
		err := res.Spec.Encode(&models.GenericSpec{
			Metadata: doc.Spec.Metadata,
			Objects:  ToModelObjects(doc.Spec.Objects),
		})
		if err != nil {
			return res, fmt.Errorf("failed to encode generic spec for mock with name: %s.  error: %s", doc.Name, err.Error())
		}
	default:
		return res, fmt.Errorf("mock with name %s is not of a valid kind", doc.Name)
	}
	return res, nil
}

func ToModelCols(cols []*proto.SqlCol) []models.SqlCol {
	res := []models.SqlCol{}
	for _, j := range cols {
		res = append(res, models.SqlCol{
			Name:      j.Name,
			Type:      j.Type,
			Precision: int(j.Precision),
			Scale:     int(j.Scale),
		})
	}
	return res
}

func toProtoCols(cols []models.SqlCol) ([]*proto.SqlCol, error) {
	if len(cols) == 0 {
		return nil, nil
	}
	res := []*proto.SqlCol{}
	for _, j := range cols {

		res = append(res, &proto.SqlCol{
			Name:      j.Name,
			Type:      j.Type,
			Precision: int64(j.Precision),
			Scale:     int64(j.Scale),
		})
	}
	return res, nil
}
func ToModelObjects(objs []*proto.Mock_Object) []models.Object {
	res := []models.Object{}
	for _, j := range objs {
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		if _, err := gz.Write(j.Data); err != nil {
			return nil
		}
		gz.Close()
		data := base64.StdEncoding.EncodeToString(b.Bytes())
		res = append(res, models.Object{
			Type: j.Type,
			Data: data,
		})
	}
	return res
}

func toProtoObjects(objs []models.Object) ([]*proto.Mock_Object, error) {
	res := []*proto.Mock_Object{}
	for _, j := range objs {
		data := []byte{}
		bin, err := base64.StdEncoding.DecodeString(j.Data)
		if err != nil {
			return nil, err
		}
		r := bytes.NewReader(bin)
		if r.Len() > 0 {
			gzr, err := gzip.NewReader(r)
			if err != nil {
				return nil, err
			}
			data, err = ioutil.ReadAll(gzr)
			if err != nil {
				return nil, err
			}
		}
		res = append(res, &proto.Mock_Object{
			Type: j.Type,
			Data: data,
		})
	}
	return res, nil
}

func Decode(doc []models.Mock) ([]*proto.Mock, error) {
	res := []*proto.Mock{}
	for _, j := range doc {
		mock := &proto.Mock{
			Version: string(j.Version),
			Name:    j.Name,
			Kind:    string(j.Kind),
		}
		switch j.Kind {
		case models.HTTP:
			spec := &models.HttpSpec{}
			err := j.Spec.Decode(spec)
			if err != nil {
				return res, fmt.Errorf("failed to decode the http spec of mock with name: %s.  error: %s", j.Name, err.Error())
			}
			obj, err := toProtoObjects(spec.Objects)
			if err != nil {
				return res, err
			}
			mock.Spec = &proto.Mock_SpecSchema{
				Metadata: spec.Metadata,
				Req: &proto.HttpReq{
					Method:     string(spec.Request.Method),
					ProtoMajor: int64(spec.Request.ProtoMajor),
					ProtoMinor: int64(spec.Request.ProtoMinor),
					URL:        spec.Request.URL,
					Header:     utils.GetProtoMap(ToHttpHeader(spec.Request.Header)),
					Body:       spec.Request.Body,
				},
				Objects: obj,
				Res: &proto.HttpResp{
					StatusCode:    int64(spec.Response.StatusCode),
					Header:        utils.GetProtoMap(ToHttpHeader(spec.Response.Header)),
					Body:          spec.Response.Body,
					StatusMessage: spec.Response.StatusMessage,
					ProtoMajor:    int64(spec.Response.ProtoMajor),
					ProtoMinor:    int64(spec.Request.ProtoMinor),
				},
				Mocks:      spec.Mocks,
				Assertions: utils.GetProtoMap(spec.Assertions),
				Created:    spec.Created,
			}
		case models.SQL:
			spec := &models.SQlSpec{}
			err := j.Spec.Decode(spec)
			if err != nil {
				return res, fmt.Errorf("failed to decode the sql spec of mock with name: %s.  error: %s", j.Name, err.Error())
			}
			cols, err := toProtoCols(spec.Table.Cols)
			if err != nil {
				return res, err
			}
			mock.Spec = &proto.Mock_SpecSchema{
				Type:     string(spec.Type),
				Metadata: spec.Metadata,
				Int:      int64(spec.Int),
				Err:      spec.Err,
			}
			if cols != nil {
				mock.Spec.Table = &proto.Table{
					Cols: cols,
					Rows: spec.Table.Rows,
				}
			}
			if spec.Err == nil {
				fmt.Println("\n\n\n nilnil", spec.Err, mock.Spec.Err)
			}

		case models.GENERIC:
			spec := &models.GenericSpec{}
			err := j.Spec.Decode(spec)
			if err != nil {
				return res, fmt.Errorf("failed to decode the generic spec of mock with name: %s.  error: %s", j.Name, err.Error())
			}
			obj, err := toProtoObjects(spec.Objects)
			if err != nil {
				return res, err
			}
			mock.Spec = &proto.Mock_SpecSchema{
				Metadata: spec.Metadata,
				Objects:  obj,
			}
		default:
			return res, fmt.Errorf("mock with name %s is not of a valid kind", j.Name)
		}
		res = append(res, mock)
	}
	return res, nil
}

func ToHttpHeader(mockHeader map[string]string) http.Header {
	header := http.Header{}
	for i, j := range mockHeader {
		header[i] = strings.Split(j, ",")
	}
	return header
}

func ToMockHeader(httpHeader http.Header) map[string]string {
	header := map[string]string{}
	for i, j := range httpHeader {
		header[i] = strings.Join(j, ",")
	}
	return header
}
