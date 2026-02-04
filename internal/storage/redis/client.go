package redis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	addr     string
	password string
	db       int

	pool chan net.Conn
	mu   sync.Mutex
}

type Config struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

func New(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = 10
	}

	c := &Client{
		addr:     cfg.Addr,
		password: cfg.Password,
		db:       cfg.DB,
		pool:     make(chan net.Conn, cfg.PoolSize),
	}

	// Validate connectivity.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for {
		select {
		case conn := <-c.pool:
			_ = conn.Close()
		default:
			return nil
		}
	}
}

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.do(ctx, "PING")
	if err != nil {
		return err
	}
	if resp.typ != respSimpleString || resp.str != "PONG" {
		return fmt.Errorf("unexpected PING response: %s", resp.String())
	}
	return nil
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	resp, err := c.do(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	if resp.typ != respInteger {
		return 0, fmt.Errorf("unexpected INCR response: %s", resp.String())
	}
	return resp.num, nil
}

func (c *Client) ExpireSeconds(ctx context.Context, key string, ttlSeconds int64) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}
	resp, err := c.do(ctx, "EXPIRE", key, strconv.FormatInt(ttlSeconds, 10))
	if err != nil {
		return err
	}
	if resp.typ != respInteger {
		return fmt.Errorf("unexpected EXPIRE response: %s", resp.String())
	}
	// 1 means TTL set, 0 means key does not exist. Either way, not fatal for rate limiting.
	return nil
}

func (c *Client) getConn(ctx context.Context) (net.Conn, *bufio.ReadWriter, func(error), error) {
	select {
	case conn := <-c.pool:
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		putBack := func(err error) {
			if err != nil {
				_ = conn.Close()
				return
			}
			select {
			case c.pool <- conn:
			default:
				_ = conn.Close()
			}
		}
		return conn, rw, putBack, nil
	default:
		// Create a new connection.
		d := net.Dialer{Timeout: 1 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", c.addr)
		if err != nil {
			return nil, nil, nil, err
		}

		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		if err := c.initConn(ctx, conn, rw); err != nil {
			_ = conn.Close()
			return nil, nil, nil, err
		}

		putBack := func(err error) {
			if err != nil {
				_ = conn.Close()
				return
			}
			select {
			case c.pool <- conn:
			default:
				_ = conn.Close()
			}
		}

		return conn, rw, putBack, nil
	}
}

func (c *Client) initConn(ctx context.Context, conn net.Conn, rw *bufio.ReadWriter) error {
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	}

	if c.password != "" {
		if err := writeArray(rw.Writer, "AUTH", c.password); err != nil {
			return err
		}
		if err := rw.Flush(); err != nil {
			return err
		}
		resp, err := readResp(rw.Reader)
		if err != nil {
			return err
		}
		if resp.typ == respError {
			return resp.err
		}
	}

	if c.db != 0 {
		if err := writeArray(rw.Writer, "SELECT", strconv.Itoa(c.db)); err != nil {
			return err
		}
		if err := rw.Flush(); err != nil {
			return err
		}
		resp, err := readResp(rw.Reader)
		if err != nil {
			return err
		}
		if resp.typ == respError {
			return resp.err
		}
	}

	return nil
}

func (c *Client) do(ctx context.Context, args ...string) (resp, error) {
	if len(args) == 0 {
		return resp{}, errors.New("redis: empty command")
	}

	conn, rw, putBack, err := c.getConn(ctx)
	if err != nil {
		return resp{}, err
	}

	var opErr error
	defer func() { putBack(opErr) }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	}

	if err := writeArray(rw.Writer, args...); err != nil {
		opErr = err
		return resp{}, err
	}
	if err := rw.Flush(); err != nil {
		opErr = err
		return resp{}, err
	}

	r, err := readResp(rw.Reader)
	if err != nil {
		opErr = err
		return resp{}, err
	}
	if r.typ == respError {
		opErr = r.err
		return resp{}, r.err
	}

	return r, nil
}

func writeArray(w *bufio.Writer, args ...string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return nil
}

type respType byte

const (
	respSimpleString respType = '+'
	respError        respType = '-'
	respInteger      respType = ':'
	respBulkString   respType = '$'
)

type resp struct {
	typ respType
	str string
	num int64
	err error
}

func (r resp) String() string {
	switch r.typ {
	case respSimpleString:
		return "+" + r.str
	case respInteger:
		return ":" + strconv.FormatInt(r.num, 10)
	case respBulkString:
		return "$" + r.str
	case respError:
		if r.err != nil {
			return "-" + r.err.Error()
		}
		return "-ERR"
	default:
		return "?"
	}
}

func readLine(rd *bufio.Reader) (string, error) {
	line, err := rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	// line includes \n; should end with \r\n
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", errors.New("redis: invalid line ending")
	}
	return line[:len(line)-2], nil
}

func readResp(rd *bufio.Reader) (resp, error) {
	b, err := rd.ReadByte()
	if err != nil {
		return resp{}, err
	}

	switch respType(b) {
	case respSimpleString:
		s, err := readLine(rd)
		if err != nil {
			return resp{}, err
		}
		return resp{typ: respSimpleString, str: s}, nil
	case respError:
		s, err := readLine(rd)
		if err != nil {
			return resp{}, err
		}
		return resp{typ: respError, err: errors.New(s)}, nil
	case respInteger:
		s, err := readLine(rd)
		if err != nil {
			return resp{}, err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return resp{}, err
		}
		return resp{typ: respInteger, num: n}, nil
	case respBulkString:
		s, err := readLine(rd)
		if err != nil {
			return resp{}, err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return resp{}, err
		}
		if n == -1 {
			return resp{typ: respBulkString, str: ""}, nil
		}
		buf := make([]byte, n+2) // includes \r\n
		if _, err := io.ReadFull(rd, buf); err != nil {
			return resp{}, err
		}
		if len(buf) < 2 || buf[len(buf)-2] != '\r' || buf[len(buf)-1] != '\n' {
			return resp{}, errors.New("redis: invalid bulk string ending")
		}
		return resp{typ: respBulkString, str: string(buf[:len(buf)-2])}, nil
	default:
		return resp{}, fmt.Errorf("redis: unsupported response type %q", b)
	}
}

