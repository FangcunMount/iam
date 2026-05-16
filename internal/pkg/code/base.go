package code

import (
	"github.com/FangcunMount/component-base/pkg/errors"
)

// Base: 平台级非业务错误码 (100001～100999).
const (
	// ErrSuccess - 200: OK.
	ErrSuccess = 100001

	// ErrUnknown - 500: Internal server error.
	ErrUnknown = 100002

	// ErrBind - 400: Error occurred while binding the request body to the struct.
	ErrBind = 100003

	// ErrValidation - 400: Validation failed.
	ErrValidation = 100004

	// ErrInvalidArgument - 400: Invalid argument.
	ErrInvalidArgument = 100005

	// ErrPageNotFound - 404: Page not found.
	ErrPageNotFound = 100006

	// ErrInvalidMessage - 400: Invalid message.
	ErrInvalidMessage = 100007

	// ErrInternalServerError - 500: Internal server error.
	ErrInternalServerError = 100008
)

// Base: 数据库错误 (100101～100199).
const (
	// ErrDatabase - 500: Database error.
	ErrDatabase = 100101
)

// Base: 编码/解码错误 (100201～100299).
const (
	// ErrEncodingFailed - 500: Encoding failed due to an error with the data.
	ErrEncodingFailed = 100201

	// ErrDecodingFailed - 500: Decoding failed due to an error with the data.
	ErrDecodingFailed = 100202

	// ErrInvalidJSON - 500: Data is not valid JSON.
	ErrInvalidJSON = 100203

	// ErrEncodingJSON - 500: JSON data could not be encoded.
	ErrEncodingJSON = 100204

	// ErrDecodingJSON - 500: JSON data could not be decoded.
	ErrDecodingJSON = 100205

	// ErrInvalidYaml - 500: Data is not valid Yaml.
	ErrInvalidYaml = 100206

	// ErrEncodingYaml - 500: Yaml data could not be encoded.
	ErrEncodingYaml = 100207

	// ErrDecodingYaml - 500: Yaml data could not be decoded.
	ErrDecodingYaml = 100208
)

// Base: 模块错误 (100301～100399).
const (
	// ErrModuleInitializationFailed - 500: Module initialization failed.
	ErrModuleInitializationFailed = 100301

	// ErrModuleNotFound - 404: Module not found.
	ErrModuleNotFound = 100302

	// ErrRateLimited - 429: Too many requests.
	ErrRateLimited = 100303
)

func init() {
	registerBase(ErrSuccess, 200, "OK")
	registerBase(ErrUnknown, 500, "Internal server error")
	registerBase(ErrBind, 400, "Error occurred while binding the request body to the struct")
	registerBase(ErrValidation, 400, "Validation failed")
	registerBase(ErrInvalidArgument, 400, "Invalid argument")
	registerBase(ErrPageNotFound, 404, "Page not found")
	registerBase(ErrInvalidMessage, 400, "Invalid message")
	registerBase(ErrInternalServerError, 500, "Internal server error")
	registerBase(ErrDatabase, 500, "Database error")
	registerBase(ErrEncodingFailed, 500, "Encoding failed due to an error with the data")
	registerBase(ErrDecodingFailed, 500, "Decoding failed due to an error with the data")
	registerBase(ErrInvalidJSON, 500, "Data is not valid JSON")
	registerBase(ErrEncodingJSON, 500, "JSON data could not be encoded")
	registerBase(ErrDecodingJSON, 500, "JSON data could not be decoded")
	registerBase(ErrInvalidYaml, 500, "Data is not valid Yaml")
	registerBase(ErrEncodingYaml, 500, "Yaml data could not be encoded")
	registerBase(ErrDecodingYaml, 500, "Yaml data could not be decoded")
	registerBase(ErrModuleInitializationFailed, 500, "Module initialization failed")
	registerBase(ErrModuleNotFound, 404, "Module not found")
	registerBase(ErrRateLimited, 429, "Too many requests")
}

func registerBase(code int, httpStatus int, message string) {
	errors.MustRegister(&baseCoder{
		code:       code,
		httpStatus: httpStatus,
		message:    message,
	})
}

type baseCoder struct {
	code       int
	httpStatus int
	message    string
}

func (c *baseCoder) Code() int {
	return c.code
}

func (c *baseCoder) String() string {
	return c.message
}

func (c *baseCoder) Reference() string {
	return ""
}

func (c *baseCoder) HTTPStatus() int {
	return c.httpStatus
}
