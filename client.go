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

// ClientOption is a function that configures a Client.
type ClientOption func(client *Client)

// clientConfig holds all mutable configuration for a Client.
type clientConfig struct {
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
	numWorkers       int
}

// WithHost sets the host for the Client.
func WithHost(host string) ClientOption {
	return func(c *Client) {
		c.cfg.host = host
		c.setLinkPrefix()
	}
}

// WithRPS sets the requests per second limit for the Client.
func WithRPS(rps int) ClientOption {
	return func(c *Client) {
		c.cfg.rps = rps
		c.setRateLimiter()
	}
}

// WithTimeout sets the HTTP client timeout for the Client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.cfg.timeout = timeout
	}
}

// WithTest sets the test mode for the Client.
func WithTest(test bool) ClientOption {
	return func(c *Client) {
		c.cfg.test = test
		c.setLinkPrefix()
	}
}

// WithHTTPClient sets the custom HTTP client for the Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.cfg.httpClient = client
	}
}

// WithDefaultParseMode sets the default parse mode for the Client.
func WithDefaultParseMode(parseMode string) ClientOption {
	return func(c *Client) {
		c.cfg.defaultParseMode = parseMode
	}
}

// WithRouter sets the router for the Client.
func WithRouter(router Processor) ClientOption {
	return func(c *Client) {
		c.cfg.router = router
	}
}

// WithNumWorkers sets the number of concurrent update-processing goroutines.
// By default equals to the GetUpdates Limit value.
func WithNumWorkers(n int) ClientOption {
	return func(c *Client) {
		c.cfg.numWorkers = n
	}
}

var defaultOpts = []ClientOption{
	WithHost(defaultHost),
	WithRPS(defaultRPS),
	WithTimeout(defaultTimeout),
	WithRouter(NewRouter()),
	WithHTTPClient(http.DefaultClient),
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

	runMu sync.Mutex
	run   *runState
}

type runState struct {
	stop     chan struct{}
	stopOnce sync.Once
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

// defaultParseMode returns the current default parse mode.
// Called by generated API methods instead of accessing cfg directly.
func (c *Client) defaultParseMode() string {
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

	for _, opt := range defaultOpts {
		opt(c)
	}

	for _, opt := range opts {
		opt(c)
	}

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
	if err := c.cfg.rateLimiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	return c.cfg.httpClient.Do(req) //nolint:gosec // G107: client can send request to user-defined hosts
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
	dst []byte,
) (json.RawMessage, error) {
	innerCtx := httptrace.WithClientTrace(ctx, c.httpTrace)

	timeout := c.cfg.timeout
	link := c.cfg.linkPrefix + method

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

	buffer := acquireBuffer()
	defer releaseBuffer(buffer)

	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return nil, err
	}

	var v Response

	err = json.Unmarshal(buffer.Bytes(), &v)
	if err != nil {
		return nil, err
	}

	if !v.OK {
		err = genError(v.ErrorCode, resp.Status, v.Description, v.Parameters)
		return c.handleRetryErr(ctx, method, reader, contentType, err, dst)
	}

	return append(dst, v.Result...), nil
}

func (c *Client) handleRetryErr(
	ctx context.Context,
	method string,
	reader io.Reader,
	contentType string,
	err error,
	dst []byte,
) (json.RawMessage, error) {
	const retryLimit = 5

	if retryErr, ok := errors.AsType[*RetryError](err); ok {
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
					// Cannot rewind reader, retry would send empty body.
					return nil, err
				}
			}

			return c.Raw(ctx, method, reader, contentType, dst)
		}
	}

	return nil, err
}

func (c *Client) startPolling(ctx context.Context, params *GetUpdatesParams, workers []chan *Context) {
	numWorkers := int64(len(workers))
	router := c.cfg.router

	for {
		select {
		case <-ctx.Done():
			return

		default:
		}

		batch, err := c.GetUpdates(ctx, params)
		if err != nil {
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
			gogramCtx := c.acquireContext(ctx, &batch[i])

			// send update to same worker
			var chatID int64

			if chat := gogramCtx.Chat(); chat != nil {
				chatID = chat.ID
			} else {
				chatID = batch[i].UpdateID
			}

			idx := chatID % numWorkers
			if idx < 0 {
				idx = -idx
			}

			select {
			case workers[idx] <- gogramCtx:
			case <-ctx.Done():
				c.releaseContext(gogramCtx)
				return
			}
		}
	}
}

func (c *Client) beginRun(ctx context.Context) (context.Context, *runState, error) {
	state := &runState{
		stop: make(chan struct{}),
	}

	c.runMu.Lock()
	if c.run != nil {
		c.runMu.Unlock()
		return nil, nil, ErrAlreadyStarted
	}
	c.run = state
	c.runMu.Unlock()

	innerCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-ctx.Done():
		case <-state.stop:
		}
		cancel()
	}()

	return innerCtx, state, nil
}

func (c *Client) finishRun(state *runState) {
	state.stopOnce.Do(func() { close(state.stop) })

	c.runMu.Lock()
	if c.run == state {
		c.run = nil
	}
	c.runMu.Unlock()
}

// Stop requests the active polling or webhook run to stop.
//
// Stop never waits for handlers to finish, so it is safe to call from a
// handler. Start or StartWebhook returns after the run has fully drained.
func (c *Client) Stop() {
	c.runMu.Lock()
	state := c.run
	c.runMu.Unlock()

	if state != nil {
		state.stopOnce.Do(func() { close(state.stop) })
	}
}
