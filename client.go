package gogram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

//go:generate go run ./cmd/gen

const (
	defaultHost    = "api.telegram.org"
	defaultRPS     = 30
	defaultTimeout = 25 * time.Second
	defaultUpdates = 100
)

// Processor is an interface for processing updates.
type Processor interface {
	Process(ctx *Context)
	HandleErr(ctx *Context, err error)
	HandlePanic(ctx *Context, v any)
}

// ClientOption is a function that configures a Client.
type ClientOption func(client *Client) ClientOption

// clientConfig holds all mutable configuration for a Client.
type clientConfig struct {
	rw sync.RWMutex

	host             string
	test             bool
	linkPrefix       string
	linkFilePrefix   string
	rps              int
	timeout          time.Duration
	httpClient       *http.Client
	rateLimiter      *rate.Limiter
	router           Processor
	defaultParseMode string
}

// WithHost sets the host for the Client.
func WithHost(host string) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.host
		c.cfg.host = host
		c.setLinkPrefix()

		return WithHost(previous)
	}
}

// WithRPS sets the requests per second limit for the Client.
func WithRPS(rps int) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.rps
		c.cfg.rps = rps
		c.setRateLimiter()

		return WithRPS(previous)
	}
}

// WithTimeout sets the HTTP client timeout for the Client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.timeout
		c.cfg.timeout = timeout

		return WithTimeout(previous)
	}
}

// WithTest sets the test mode for the Client.
func WithTest(test bool) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.test
		c.cfg.test = test
		c.setLinkPrefix()

		return WithTest(previous)
	}
}

// WithHTTPClient sets the custom HTTP client for the Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.httpClient
		c.cfg.httpClient = client

		if previous == nil {
			return WithHTTPClient(http.DefaultClient)
		}

		return WithHTTPClient(previous)
	}
}

// WithDefaultParseMode sets the default parse mode for the Client.
func WithDefaultParseMode(parseMode string) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.defaultParseMode
		c.cfg.defaultParseMode = parseMode

		return WithDefaultParseMode(previous)
	}
}

// WithRouter sets the router for the Client.
func WithRouter(router Processor) ClientOption {
	return func(c *Client) ClientOption {
		c.cfg.rw.Lock()
		defer c.cfg.rw.Unlock()

		previous := c.cfg.router
		c.cfg.router = router

		if previous == nil {
			return WithRouter(NewRouter())
		}

		return WithRouter(previous)
	}
}

var defaultOpts = []ClientOption{
	WithHost(defaultHost),
	WithRPS(defaultRPS),
	WithTimeout(defaultTimeout),
	WithRouter(NewRouter()),
}

// Client is a Telegram Bot API client.
//
// References:
//   - https://core.telegram.org/bots/api
//   - https://core.telegram.org/bots/webapps
type Client struct {
	id    int64
	token string

	cfg clientConfig

	httpTrace             *httptrace.ClientTrace
	localAddr, remoteAddr atomic.Value

	cancel atomic.Pointer[context.CancelFunc]
}

func (c *Client) setLinkPrefix() {
	linkPrefix := "https://" + c.cfg.host + "/bot" + c.token + "/"
	linkFilePrefix := "https://" + c.cfg.host + "/file/bot" + c.token + "/"

	if c.cfg.test {
		linkPrefix += "test/"
		linkFilePrefix += "test/"
	}

	c.cfg.linkPrefix = linkPrefix
	c.cfg.linkFilePrefix = linkFilePrefix
}

func (c *Client) setRateLimiter() {
	var limit rate.Limit

	burst := c.cfg.rps
	if c.cfg.rps > 0 {
		limit = rate.Every(time.Second / time.Duration(c.cfg.rps))
	} else {
		limit = rate.Inf
		burst = 0
	}

	c.cfg.rateLimiter = rate.NewLimiter(limit, burst)
}

// Option applies one or more ClientOption values and returns
// the last rollback option produced by the applied options.
func (c *Client) Option(opts ...ClientOption) []ClientOption {
	rollbacks := make([]ClientOption, 0, len(opts))

	for _, opt := range opts {
		rollbacks = append(rollbacks, opt(c))
	}

	return rollbacks
}

// defaultParseModeValue returns the current default parse mode.
// Called by generated API methods instead of accessing cfg directly.
func (c *Client) defaultParseMode() string {
	c.cfg.rw.RLock()
	defer c.cfg.rw.RUnlock()

	return c.cfg.defaultParseMode
}

// Client errors returned by constructors and runtime operations.
var (
	// ErrNoToken indicates that an empty bot token was provided.
	ErrNoToken = errors.New("gogram: no token provided")
	// ErrInvalidToken indicates that the provided bot token is malformed.
	ErrInvalidToken = errors.New("gogram: invalid token provided")
	// ErrAlreadyStarted indicates that Start was called for an already running client.
	ErrAlreadyStarted = errors.New("gogram: already started")
)

// NewClient creates a new Client with the provided token and options.
func NewClient(token string, opts ...ClientOption) (*Client, error) {
	if token == "" {
		return nil, ErrNoToken
	}

	botID, err := strconv.ParseInt(strings.SplitN(token, ":", 2)[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	c := new(Client)

	c.id = botID
	c.token = token

	c.httpTrace = &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			c.localAddr.Store(info.Conn.LocalAddr())
			c.remoteAddr.Store(info.Conn.RemoteAddr())
		},
	}

	c.Option(defaultOpts...)
	c.Option(opts...)

	return c, nil
}

// ID returns the bot's ID.
func (c *Client) ID() int64 {
	return c.id
}

// Token returns the bot's token. Be aware, it is credentials.
func (c *Client) Token() string {
	return c.token
}

// LocalAddr returns the local network address used by the client.
func (c *Client) LocalAddr() net.Addr {
	v, _ := c.localAddr.Load().(net.Addr)
	return v
}

// RemoteAddr returns the remote network address used by the client.
func (c *Client) RemoteAddr() net.Addr {
	v, _ := c.remoteAddr.Load().(net.Addr)
	return v
}

// Do sends an HTTP request and returns an HTTP response.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.cfg.rw.RLock()
	limiter := c.cfg.rateLimiter
	client := c.cfg.httpClient
	c.cfg.rw.RUnlock()

	if err := limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	return client.Do(req) //nolint:gosec // G704: client can send request to user-defined hosts
}

type contextKey struct {
	name string
}

var retryCountContextKey = &contextKey{name: "retry-count"}

// Raw sends a raw request to the Telegram Bot API.
func (c *Client) Raw(
	ctx context.Context,
	method string,
	reader io.Reader,
	contentType string,
) (json.RawMessage, error) {
	innerCtx := httptrace.WithClientTrace(ctx, c.httpTrace)

	c.cfg.rw.RLock()
	timeout := c.cfg.timeout
	link := c.cfg.linkPrefix + method
	c.cfg.rw.RUnlock()

	if timeout > 0 {
		var cancel context.CancelFunc
		innerCtx, cancel = context.WithTimeout(innerCtx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(innerCtx, http.MethodPost, link, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var v Response

	err = json.Unmarshal(buf, &v)
	if err != nil {
		return nil, err
	}

	if !v.OK {
		err = genError(v.ErrorCode, resp.Status, v.Description, v.Parameters)
		return c.handleRetryErr(ctx, method, reader, contentType, err)
	}

	return v.Result, nil
}

func (c *Client) handleRetryErr(
	ctx context.Context,
	method string,
	reader io.Reader,
	contentType string,
	err error,
) (json.RawMessage, error) {
	const retryLimit = 5

	var retryErr *RetryError

	if errors.As(err, &retryErr) {
		retryCount := 0
		if v := ctx.Value(retryCountContextKey); v != nil {
			retryCount = v.(int)
		}

		if retryCount < retryLimit {
			retryCount++
			ctx = context.WithValue(ctx, retryCountContextKey, retryCount)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryErr.RetryAfter):
			}

			if reader != nil {
				if seeker, ok := reader.(io.Seeker); ok {
					if _, err = seeker.Seek(0, io.SeekStart); err != nil {
						return nil, err
					}
				} else {
					// Cannot rewind reader, retry would send empty body
					return nil, err
				}
			}

			return c.Raw(ctx, method, reader, contentType)
		}
	}

	return nil, err
}

// Start starts the client and listens for updates.
func (c *Client) Start(ctx context.Context, params *GetUpdatesParams) error {
	oldCancel := c.cancel.Load()
	if oldCancel != nil {
		return ErrAlreadyStarted
	}

	innerCtx, cancel := context.WithCancel(ctx)

	if !c.cancel.CompareAndSwap(oldCancel, &cancel) {
		cancel()
		return ErrAlreadyStarted
	}

	defer func() {
		cancel()
		c.cancel.Store(nil)
	}()

	var localParams GetUpdatesParams

	if params != nil {
		localParams = *params
	}

	if localParams.Limit == 0 {
		localParams.Limit = defaultUpdates
	}

	updates := make(chan *Update, localParams.Limit)

	var wg sync.WaitGroup

	wg.Go(func() {
		defer close(updates)
		c.startPolling(innerCtx, updates, &localParams)
	})

	for range localParams.Limit {
		wg.Go(func() {
			for u := range updates {
				c.processUpdate(innerCtx, u)
			}
		})
	}

	wg.Wait()

	if err := ctx.Err(); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (c *Client) startPolling(ctx context.Context, updates chan<- *Update, params *GetUpdatesParams) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			batch, err := c.GetUpdates(ctx, params)
			if err != nil {
				c.cfg.rw.RLock()
				router := c.cfg.router
				c.cfg.rw.RUnlock()

				gogramCtx := c.acquireContext(ctx, nil)
				router.HandleErr(gogramCtx, err)
				c.releaseContext(gogramCtx)

				if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFoundBanned) {
					return
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(defaultTimeoutOnError):
				}

				continue
			}

			for i := range batch {
				params.Offset = batch[i].UpdateID + 1
				updates <- &batch[i]
			}
		}
	}
}

// Stop stops the Client.
func (c *Client) Stop() {
	cancel := c.cancel.Load()
	if cancel != nil {
		(*cancel)()
	}
}

func (c *Client) processUpdate(ctx context.Context, u *Update) {
	gogramCtx := c.acquireContext(ctx, u)
	defer c.releaseContext(gogramCtx)

	c.cfg.rw.RLock()
	router := c.cfg.router
	c.cfg.rw.RUnlock()

	router.Process(gogramCtx)
}
