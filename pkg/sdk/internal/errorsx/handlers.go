package errorsx

import (
	"fmt"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
)

type ErrorHandler struct {
	handlers map[codes.Code]func(error) error
	fallback func(error) error
}

func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		handlers: make(map[codes.Code]func(error) error),
		fallback: func(err error) error { return err },
	}
}

func (h *ErrorHandler) On(code codes.Code, handler func(error) error) *ErrorHandler {
	h.handlers[code] = handler
	return h
}

func (h *ErrorHandler) OnAuth(handler func(error) error) *ErrorHandler {
	h.handlers[codes.Unauthenticated] = handler
	h.handlers[codes.PermissionDenied] = handler
	return h
}

func (h *ErrorHandler) OnNetwork(handler func(error) error) *ErrorHandler {
	h.handlers[codes.Unavailable] = handler
	h.handlers[codes.DeadlineExceeded] = handler
	return h
}

func (h *ErrorHandler) OnNotFound(handler func(error) error) *ErrorHandler {
	h.handlers[codes.NotFound] = handler
	return h
}

func (h *ErrorHandler) Fallback(handler func(error) error) *ErrorHandler {
	h.fallback = handler
	return h
}

func (h *ErrorHandler) Handle(err error) error {
	if err == nil {
		return nil
	}

	code := sdkerrors.GRPCCode(err)
	if handler, ok := h.handlers[code]; ok {
		return handler(err)
	}
	return h.fallback(err)
}

func MustHandle(err error, handlers ...func(error) error) {
	if err == nil {
		return
	}
	for _, h := range handlers {
		if h(err) == nil {
			return
		}
	}
	panic(fmt.Sprintf("unhandled error: %v", err))
}

func IgnoreNotFound(err error) error {
	if sdkerrors.IsNotFound(err) {
		return nil
	}
	return err
}

func IgnoreAlreadyExists(err error) error {
	if sdkerrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
