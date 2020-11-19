package parser

import (
	"context"
	"io"
	"time"
	"unicode/utf8"
)

func (p *Parser) readInput(ctx context.Context) {
	for {
		buf := make([]byte, 128)
		n, err := p.r.Read(buf)
		if err != nil {
			close(p.newData)
			return
		}

		select {
		case <-ctx.Done():
			return
		case p.newData <- buf[:n]:
			// aok
		}
	}
}

func (p *Parser) waitDataOrTimeout(c <-chan time.Time) (bool, error) {
	select {
	case data := <-p.newData:
		if data == nil {
			return false, io.EOF
		}

		if len(p.curData) == p.pos {
			p.curData = data
		} else {
			p.curData = append(p.curData, data...)
		}

		return false, nil
	case <-c:
		return true, nil
	}
}

func (p *Parser) peekByte(ctx context.Context) (byte, error) {
	if len(p.curData) == p.pos {
	more:
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case ev := <-p.injected:
			err := p.handler.HandleEvent(ev)
			if err != nil {
				return 0, err
			}

			goto more
		case data := <-p.newData:
			if data == nil {
				return 0, io.EOF
			}

			p.curData = data
			p.pos = 0
		}
	}

	b := p.curData[p.pos]

	return b, nil
}

func (p *Parser) readByte(ctx context.Context) (byte, error) {
	b, err := p.peekByte(ctx)
	if err != nil {
		return 0, err
	}

	p.pos++

	return b, nil
}

func (p *Parser) buffered() int {
	return len(p.curData) - p.pos
}

func (p *Parser) unreadByte() error {
	if p.pos == 0 {
		return io.EOF
	}

	p.pos--

	return nil
}

func (p *Parser) readRune(ctx context.Context) (r rune, size int, err error) {
	_, err = p.peekByte(ctx)
	if err != nil {
		return 0, 0, err
	}

	r, size = rune(p.curData[p.pos]), 1
	if r < utf8.RuneSelf {
		p.pos++
		return r, size, nil
	}

	for !utf8.FullRune(p.curData[p.pos:]) {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		case data := <-p.newData:
			if data == nil {
				return 0, 0, io.EOF
			}

			p.curData = append(p.curData, data...)
		case ev := <-p.injected:
			err = p.handler.HandleEvent(ev)
			if err != nil {
				return 0, 0, err
			}
		}
	}

	r, size = utf8.DecodeRune(p.curData[p.pos:])

	p.pos += size
	return r, size, nil
}
