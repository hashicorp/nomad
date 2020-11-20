package components

import (
  "time"
  "context"

  "github.com/mitchellh/go-glint"
  "github.com/hashicorp/nomad/cli/icons"
)

func IconWarning() glint.Component {
  return Warning(icons.Warning + " ")
}

func IconFailure() glint.Component {
  return Error(icons.Failure + " ")
}

func IconSuccess() glint.Component {
  return Success(icons.Success + " ")
}

func IconHealthy() glint.Component {
  return Success(icons.Healthy + " ")
}

// A stateless variation of the Spinner that comes with go-glint
// This still has to be a struct with a Body method or else it doesn't
// rerender
func IconRunning() *IconRunningComponent {
  return &IconRunningComponent{}
}

type IconRunningComponent struct {
}

func (c *IconRunningComponent) Body(context.Context) glint.Component {
  frames := []rune(`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`)
  length := len(frames)
  currentFrame := (time.Now().UnixNano() / int64(time.Millisecond) / 150) % int64(length)

  return Info(string(frames[currentFrame]) + " ")
}
