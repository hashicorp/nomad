package state

import (
	"fmt"

	"github.com/lab47/vterm/parser"
)

func (s *State) handleString(ev *parser.StringEvent) error {
	return s.output.StringEvent(ev.Kind, ev.Data)
}

func (s *State) handleOSC(ev *parser.OSCEvent) error {
	switch ev.Command {
	case 0:
		err := s.output.SetTermProp(TermAttrTitle, ev.Data)
		if err != nil {
			return err
		}

		return s.output.SetTermProp(TermAttrIconName, ev.Data)
	case 1:
		return s.output.SetTermProp(TermAttrIconName, ev.Data)
	case 2:
		return s.output.SetTermProp(TermAttrTitle, ev.Data)
	default:
		return s.output.SetTermProp(TermAttrOSC, fmt.Sprintf("%d;%s", ev.Command, ev.Data))
	}

	return nil
}
