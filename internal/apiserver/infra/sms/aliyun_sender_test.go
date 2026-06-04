package sms

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	dypnsapi "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/require"
)

func TestNewAliyunSenderValidatesRequiredConfig(t *testing.T) {
	t.Parallel()

	_, err := NewAliyunSender(AliyunConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sign_name and template_code are required")

	_, err = NewAliyunSender(AliyunConfig{
		AccessKeyID:  "ak",
		SignName:     "IAM",
		TemplateCode: "100001",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "access_key_id and access_key_secret")

	sender, err := NewAliyunSender(AliyunConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		SignName:        "IAM",
		TemplateCode:    "100001",
		Endpoint:        "dypnsapi.aliyuncs.com",
	})
	require.NoError(t, err)
	require.NotNil(t, sender)
}

func TestAliyunSenderSendsExpectedRequest(t *testing.T) {
	t.Parallel()

	client := &aliyunSMSClientStub{
		resp: &dypnsapi.SendSmsVerifyCodeResponse{
			Body: &dypnsapi.SendSmsVerifyCodeResponseBody{
				Code:      tea.String("OK"),
				RequestId: tea.String("req-1"),
				Model: &dypnsapi.SendSmsVerifyCodeResponseBodyModel{
					BizId: tea.String("biz-1"),
				},
			},
		},
	}
	sender := &AliyunSender{
		client:         client,
		signName:       "IAM",
		templateCode:   "100001",
		codeParamName:  "code",
		minParamName:   "min",
		validMinutes:   5,
		connectTimeout: 5000,
		readTimeout:    1234,
	}

	err := sender.SendLoginOTP(context.Background(), "+8613800138000", "123456")

	require.NoError(t, err)
	require.Equal(t, "86", tea.StringValue(client.req.CountryCode))
	require.Equal(t, "13800138000", tea.StringValue(client.req.PhoneNumber))
	require.Equal(t, "IAM", tea.StringValue(client.req.SignName))
	require.Equal(t, "100001", tea.StringValue(client.req.TemplateCode))
	require.Equal(t, int64(300), tea.Int64Value(client.req.ValidTime))
	require.Equal(t, 5000, tea.IntValue(client.runtime.ConnectTimeout))
	require.Equal(t, 1234, tea.IntValue(client.runtime.ReadTimeout))
	var params map[string]string
	require.NoError(t, json.Unmarshal([]byte(tea.StringValue(client.req.TemplateParam)), &params))
	require.Equal(t, map[string]string{"code": "123456", "min": "5"}, params)
}

func TestAliyunSenderReturnsErrorForNonOKResponse(t *testing.T) {
	t.Parallel()

	sender := &AliyunSender{
		client: &aliyunSMSClientStub{
			resp: &dypnsapi.SendSmsVerifyCodeResponse{
				Body: &dypnsapi.SendSmsVerifyCodeResponseBody{
					Code:      tea.String("isv.BUSINESS_LIMIT_CONTROL"),
					Message:   tea.String("too many requests"),
					RequestId: tea.String("req-2"),
				},
			},
		},
		signName:       "IAM",
		templateCode:   "100001",
		codeParamName:  "code",
		minParamName:   "min",
		validMinutes:   5,
		connectTimeout: 5000,
		readTimeout:    10000,
	}

	err := sender.SendLoginOTP(context.Background(), "+8613800138000", "123456")

	require.Error(t, err)
	require.Contains(t, err.Error(), "isv.BUSINESS_LIMIT_CONTROL")
	require.Contains(t, err.Error(), "req-2")
}

func TestAliyunSenderWrapsClientError(t *testing.T) {
	t.Parallel()

	sender := &AliyunSender{
		client:         &aliyunSMSClientStub{err: errors.New("network down")},
		signName:       "IAM",
		templateCode:   "100001",
		codeParamName:  "code",
		minParamName:   "min",
		validMinutes:   5,
		connectTimeout: 5000,
		readTimeout:    10000,
	}

	err := sender.SendLoginOTP(context.Background(), "+8613800138000", "123456")

	require.Error(t, err)
	require.Contains(t, err.Error(), "network down")
}

type aliyunSMSClientStub struct {
	req     *dypnsapi.SendSmsVerifyCodeRequest
	runtime *util.RuntimeOptions
	resp    *dypnsapi.SendSmsVerifyCodeResponse
	err     error
}

func (s *aliyunSMSClientStub) SendSmsVerifyCodeWithOptions(req *dypnsapi.SendSmsVerifyCodeRequest, runtime *util.RuntimeOptions) (*dypnsapi.SendSmsVerifyCodeResponse, error) {
	s.req = req
	s.runtime = runtime
	return s.resp, s.err
}
