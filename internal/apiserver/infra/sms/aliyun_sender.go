package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/FangcunMount/component-base/pkg/logger"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dypnsapi "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	"github.com/nyaruka/phonenumbers"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
)

const (
	defaultAliyunConnectTimeoutMillis = 5000
	defaultAliyunReadTimeoutMillis    = 10000
	defaultAliyunValidMinutes         = 5
)

// AliyunSender 直连阿里云号码认证（Dypns）SendSmsVerifyCode 发送登录 OTP。
// 验证码仍由 IAM 自行生成/存储/校验，阿里云仅负责投递。
type AliyunSender struct {
	client         aliyunSMSClient
	signName       string
	templateCode   string
	codeParamName  string
	minParamName   string
	validMinutes   int
	connectTimeout int
	readTimeout    int
}

var _ authentication.SMSSender = (*AliyunSender)(nil)

type aliyunSMSClient interface {
	SendSmsVerifyCodeWithOptions(request *dypnsapi.SendSmsVerifyCodeRequest, runtime *util.RuntimeOptions) (*dypnsapi.SendSmsVerifyCodeResponse, error)
}

// AliyunConfig 构造 AliyunSender 所需的配置。
// AccessKeyID/AccessKeySecret 留空时走阿里云默认凭据链（环境变量/RAM 角色等），推荐生产使用。
type AliyunConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
	Endpoint        string
	CodeParamName   string
	MinParamName    string
	ValidMinutes    int
	TimeoutMillis   int
}

// NewAliyunSender 创建阿里云号码认证短信发送器；缺少必填项时返回错误。
func NewAliyunSender(cfg AliyunConfig) (*AliyunSender, error) {
	if strings.TrimSpace(cfg.SignName) == "" || strings.TrimSpace(cfg.TemplateCode) == "" {
		return nil, fmt.Errorf("aliyun sms: sign_name and template_code are required")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "dypnsapi.aliyuncs.com"
	}
	codeParam := strings.TrimSpace(cfg.CodeParamName)
	if codeParam == "" {
		codeParam = "code"
	}
	minParam := strings.TrimSpace(cfg.MinParamName)
	if minParam == "" {
		minParam = "min"
	}
	validMinutes := cfg.ValidMinutes
	if validMinutes <= 0 {
		validMinutes = defaultAliyunValidMinutes
	}

	openapiConfig := &openapi.Config{Endpoint: tea.String(endpoint)}
	akID := strings.TrimSpace(cfg.AccessKeyID)
	akSecret := strings.TrimSpace(cfg.AccessKeySecret)
	switch {
	case akID != "" && akSecret != "":
		openapiConfig.AccessKeyId = tea.String(akID)
		openapiConfig.AccessKeySecret = tea.String(akSecret)
	case akID == "" && akSecret == "":
		cred, err := credential.NewCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("aliyun sms: init credential chain: %w", err)
		}
		openapiConfig.Credential = cred
	default:
		return nil, fmt.Errorf("aliyun sms: access_key_id and access_key_secret must be both set or both empty")
	}

	client, err := dypnsapi.NewClient(openapiConfig)
	if err != nil {
		return nil, fmt.Errorf("aliyun sms: init client: %w", err)
	}

	readTimeout := cfg.TimeoutMillis
	if readTimeout <= 0 {
		readTimeout = defaultAliyunReadTimeoutMillis
	}

	return &AliyunSender{
		client:         client,
		signName:       cfg.SignName,
		templateCode:   cfg.TemplateCode,
		codeParamName:  codeParam,
		minParamName:   minParam,
		validMinutes:   validMinutes,
		connectTimeout: defaultAliyunConnectTimeoutMillis,
		readTimeout:    readTimeout,
	}, nil
}

// SendLoginOTP 调用 Dypns SendSmsVerifyCode 发送验证码。
// phoneE164 形如 +8613800138000；验证码由 IAM 传入，校验仍在 IAM 侧完成。
func (s *AliyunSender) SendLoginOTP(ctx context.Context, phoneE164, code string) error {
	countryCode, nationalNumber, err := dypnsPhoneParts(phoneE164)
	if err != nil {
		return fmt.Errorf("aliyun sms: phone: %w", err)
	}

	param, err := json.Marshal(map[string]string{
		s.codeParamName: code,
		s.minParamName:  strconv.Itoa(s.validMinutes),
	})
	if err != nil {
		return fmt.Errorf("aliyun sms: marshal template param: %w", err)
	}

	validSeconds := int64(s.validMinutes * 60)
	req := &dypnsapi.SendSmsVerifyCodeRequest{
		CountryCode:   tea.String(countryCode),
		PhoneNumber:   tea.String(nationalNumber),
		SignName:      tea.String(s.signName),
		TemplateCode:  tea.String(s.templateCode),
		TemplateParam: tea.String(string(param)),
		ValidTime:     tea.Int64(validSeconds),
	}
	runtime := &util.RuntimeOptions{
		ConnectTimeout: tea.Int(s.connectTimeout),
		ReadTimeout:    tea.Int(s.readTimeout),
	}

	resp, err := s.client.SendSmsVerifyCodeWithOptions(req, runtime)
	if err != nil {
		return fmt.Errorf("aliyun sms: send: %w", err)
	}
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("aliyun sms: empty response")
	}
	if tea.StringValue(resp.Body.Code) != "OK" {
		return fmt.Errorf("aliyun sms: send failed code=%s msg=%s requestId=%s",
			tea.StringValue(resp.Body.Code),
			tea.StringValue(resp.Body.Message),
			tea.StringValue(resp.Body.RequestId),
		)
	}

	var bizID string
	if resp.Body.Model != nil {
		bizID = tea.StringValue(resp.Body.Model.BizId)
	}
	logger.L(ctx).Infow("aliyun sms login otp sent",
		"biz_id", bizID,
		"request_id", tea.StringValue(resp.Body.RequestId),
	)
	return nil
}

func dypnsPhoneParts(phoneE164 string) (countryCode, nationalNumber string, err error) {
	n, err := phonenumbers.Parse(strings.TrimSpace(phoneE164), "")
	if err != nil {
		return "", "", fmt.Errorf("parse: %w", err)
	}
	cc := n.GetCountryCode()
	if cc <= 0 {
		return "", "", fmt.Errorf("missing country code")
	}
	national := n.GetNationalNumber()
	if national <= 0 {
		return "", "", fmt.Errorf("missing national number")
	}
	return strconv.Itoa(int(cc)), strconv.FormatUint(national, 10), nil
}
