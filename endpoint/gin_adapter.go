package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/smilemakc/mbkit/internal/l"
	"github.com/smilemakc/mbkit/policy"

	"github.com/uptrace/bun"
)

type PrincipalProvider func(c *gin.Context) (*policy.Principal, error)

type GinRequest struct{ C *gin.Context }

func (g GinRequest) Context() context.Context           { return g.C }
func (g GinRequest) PathParam(k string) (string, bool)  { v := g.C.Param(k); return v, v != "" }
func (g GinRequest) QueryParam(k string) (string, bool) { v, ok := g.C.GetQuery(k); return v, ok }
func (g GinRequest) Header(k string) string             { return g.C.GetHeader(k) }
func (g GinRequest) BindJSON(dst any) error             { return g.C.ShouldBindJSON(dst) }
func (g GinRequest) BindQuery(dst any) error            { return g.C.ShouldBindQuery(dst) }
func (g GinRequest) BindForm(dst any) error             { return g.C.ShouldBindWith(dst, binding.FormMultipart) }

type TxRunner func(c *gin.Context, fn func(ctx context.Context, tx bun.IDB) error) error

func RunInTransactionRunner(db bun.IDB) TxRunner {
	return func(c *gin.Context, fn func(ctx context.Context, tx bun.IDB) error) error {
		return db.RunInTx(c, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			return fn(ctx, tx)
		})
	}
}

type (
	GinOKWriter  func(c *gin.Context, status int, data any)
	GinErrWriter func(c *gin.Context, err error)
)

// GinAdapter is a gin.IRouter implementation that uses a HTTPService to mount endpoints.
type GinAdapter struct {
	Group  *gin.RouterGroup  // Group is the gin.RouterGroup to mount the endpoints to.
	RunTx  TxRunner          // RunTx executes a transaction with a given transaction function and context from the gin.Context.
	Write  GinOKWriter       // Write is used to write data to the response.
	WriteE GinErrWriter      // WriteE is used to write errors to the response.
	Authn  PrincipalProvider // Authn represents a function to retrieve a Principal from a gin.Context for authentication purposes.
}

func (ga *GinAdapter) WithAuth(provider PrincipalProvider) *GinAdapter {
	ga.Authn = provider
	return ga
}

func (ga *GinAdapter) Mount(base string, endpoints ...Endpoint) {
	if ga.Write == nil {
		ga.Write = DefaultGinOKWriter
	}
	if ga.WriteE == nil {
		ga.WriteE = DefaultGinErrWriter
	}
	if ga.RunTx == nil {
		l.Log.Fatal().Msg("txRunner is required")
	}
	g := ga.Group.Group(base)
	for _, endpoint := range endpoints {
		ep := endpoint
		handler := func(c *gin.Context) {
			if ga.Authn != nil {
				if p, ee := ga.Authn(c); ee == nil && p != nil {
					policy.WithGinPrincipal(c, p)
				}
			}
			req := GinRequest{C: c}

			args, err := ep.Bind(req)
			if err != nil {
				ga.WriteE(c, err)
				return
			}

			var out any
			err = ga.RunTx(c, func(ctx context.Context, tx bun.IDB) error {
				h := ep.Chain()
				var ee error
				out, ee = h(ctx, tx, args)
				return ee
			})
			if err != nil {
				ga.WriteE(c, err)
				return
			}

			status := ep.Status
			if status == 0 {
				switch ep.Method {
				case http.MethodPost:
					status = http.StatusCreated
				case http.MethodDelete:
					status = http.StatusNoContent
				default:
					status = http.StatusOK
				}
			}
			ga.Write(c, status, out)
		}

		g.Handle(ep.Method, ep.Path, handler)
	}
}

func DefaultGinErrWriter(c *gin.Context, err error) {
	status, payload := ClassifyHTTPError(err)
	if status >= http.StatusInternalServerError {
		l.Log.Error().Err(err).Int("status", status).Str("path", c.FullPath()).Msg("request failed")
	}
	c.JSON(status, payload)
}

// DefaultGinHTMLWriter writes an HTML response or just a status if there's no content.
// Useful for serving pre-rendered HTML or template results.
func DefaultGinHTMLWriter(c *gin.Context, status int, html any) {
	if status == http.StatusNoContent {
		c.Status(status)
		return
	}
	data, ok := html.([]byte)
	if !ok {
		c.String(status, "%s", html)
		return
	}
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(status)
	c.Writer.Write(data)
}

func DefaultGinOKWriter(c *gin.Context, status int, data any) {
	if status == http.StatusNoContent {
		c.Status(status)
		return
	}
	c.JSON(status, data)
}

func NewGinAdapter(db bun.IDB, r *gin.RouterGroup) *GinAdapter {
	return &GinAdapter{
		Group:  r,
		RunTx:  RunInTransactionRunner(db),
		Write:  DefaultGinOKWriter,
		WriteE: DefaultGinErrWriter,
	}
}

type GinAdapterBuilder struct {
	txRunner          TxRunner
	r                 *gin.RouterGroup
	okWriter          GinOKWriter
	errWriter         GinErrWriter
	principalProvider PrincipalProvider
}

func (g *GinAdapterBuilder) WithTxRunner(runner TxRunner) *GinAdapterBuilder {
	g.txRunner = runner
	return g
}

func (g *GinAdapterBuilder) WithPrincipalProvider(provider PrincipalProvider) *GinAdapterBuilder {
	g.principalProvider = provider
	return g
}

func (g *GinAdapterBuilder) WithGroup(r *gin.RouterGroup) *GinAdapterBuilder {
	g.r = r
	return g
}

func (g *GinAdapterBuilder) WithOKWriter(w GinOKWriter) *GinAdapterBuilder {
	g.okWriter = w
	return g
}

func (g *GinAdapterBuilder) WithErrWriter(w GinErrWriter) *GinAdapterBuilder {
	g.errWriter = w
	return g
}

func (g *GinAdapterBuilder) Build() (*GinAdapter, error) {
	if g.txRunner == nil {
		return nil, fmt.Errorf("txRunner is required")
	}
	if g.r == nil {
		return nil, fmt.Errorf("router group is required")
	}
	if g.okWriter == nil {
		l.Log.Warn().Msg("okWriter is not set will be used default DefaultGinOKWriter")
		g.okWriter = DefaultGinOKWriter
	}
	if g.errWriter == nil {
		l.Log.Warn().Msg("errWriter is not set will be used default DefaultGinErrWriter")
		g.errWriter = DefaultGinErrWriter
	}
	return &GinAdapter{
		Group:  g.r,
		RunTx:  g.txRunner,
		Write:  g.okWriter,
		WriteE: g.errWriter,
		Authn:  g.principalProvider,
	}, nil
}

func NewGinAdapterBuilder() *GinAdapterBuilder {
	return &GinAdapterBuilder{}
}
