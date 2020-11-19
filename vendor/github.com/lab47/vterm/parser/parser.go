package parser

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type EventHandler interface {
	HandleEvent(Event) error
}

type Parser struct {
	Debug bool

	br      *bufio.Reader
	plain   bytes.Buffer
	handler EventHandler

	r        io.Reader
	newData  chan []byte
	injected chan Event

	curData []byte
	pos     int
}

func NewParser(r io.Reader, h EventHandler) (*Parser, error) {
	// br := bufio.NewReader(r)

	parser := &Parser{
		r: r,
		// br:      br,
		handler:  h,
		newData:  make(chan []byte, 3),
		injected: make(chan Event),
	}

	return parser, nil
}

type Event interface{}

type ResizeEvent struct {
	Rows, Cols int
	Confirm    chan error
}

func (p *Parser) Resize(ctx context.Context, rows, cols int) error {
	c := make(chan error)
	p.injected <- ResizeEvent{
		Rows:    rows,
		Cols:    cols,
		Confirm: c,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-c:
		return err
	}
}

const (
	NUL = 0x0
	DEL = 0x7f
	CAN = 0x18
	SUB = 0x1a
	ESC = 0x1b
	BEL = 0x7
	C0  = 0x20
)

func (p *Parser) Drive(ctx context.Context) error {
	go p.readInput(ctx)

	for {
		b, err := p.readByte(ctx)
		if err != nil {
			return err
		}

		switch b {
		case NUL, DEL:
			continue
		case CAN, SUB:
			continue
		case ESC:
			err := p.readEsc(ctx)
			if err != nil {
				return err
			}
			continue
		default:
			if b < C0 {
				err := p.readControl(b)
				if err != nil {
					return err
				}

				continue
			}
		}

		err = p.unreadByte()
		if err != nil {
			return err
		}

	normal:
		for {
			b, err := p.readByte(ctx)
			if err != nil {
				if p.plain.Len() > 0 {
					p.readSpan()
				}

				return err
			}

			switch b {
			case NUL, DEL, CAN, SUB:
				continue normal
			case ESC:
				err = p.unreadByte()
				if err != nil {
					return err
				}

				err := p.readSpan()
				if err != nil {
					return err
				}

				break normal
			default:
				if b < C0 {
					err = p.unreadByte()
					if err != nil {
						return err
					}

					err := p.readSpan()
					if err != nil {
						return err
					}

					break normal
				}
			}

			err = p.unreadByte()
			if err != nil {
				return err
			}

			r, _, err := p.readRune(ctx)
			if err != nil {
				return err
			}

			_, err = p.plain.WriteRune(r)
			if err != nil {
				return err
			}

			// optimization because it's really common to get here and have just one single character
			// in the buffer we need to emit immediately.
			if p.buffered() == 0 {
				p.readSpan()
			}
		}
	}
}

var textPool sync.Pool

func init() {
	textPool.New = func() interface{} {
		return &TextEvent{
			Text: make([]byte, 0, 128),
		}
	}
}

type TextEvent struct {
	Text []byte
}

func (ev *TextEvent) Recycle() {
	textPool.Put(ev)
}

func (p *Parser) readSpan() error {
	if p.plain.Len() == 0 {
		return nil
	}

	ev := textPool.Get().(*TextEvent)

	var buf []byte

	if cap(ev.Text) > p.plain.Len() {
		buf = ev.Text[:p.plain.Len()]
	} else {
		buf = make([]byte, p.plain.Len())
	}

	ev.Text = buf

	_, err := p.plain.Read(buf)
	if err != nil {
		return err
	}

	return p.handler.HandleEvent(ev)
}

type ControlEvent byte

func (c ControlEvent) String() string {
	return fmt.Sprintf("CTL: %#v (0x%x)", string(c), byte(c))
}

func (p *Parser) readControl(b byte) error {
	return p.handler.HandleEvent(ControlEvent(b))
}

func isIntermed(b byte) bool {
	return b >= 0x20 && b <= 0x2f
}

type EscapeEvent struct {
	Data []byte
}

func (p *Parser) readEsc(ctx context.Context) error {
	var intermed []byte
top:
	for {
		b, err := p.readByte(ctx)
		if err != nil {
			return err
		}

		switch b {
		case NUL, DEL:
			continue
		case ESC:
			intermed = nil
			continue
		case CAN, SUB:
			return nil
		default:
			if b < C0 {
				p.readControl(b)
				continue top
			}
		}

		switch b {
		case 0x50: // DCS
			return p.readString(ctx, "DCS")
		case 0x5b: // CSI
			return p.readCSI(ctx)
		case 0x5d: // OSC
			return p.readString(ctx, "OSC")
		default:
			if isIntermed(b) {
				intermed = append(intermed, b)
			} else if len(intermed) == 0 && b >= 0x40 && b < 0x60 {
				return p.readControl(b + 0x40)
			} else if b >= 0x30 && b < 0x7f {
				intermed = append(intermed, b)
				return p.handler.HandleEvent(&EscapeEvent{intermed})
			} else {
				L.Debug("Unhandled byte in escape", "byte", b)
			}
		}
	}
}

type OSCEvent struct {
	Command int
	Data    string
}

type StringEvent struct {
	Kind string
	Data []byte
}

func (p *Parser) emitStringEvent(kind string, data []byte) error {
	if kind == "OSC" {
		str := string(data)
		if sc := strings.IndexByte(str, ';'); sc != -1 {
			if cmd, err := strconv.Atoi(str[:sc]); err == nil {
				return p.handler.HandleEvent(&OSCEvent{
					Command: cmd,
					Data:    str[sc+1:],
				})
			}
		}
	}

	return p.handler.HandleEvent(&StringEvent{
		Kind: kind,
		Data: data,
	})
}

func (p *Parser) readString(ctx context.Context, kind string) error {
	var data []byte

top:
	for {
		b, err := p.readByte(ctx)
		if err != nil {
			return err
		}

		switch b {
		case NUL, DEL:
			continue top
		case CAN, SUB:
			return nil
		case ESC:
			b, err := p.readByte(ctx)
			if err != nil {
				return err
			}

			if b == 0x5c {
				return p.emitStringEvent(kind, data)
			}

			err = p.unreadByte()
			if err != nil {
				return err
			}

			return p.readEsc(ctx)
		default:
			switch {
			case b == 0x7:
				return p.emitStringEvent(kind, data)
			case b < C0:
				p.readControl(b)
				continue top
			default:
				data = append(data, b)
			}
		}
	}
}

var csiEvents sync.Pool

func init() {
	csiEvents.New = func() interface{} {
		return &CSIEvent{
			Args:     make([]int, 5),
			Leader:   make([]byte, 2),
			Intermed: make([]byte, 2),
		}
	}
}

type CSIEvent struct {
	Command  byte
	Leader   []byte
	Args     []int
	Intermed []byte
}

func (c *CSIEvent) CSICommand() CSICommand {
	idx := CSICommand(c.Command)
	if len(c.Leader) == 1 {
		idx = LEADER(c.Leader[0], c.Command)
	}

	if len(c.Intermed) == 1 {
		idx = INTERMED(c.Intermed[0], c.Command)
	}

	return idx
}

func (c *CSIEvent) Recycle() {
	csiEvents.Put(c)
}

func (c *CSIEvent) String() string {
	cmd := c.CSICommand()

	return fmt.Sprintf("CSI: %s (0x%x) Leader=%#v Args=%#v Intermed=%#v", cmd.String(), c.Command, c.Leader, c.Args, c.Intermed)
}

func (p *Parser) readCSI(ctx context.Context) error {
	const (
		LEADER   = 1
		ARG      = 2
		INTERMED = 3
	)

	ev := csiEvents.Get().(*CSIEvent)

	var (
		leader   []byte = ev.Leader[:0]
		state    int    = LEADER
		arg      int    = -1
		args     []int  = ev.Args[:0]
		intermed []byte = ev.Intermed[:0]
	)

top:
	for {
		b, err := p.readByte(ctx)
		if err != nil {
			ev.Command = b
			ev.Leader = leader
			ev.Args = args
			ev.Intermed = intermed

			p.handler.HandleEvent(ev)
			return err
		}

		switch b {
		case NUL, DEL:
			continue
		case CAN, SUB:
			return nil
		case ESC:
			return p.readEsc(ctx)
		default:
			if b < C0 {
				p.readControl(b)
				continue top
			}
		}

		switch state {
		case LEADER:
			if b >= 0x3c && b <= 0x3f {
				leader = append(leader, b)
				continue
			} else {
				state = ARG
			}

			fallthrough
		case ARG:
			if b >= '0' && b <= '9' {
				if arg == -1 {
					arg = 0
				}

				arg *= 10
				arg += int(b - '0')
				continue top
			}

			if b == ':' {
				b = ';'
			}

			if b == ';' {
				args = append(args, arg)
				arg = -1
				continue top
			}

			if arg != -1 {
				args = append(args, arg)
			}

			state = INTERMED
			fallthrough
		case INTERMED:
			switch {
			case isIntermed(b):
				intermed = append(intermed, b)
				continue top
			case b == ESC:
				return nil
			case b >= 0x40 && b <= 0x7e:
				ev.Command = b
				ev.Leader = leader
				ev.Args = args
				ev.Intermed = intermed

				return p.handler.HandleEvent(ev)
			}

			// Invalid in CSI, cancel it.
			return nil
		}
	}
}
