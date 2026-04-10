package runtime

import "github.com/zhaodengfeng/dtsw/internal/config"

type Renderer interface {
	Name() string
	Render(cfg config.Config) ([]byte, error)
}
