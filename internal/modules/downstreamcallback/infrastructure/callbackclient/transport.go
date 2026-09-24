package callbackclient

import (
	"net/http"
	"time"

	"github.com/dujiao-next/internal/shared/netguard"
)

// newSafeHTTPClient 返回用于向下游回调地址发起请求的 HTTP 客户端：
// 所有请求都复用共享的公网 URL 阻断器，并保持回调原有的“不跟随重定向”行为。
func newSafeHTTPClient() *http.Client {
	client := netguard.NewHTTPClient(15 * time.Second)
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}
