package authn

import (
	"context"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	linkingapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type capturingLinker struct {
	linkingapp.Linker
	request linkingapp.LinkRequest
}

func (s *capturingLinker) Link(_ context.Context, req linkingapp.LinkRequest) (*linkingapp.LinkResult, error) {
	s.request = req
	return &linkingapp.LinkResult{Identity: &loginidentity.LoginIdentity{ID: meta.FromUint64(3)}}, nil
}

func TestLinkServicesForwardOriginalActorAuthenticationTime(t *testing.T) {
	at := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	actor := &authnv2.AuthenticatedUserContext{UserId: "7", AuthenticatedAt: timestamppb.New(at)}
	for _, method := range []string{"phone", "mini", "wecom"} {
		t.Run(method, func(t *testing.T) {
			linker := &capturingLinker{}
			s := &loginIdentityServiceServer{linking: linker}
			var err error
			switch method {
			case "phone":
				_, err = s.LinkPhone(context.Background(), &authnv2.LinkPhoneRequest{Actor: actor})
			case "mini":
				_, err = s.LinkWechatMiniProgram(context.Background(), &authnv2.LinkWechatMiniProgramRequest{Actor: actor})
			case "wecom":
				_, err = s.LinkWecom(context.Background(), &authnv2.LinkWecomRequest{Actor: actor})
			}
			require.NoError(t, err)
			require.Equal(t, meta.FromUint64(7), linker.request.UserID)
			require.Equal(t, &at, linker.request.AuthenticatedAt)
		})
	}
}
