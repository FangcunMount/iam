package sms

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v4/client"
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
		TemplateCode: "SMS_001",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "access_key_id and access_key_secret")

	sender, err := NewAliyunSender(AliyunConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		SignName:        "IAM",
		TemplateCode:    "SMS_001",
		Endpoint:        "dysmsapi.aliyuncs.com",
	})
	require.NoError(t, err)
	require.NotNil(t, sender)
}

func TestAliyunSenderSendsExpectedRequest(t *testing.T) {
	t.Parallel()

	client := &aliyunSMSClientStub{
		resp: &dysmsapi.SendSmsResponse{
			Body: &dysmsapi.SendSmsResponseBody{
				Code:      tea.String("OK"),
				BizId:     tea.String("biz-1"),
				RequestId: tea.String("req-1"),
			},
		},
	}
	sender := &AliyunSender{
		client:         client,
		signName:       "IAM",
		templateCode:   "SMS_001",
		codeParamName:  "otp",
		connectTimeout: 5000,
		readTimeout:    1234,
	}

	err := sender.SendLoginOTP(context.Background(), "+8613800138000", "123456")

	require.NoError(t, err)
	require.Equal(t, "+8613800138000", tea.StringValue(client.req.PhoneNumbers))
	require.Equal(t, "IAM", tea.StringValue(client.req.SignName))
	require.Equal(t, "SMS_001", tea.StringValue(client.req.TemplateCode))
	require.Equal(t, 5000, tea.IntValue(client.runtime.ConnectTimeout))
	require.Equal(t, 1234, tea.IntValue(client.runtime.ReadTimeout))
	var params map[string]string
	require.NoError(t, json.Unmarshal([]byte(tea.StringValue(client.req.TemplateParam)), &params))
	require.Equal(t, map[string]string{"otp": "123456"}, params)
}

func TestAliyunSenderReturnsErrorForNonOKResponse(t *testing.T) {
	t.Parallel()

	sender := &AliyunSender{
		client: &aliyunSMSClientStub{
			resp: &dysmsapi.SendSmsResponse{
				Body: &dysmsapi.SendSmsResponseBody{
					Code:      tea.String("isv.BUSINESS_LIMIT_CONTROL"),
					Message:   tea.String("too many requests"),
					RequestId: tea.String("req-2"),
				},
			},
		},
		signName:       "IAM",
		templateCode:   "SMS_001",
		codeParamName:  "code",
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
		templateCode:   "SMS_001",
		codeParamName:  "code",
		connectTimeout: 5000,
		readTimeout:    10000,
	}

	err := sender.SendLoginOTP(context.Background(), "+8613800138000", "123456")

	require.Error(t, err)
	require.Contains(t, err.Error(), "network down")
}

type aliyunSMSClientStub struct {
	req     *dysmsapi.SendSmsRequest
	runtime *util.RuntimeOptions
	resp    *dysmsapi.SendSmsResponse
	err     error
}

func (s *aliyunSMSClientStub) SendSmsWithOptions(req *dysmsapi.SendSmsRequest, runtime *util.RuntimeOptions) (*dysmsapi.SendSmsResponse, error) {
	s.req = req
	s.runtime = runtime
	return s.resp, s.err
}
