package fallback

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

var pageTemplate = template.Must(template.New("fallback").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }}</title>
  <style>
    :root { color-scheme: light; }
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: linear-gradient(135deg, #f3efe6, #ffffff); color: #1e1c1a; }
    main { max-width: 720px; margin: 12vh auto; padding: 32px; }
    h1 { font-size: 3rem; margin-bottom: 0.5rem; }
    p { font-size: 1.1rem; line-height: 1.7; }
  </style>
</head>
<body>
  <main>
    <h1>{{ .Title }}</h1>
    <p>{{ .Message }}</p>
  </main>
</body>
</html>
`))

func Serve(ctx context.Context, cfg config.Config) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(cfg.Fallback.StatusCode)
		_ = pageTemplate.Execute(w, map[string]string{
			"Title":   cfg.Fallback.SiteTitle,
			"Message": cfg.Fallback.SiteMessage,
		})
	})

	server := &http.Server{
		Addr:              cfg.Fallback.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
