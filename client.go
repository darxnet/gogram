package gogram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

//go:generate go run ./cmd/gen

const (
	defaultHost    = "api.telegram.org"
	defaultRPS     = 30
	defaultTimeout = 10 * time.Second
	defaultUpdates = 100
)

// ClientOption is a function that configures a Client.
type ClientOption func(client *Client) ClientOption

// WithHost sets the host for the Client.
func WithHost(host string) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.host
		c.options.host = host

		c.buildLinkPrefix(c.options.test, c.options.host, c.token)

		return WithHost(previous)
	}
}

// WithRPS sets the requests per second limit for the Client.
func WithRPS(rps int) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.rps
		if rps <= 0 {
			rps = defaultRPS
		}

		c.options.rps = rps

		c.rateLimiter.SetLimit(rate.Every(time.Second / time.Duration(rps)))
		c.rateLimiter.SetBurst(rps)

		return WithRPS(previous)
	}
}

// WithTimeout sets the HTTP client timeout for the Client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.timeout
		c.options.timeout = timeout

		c.httpClient.Timeout = timeout

		return WithTimeout(previous)
	}
}

// WithTest sets the test mode for the Client.
func WithTest(test bool) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.test
		c.options.test = test

		c.buildLinkPrefix(c.options.test, c.options.host, c.token)

		return WithTest(previous)
	}
}

// WithHTTPClient sets the custom HTTP client for the Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.httpClient
		if client == nil {
			client = &http.Client{Timeout: c.options.timeout}
		}
		c.httpClient = client

		return WithHTTPClient(previous)
	}
}

// WithDefaultParseMode sets the default parse mode for the Client.
func WithDefaultParseMode(parseMode string) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.defaultParseMode
		c.defaultParseMode = parseMode

		return WithDefaultParseMode(previous)
	}
}

var defaultOpts = []ClientOption{
	WithHost(defaultHost),
	WithRPS(defaultRPS),
	WithTest(false),
	WithTimeout(defaultTimeout),
}

type clientOption struct {
	host    string
	rps     int
	timeout time.Duration
	test    bool
}

// Client is a Telegram Bot API client.
//
// References:
//   - https://core.telegram.org/bots/api
//   - https://core.telegram.org/bots/webapps
type Client struct {
	id      int64
	token   string
	options clientOption

	linkPrefix string

	httpTrace             *httptrace.ClientTrace
	httpClient            *http.Client
	localAddr, remoteAddr net.Addr

	rateLimiter *rate.Limiter

	defaultParseMode string

	updates chan *Update
	done    chan struct{}
	started bool

	router *Router

	locker sync.RWMutex
}

func (c *Client) buildLinkPrefix(test bool, host, token string) {
	if test {
		c.linkPrefix = "https://" + host + "/bot" + token + "/test/"
	} else {
		c.linkPrefix = "https://" + host + "/bot" + token + "/"
	}
}

// Option applies one or more ClientOption values and returns
// the last rollback option produced by the applied options.
func (c *Client) Option(opts ...ClientOption) (previous ClientOption) {
	for _, opt := range opts {
		previous = opt(c)
	}
	return previous
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
			c.localAddr = info.Conn.LocalAddr()
			c.remoteAddr = info.Conn.RemoteAddr()
		},
	}

	c.httpClient = &http.Client{}
	c.rateLimiter = &rate.Limiter{}

	c.router = NewRouter()

	c.Option(defaultOpts...)
	c.Option(opts...)

	return c, nil
}

// ID returns the bot's ID.
func (c *Client) ID() int64 {
	return c.id
}

// Token returns the bot's token.
func (c *Client) Token() string {
	return c.token
}

// LocalAddr returns the local network address used by the client.
func (c *Client) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address used by the client.
func (c *Client) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// Router returns the client's router.
func (c *Client) Router() *Router {
	return c.router
}

// SetRouter sets the client's router.
func (c *Client) SetRouter(r *Router) {
	c.router = r
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	err := c.rateLimiter.Wait(req.Context())
	if err != nil {
		return nil, err
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), c.httpTrace))

	//nolint:gosec // G704: client can send request to user-defined hosts
	return c.httpClient.Do(req)
}

// Raw sends a raw request to the Telegram Bot API.
func (c *Client) Raw(method string, reader io.Reader, contentType ...string) (json.RawMessage, error) {
	link := c.linkPrefix + method

	req, err := http.NewRequest(http.MethodPost, link, reader)
	if err != nil {
		return nil, err
	}

	if len(contentType) != 0 {
		req.Header.Set("Content-Type", contentType[0])
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	v := new(Response)

	err = json.NewDecoder(resp.Body).Decode(v)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.Join(ErrBadRequest, ErrEOF)
		}
		return nil, err
	}

	if !v.OK {
		var retryErr *RetryError

		err = genError(v.ErrorCode, resp.Status, v.Description, v.Parameters)

		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFoundBanned) {
			c.Stop()
		}

		if errors.As(err, &retryErr) {
			c.rateLimiter.SetLimitAt(
				time.Now().Add(retryErr.RetryAfter),
				c.rateLimiter.Limit(),
			)
		}

		return nil, err
	}

	return v.Result, nil
}

// Start starts the client and listens for updates.
func (c *Client) Start(ctx context.Context, params *GetUpdatesParams) error {
	c.locker.Lock()

	if c.started {
		c.locker.Unlock()
		return ErrAlreadyStarted
	}

	c.updates = make(chan *Update, defaultUpdates)
	c.done = make(chan struct{})
	c.started = true

	done := c.done
	timeout := c.options.timeout

	c.locker.Unlock()

	localParams := GetUpdatesParams{
		Limit:   defaultUpdates,
		Timeout: int64(timeout.Seconds()),
	}
	if params != nil {
		localParams = *params
		if localParams.Limit == 0 {
			localParams.Limit = defaultUpdates
		}
	}

	workerCount := min(max(runtime.GOMAXPROCS(0), 1), int(localParams.Limit))

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.Stop()

			case <-done:
				close(c.updates)
				return

			default:
				updates, err := c.GetUpdates(&localParams)
				if err != nil {
					c.locker.RLock()
					handlerErr := c.Router().handlerErr
					c.locker.RUnlock()

					if handlerErr != nil {
						gogramCtx := c.acquireContext(ctx, nil)
						handlerErr(gogramCtx, err)
						c.releaseContext(gogramCtx)
					}

					continue
				}

				for i := range updates {
					c.updates <- &updates[i]
					localParams.Offset = updates[i].UpdateID + 1
				}
			}
		}
	}()

	wg := sync.WaitGroup{}

	wg.Add(workerCount)

	for range workerCount {
		go func() {
			c.processUpdates(ctx)
			wg.Done()
		}()
	}

	wg.Wait()

	c.locker.Lock()

	c.updates = nil
	c.done = nil
	c.started = false

	c.locker.Unlock()

	return ctx.Err()
}

// Stop stops the client.
func (c *Client) Stop() {
	c.locker.Lock()

	if c.done != nil {
		close(c.done)
		c.done = nil
	}

	c.locker.Unlock()
}

func (c *Client) processUpdates(ctx context.Context) {
	for u := range c.updates {
		c.processUpdate(ctx, u)
	}
}

func (c *Client) processUpdate(ctx context.Context, u *Update) {
	gogramCtx := c.acquireContext(ctx, u)
	defer c.releaseContext(gogramCtx)

	c.Router().Process(gogramCtx)
}
