package handler

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
)

func advanceOpenAIWSCyberBlockState(_, _, _ bool, _ error) (bool, bool) {
	// 本地不再对 cyber 做连接级后续封禁。调用点仍保留，后续 turn 不会被关掉。
	return false, false
}

func openAIWSIngressEndedByClient(err error) bool {
	if err == nil {
		return true
	}
	var closeErr *service.OpenAIWSClientCloseError
	if errors.As(err, &closeErr) && closeErr.StatusCode() == coderws.StatusNormalClosure {
		return true
	}
	if coderws.CloseStatus(err) == coderws.StatusNormalClosure {
		return true
	}
	return errors.Is(err, context.Canceled)
}
