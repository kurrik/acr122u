package acr122u

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ebfe/scard"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var scardEstablishContext = scard.EstablishContext

// Context for ACR122U readers
type Context struct {
	context   scardContext
	readers   []string
	shareMode ShareMode
	protocol  Protocol
	logLevel  LogLevel
	logWriter io.Writer
}

// EstablishContext creates a ACR122U context
func EstablishContext(options ...Option) (*Context, error) {
	sctx, err := scardEstablishContext()
	if err != nil {
		return nil, err
	}

	return newContext(sctx, options...)
}

// Option is the function type used to configure the context
type Option func(*Context)

// WithShareMode accepts Exclusive (0x1) or Shared mode (0x2)
func WithShareMode(sm ShareMode) Option {
	return func(actx *Context) {
		actx.shareMode = sm
	}
}

// WithProtocol accepts Undefined (0x0), T0 (0x1), T1 (0x2) or Any (T0|T1)
func WithProtocol(p Protocol) Option {
	return func(actx *Context) {
		actx.protocol = p
	}
}

// Sets the logging level
func WithLogLevel(l LogLevel) Option {
	return func(actx *Context) {
		actx.logLevel = l
	}
}

// Sets the log writer
func WithLogWriter(w io.Writer) Option {
	return func(actx *Context) {
		actx.logWriter = w
	}
}

// Creates a context with the supplied options.  Processes options for logging.
func newContext(sctx scardContext, options ...Option) (*Context, error) {
	if _, err := sctx.IsValid(); err != nil {
		return nil, err
	}
	readers, err := sctx.ListReaders()
	if err != nil {
		return nil, err
	}
	if len(readers) == 0 {
		return nil, scard.ErrNoReadersAvailable
	}
	actx := &Context{
		context:   sctx,
		readers:   readers,
		shareMode: ShareShared,
		protocol:  ProtocolAny,
		logLevel:  LogDebug,
		logWriter: ConsoleLogger,
	}
	for _, option := range options {
		option(actx)
	}
	zerolog.SetGlobalLevel(zerolog.Level(actx.logLevel))
	log.Logger = log.Output(actx.logWriter)

	return actx, nil
}

// Release should be called when the context is not needed anymore
func (actx *Context) Release() error {
	return actx.context.Release()
}

// Readers returns a list of readers
func (actx *Context) Readers() []string {
	return actx.readers
}

// SetReaders updates the list of readers, e.g. to filter to a specific reader
func (actx *Context) SetReaders(r []string) {
	actx.readers = r
}

// ServeFunc uses the provided HandlerFunc as a Handler
func (actx *Context) ServeFunc(ctx context.Context, hf HandlerFunc) error {
	return actx.Serve(ctx, hf)
}

// Serve cards being swiped using the provided Handler
func (actx *Context) Serve(ctx context.Context, h Handler) error {
	var (
		logger = log.With().Str("Caller", "Serve").Logger()
	)
	// Channel for state reads
	stateChan := make(chan scard.ReaderState, 1)
	go actx.read(ctx, stateChan)

	for stateReceived := range stateChan {
		logger.Info().
			Str("Cur state", formatStateFlag(stateReceived.CurrentState)).
			Str("Evt state", formatStateFlag(stateReceived.EventState)).
			Str("User data", fmt.Sprintf("%v", stateReceived.UserData)).
			Msg("Signal received")

		if stateReceived.EventState&scard.StatePresent != 0 {
			switch v := stateReceived.UserData.(type) {
			case *card:
				logger.Debug().Str("UserData", fmt.Sprintf("%v", v)).Msg("Handling card")
				if v != nil {
					h.ServeCard(v)
				}
			default:
				logger.Error().Str("UserData", fmt.Sprintf("%v", v)).Msg("Unahandled card data type")
				return ErrUnhandledCardData
			}
		}
	}
	return nil
}

// Connects to the reader.  Needs to be called before waiting for state change.
func (actx *Context) connect(reader string) (*card, error) {
	sc, err := actx.context.Connect(reader,
		scard.ShareMode(actx.shareMode),
		scard.Protocol(actx.protocol),
	)
	if err != nil {
		return nil, err
	}
	return newCard(reader, sc), nil
}

// Disconnects from the reader.  Needs to be called when exiting.
func (actx *Context) disconnect(c *card) error {
	err := c.scard.Disconnect(scard.ResetCard)
	return err
}

// Initializes a reader structure which will be populated by waitForStatusChange.
func (actx *Context) initializeReaderState() []scard.ReaderState {
	rs := make([]scard.ReaderState, len(actx.readers))
	for i := range rs {
		rs[i].Reader = actx.readers[i]
		rs[i].CurrentState = scard.StateUnaware
	}
	return rs
}

// Blocks until the card state changes.  Meant to be called in a goroutine.
// - Will exit when `ctx“ is closed.
// - `rs` is an initialized reader state array.
// - `interruptDuration` configures how frequently the read will timeout and check for the channel close.
func (actx *Context) waitForStatusChange(ctx context.Context, rs []scard.ReaderState, interruptDuration time.Duration) error {
	var (
		logger = log.With().Str("Caller", "waitForStatusChange").Logger()
	)
	logger.Debug().Msg("Waiting for status to change")
	for {
		err := actx.context.GetStatusChange(rs, interruptDuration)
		select {
		case <-ctx.Done():
			return ErrShutdown
		default:
		}
		if err == nil {
			// Status has changed, signal by returning.
			logger.Debug().Msg("Got signal")
			return nil
		} else {
			err = wrapError("error waiting for status change", err)
			switch {
			case errors.Is(err, scard.ErrTimeout):
				logger.Trace().Err(err).Msg("Handled ErrTimeout")
			default:
				return err
			}
		}
	}
}

// Reads the data payload from the reader.  Meant to be called when the state changes to StatePresent.
func (actx *Context) readCardData(state scard.ReaderState) (*card, error) {
	var (
		logger = log.With().Str("Caller", "readCardData").Logger()
	)
	// Step 1: Connect
	logger.Debug().Msg("Connecting to reader")
	c, err := actx.connect(state.Reader)
	if err != nil {
		err2 := wrapError("readCardData connect error", err)
		switch {
		case errors.Is(err2, scard.ErrNoSmartcard):
			logger.Trace().Err(err2).Msg("Handled ErrNoSmartcard")
			return nil, nil
		case errors.Is(err2, scard.ErrUnpoweredCard):
			logger.Trace().Err(err2).Msg("Handled ErrUnpoweredCard")
			return nil, nil
		default:
			return nil, err2
		}
	}
	// Step 3 (defer): Disconnect when exiting
	defer func() {
		logger.Debug().Msg("Disconnecting")
		if err := actx.disconnect(c); err != nil {
			logger.Error().Err(err).Msg("Problem disconnecting")
		}
	}()
	// Step 2: Read payload
	logger.Debug().Msg("Reading payload")
	if c.uid, err = c.getUID(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, err
	}
	return c, err
}

func (actx *Context) read(ctx context.Context, results chan<- scard.ReaderState) {
	var (
		logger = log.With().Str("Caller", "read").Logger()
		rs     = actx.initializeReaderState()
		err    error
	)
	defer close(results)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		err = actx.waitForStatusChange(ctx, rs, time.Second)
		if err != nil {
			return
		}
		for i := range rs {
			if rs[i].EventState != rs[i].CurrentState {
				if rs[i].EventState&scard.StatePresent != 0 {
					logger.Debug().Msg("Card present")
					rs[i].UserData, err = actx.readCardData(rs[i])
					if err != nil {
						logger.Error().Err(err).Msg("Problem reading card data")
						return
					}
				}
				results <- rs[i]
				rs[i].CurrentState = rs[i].EventState
				rs[i].UserData = nil
			}
		}
	}
}
