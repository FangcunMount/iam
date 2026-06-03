package wechatapp

import "testing"

func TestAppTypeIsValid(t *testing.T) {
	cases := []struct {
		in   AppType
		want bool
	}{
		{MiniProgram, true},
		{MP, true},
		{OpenPlatformWebsite, true},
		{AppType("WebsiteApp"), false},
		{AppType(""), false},
		{AppType("mp"), false},
	}
	for _, c := range cases {
		if got := c.in.IsValid(); got != c.want {
			t.Fatalf("AppType(%q).IsValid()=%v, want %v", c.in, got, c.want)
		}
	}
}
