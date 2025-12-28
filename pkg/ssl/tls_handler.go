package ssl

import (
	//"OmniLink/pkg/zlog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/unrolled/secure"
)

func TlsHandler(host string, port int) gin.HandlerFunc {
	return func(c *gin.Context) {
		secureMiddleware := secure.New(secure.Options{
			SSLRedirect: true,
			SSLHost:     host + ":" + strconv.Itoa(port),
		})
		err := secureMiddleware.Process(c.Writer, c.Request)

		// If there was an error, do not continue.
		if err != nil {
			// 仅仅是中止当前 Gin 的处理链，因为 Process 已经处理了响应（重定向）
			// 不要调用 c.Abort()，因为 secure 库已经写入了 Response
			//zlog.Fatal(err.Error())
			return
		}

		c.Next()
	}
}
