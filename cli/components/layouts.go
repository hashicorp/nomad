package components

import (
  "context"

  "github.com/mitchellh/go-glint"
)

func WatchEvent(isRunning bool, message glint.Component, yield []glint.Component) *WatchEventComponent {
  return &WatchEventComponent{isRunning: isRunning, message: message, yield: yield}
}

func LargeEvent(message glint.Component, yield []glint.Component) *LargeEventComponent {
  return &LargeEventComponent{message: message, yield: yield}
}

func SmallEvent(event glint.Component, line ...glint.Component) glint.Component {
  components := append([]glint.Component{event}, line...)
  return glint.Layout(
    components...,
  ).Row()
}

type WatchEventComponent struct {
  isRunning bool
  message glint.Component
  yield []glint.Component
}

func (c *WatchEventComponent) Body(context.Context) glint.Component {
  genRunning := func() glint.Component {
    return IconRunning()
  }

  return glint.Layout(
    LineSpacing(),
    glint.Layout(
      Maybe(genRunning, c.isRunning),
      c.message,
    ).Row(),
    LineSpacing(),
    glint.Layout(c.yield...).MarginLeft(4),
    LineSpacing(),
  )
}

type LargeEventComponent struct {
  message glint.Component
  yield []glint.Component
}

func (c *LargeEventComponent) Body(context.Context) glint.Component {
  return glint.Layout(
    c.message,
    glint.Layout(c.yield...).MarginLeft(4),
    LineSpacing(),
  )
}

func Maybe(gen func() glint.Component, predicate bool) glint.Component {
  if predicate {
    return gen()
  }
  // Return a "noop" for convenience
  return glint.Text("")
}
