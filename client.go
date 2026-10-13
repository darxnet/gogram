package gogram

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// The generator is its own module so that golang.org/x/net stays out of the library's dependencies.
//go:generate go run -C ./cmd/gen . -out ../..

const (
	defaultHost    = "api.telegram.org"
	defaultRPS     = 30
	defaultTimeout = 25 * time.Second
	defaultUpdates = 100

	// pollGrace is added to the long-poll timeout for the HTTP deadline of
	// getUpdates: Telegram answers at the timeout boundary and the answer
	// (up to 100 updates) still has to travel back. Too small means spurious
	// deadline errors plus a backoff; too large only delays detecting a hung
	// connection by the same amount.
	pollGrace = 5 * time.Second

	retryLimit = 5

	contentTypeJSON = "application/json"
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
	caller           Caller
	transformers     []Transformer
	store            StateStore
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

// WithTimeout sets the per-request HTTP timeout for the Client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.cfg.timeout = timeout
	}
}

// WithTestEnvironment routes calls to Telegram's test environment (/test/ path).
func WithTestEnvironment(test bool) ClientOption {
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

// WithCaller replaces the HTTP transport of the client with caller: every
// API call ends in it. Transformers registered with [WithTransformers]
// still wrap it. Meant for tests and alternative transports.
func WithCaller(caller Caller) ClientOption {
	return func(c *Client) {
		c.cfg.caller = caller
	}
}

// WithTransformers wraps the client's Caller: the first transformer is the
// outermost, so it sees the request first and the result last. Use it for
// logging, tracing, throttling, or mocking API calls.
func WithTransformers(transformers ...Transformer) ClientOption {
	return func(c *Client) {
		c.cfg.transformers = append(c.cfg.transformers, transformers...)
	}
}

// WithStateStore sets the state storer used by [Context.FSM] and [FilterState]
// when none is passed explicitly. Default: [NewMemoryStateStore] without TTL.
func WithStateStore(store StateStore) ClientOption {
	return func(c *Client) {
		c.cfg.store = store
	}
}

// WithWorkers sets the number of concurrent update-processing goroutines.
// By default equals to the GetUpdates Limit value.
func WithWorkers(n int) ClientOption {
	return func(c *Client) {
		c.cfg.numWorkers = n
	}
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

	trace *httptrace.ClientTrace
	addrs atomic.Pointer[connAddrs]

	caller Caller // transformer chain ending in cfg.caller or httpCaller
	info   botInfo

	run atomic.Pointer[runState]
}

// connAddrs is the address pair of the connection the last request went
// through. Stored as one pointer so both addresses change atomically.
type connAddrs struct {
	conn          net.Conn
	local, remote net.Addr
}

// runState is one Start/StartWebhook run.
type runState struct {
	cancel context.CancelFunc
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
	limit, burst := rate.Inf, 0
	if c.cfg.rps > 0 {
		limit = rate.Every(time.Second / time.Duration(c.cfg.rps))
		burst = c.cfg.rps
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
	// ErrBodyNotRewindable indicates a retryable API error on a request whose
	// body cannot be replayed (streamed multipart upload).
	ErrBodyNotRewindable = errors.New("gogram: request body cannot be rewound for retry")
)

// NewClient creates a new Client with the provided token and options.
func NewClient(token string, opts ...ClientOption) (*Client, error) {
	if token == "" {
		return nil, ErrNoToken
	}

	botIDText, _, _ := strings.Cut(token, ":")
	botID, err := strconv.ParseInt(botIDText, 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	c := &Client{
		id:    botID,
		token: token,
		cfg: clientConfig{
			host:       defaultHost,
			rps:        defaultRPS,
			timeout:    defaultTimeout,
			httpClient: http.DefaultClient,
			router:     NewRouter(), // one per client, never shared
			store:      NewMemoryStateStore(),
		},
	}

	c.setLinkPrefix()
	c.setRateLimiter()

	c.trace = &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			// A reused connection stores nothing: no allocation per request.
			if cur := c.addrs.Load(); cur != nil && cur.conn == info.Conn {
				return
			}

			c.addrs.Store(&connAddrs{
				conn:   info.Conn,
				local:  info.Conn.LocalAddr(),
				remote: info.Conn.RemoteAddr(),
			})
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	c.caller = c.cfg.caller
	if c.caller == nil {
		c.caller = httpCaller{client: c}
	}

	for _, transform := range slices.Backward(c.cfg.transformers) {
		c.caller = transform(c.caller)
	}

	return c, nil
}

// ID returns the bot's ID.
func (c *Client) ID() int64 {
	return c.id
}

// SameToken reports whether token equals the client's token, in constant time.
// Use it to detect a rotated token without exposing the current one.
func (c *Client) SameToken(token string) bool {
	return subtle.ConstantTimeCompare([]byte(c.token), []byte(token)) == 1
}

// LocalAddr returns the local network address of the last connection the
// client used. On a multi-homed host the kernel may route each connection
// through a different interface; this reports which one actually carried
// the traffic.
func (c *Client) LocalAddr() net.Addr {
	if a := c.addrs.Load(); a != nil {
		return a.local
	}

	return nil
}

// RemoteAddr returns the remote network address of the last connection the
// client used, see [Client.LocalAddr].
func (c *Client) RemoteAddr() net.Addr {
	if a := c.addrs.Load(); a != nil {
		return a.remote
	}

	return nil
}

// do sends an HTTP request through the rate limiter.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	if err := c.cfg.rateLimiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	return c.cfg.httpClient.Do(req) //nolint:gosec // G107: client can send request to user-defined hosts
}

// Call invokes a Bot API method with params encoded as JSON and decodes
// the result into result, a pointer to the method's return type. It is the
// escape hatch for methods without a generated wrapper:
//
//	var msg gogram.Message
//	err := client.Call(ctx, "sendMessage", map[string]any{"chat_id": 7, "text": "hi"}, &msg)
//
// Transient errors are retried like for generated methods.
func (c *Client) Call(ctx context.Context, method string, params, result any) error {
	buffer := acquireBuffer()
	defer releaseBuffer(buffer)

	if err := json.NewEncoder(buffer).Encode(params); err != nil {
		return err
	}

	return c.caller.Call(ctx, Request{
		Method:      method,
		Params:      params,
		Body:        bytes.NewReader(buffer.B),
		ContentType: contentTypeJSON,
		Timeout:     c.cfg.timeout,
	}, result)
}

// call is the generated methods' entry point: it routes the request through
// the client's Caller chain and decodes the result into T.
func call[T any](
	ctx context.Context,
	c *Client,
	method string,
	params any,
	body io.Reader,
	contentType string,
	timeout time.Duration,
) (ret T, err error) {
	err = c.caller.Call(ctx, Request{
		Method:      method,
		Params:      params,
		Body:        body,
		ContentType: contentType,
		Timeout:     timeout,
	}, &ret)

	return ret, err
}

// getUpdates is the polling-loop variant of GetUpdates: the HTTP deadline
// is derived from the long-poll timeout instead of cfg.timeout, see pollGrace.
func (c *Client) getUpdates(ctx context.Context, params *GetUpdatesParams) ([]Update, error) {
	buffer := acquireBuffer()
	defer releaseBuffer(buffer)

	if err := json.NewEncoder(buffer).Encode(params); err != nil {
		return nil, err
	}

	timeout := time.Duration(params.Timeout)*time.Second + pollGrace

	return call[[]Update](ctx, c, "getUpdates", params, bytes.NewReader(buffer.B), contentTypeJSON, timeout)
}

// poll runs the long-polling loop and returns ctx.Err() when ctx is done or
// the API error when the bot is unauthorized or banned.
func (c *Client) poll(ctx context.Context, params *GetUpdatesParams, workers []chan *Context) error {
	numWorkers := int64(len(workers))
	router := c.cfg.router

	for {
		batch, err := c.getUpdates(ctx, params)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			gogramCtx := c.acquireContext(ctx, nil)
			router.HandleErr(gogramCtx, err)
			c.releaseContext(gogramCtx)

			if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFoundBanned) {
				return err
			}

			if sleepErr := sleep(ctx, defaultTimeoutOnError); sleepErr != nil {
				return sleepErr
			}

			continue
		}

		for i := range batch {
			params.Offset = batch[i].UpdateID + 1
			gogramCtx := c.acquireContext(ctx, &batch[i])

			// Updates of one chat always go to the same worker: ordering per chat.
			key := batch[i].UpdateID
			if chat := gogramCtx.Chat(); chat != nil {
				key = chat.ID
			}

			idx := key % numWorkers
			if idx < 0 {
				idx = -idx
			}

			select {
			case workers[idx] <- gogramCtx:
			case <-ctx.Done():
				c.releaseContext(gogramCtx)
				return ctx.Err()
			}
		}
	}
}

func (c *Client) beginRun(ctx context.Context) (context.Context, *runState, error) {
	innerCtx, cancel := context.WithCancel(ctx)
	state := &runState{cancel: cancel}

	if !c.run.CompareAndSwap(nil, state) {
		cancel()
		return nil, nil, ErrAlreadyStarted
	}

	return innerCtx, state, nil
}

func (c *Client) finishRun(state *runState) {
	state.cancel()
	c.run.CompareAndSwap(state, nil)
}

// Stop requests the active polling or webhook run to stop.
//
// Stop never waits for handlers to finish, so it is safe to call from a
// handler. Start or StartWebhook returns after the run has fully drained.
func (c *Client) Stop() {
	if state := c.run.Load(); state != nil {
		state.cancel()
	}
}
