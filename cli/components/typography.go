package components

import (
  "context"

  "github.com/mitchellh/go-glint"
)

type TextComponent struct {
  text string
  bold bool
  underlined bool
  color string
}

func (c *TextComponent) Bold() *TextComponent {
  c.bold = true
  return c
}

func (c *TextComponent) Underline() *TextComponent {
  c.underlined = true
  return c
}

func (c *TextComponent) Body(context.Context) glint.Component {
  var styles []glint.StyleOption

  if (c.color != "") {
    styles = append(styles, glint.Color(c.color))
  }
  if (c.bold) {
    styles = append(styles, glint.Bold())
  }
  if (c.underlined) {
    styles = append(styles, glint.Underline())
  }

  return glint.Style(glint.Text(c.text), styles...)
}

func Text(text string) *TextComponent {
  return &TextComponent{text: text}
}

func Subtle(text string) *TextComponent {
  return &TextComponent{text: text, color: "darkGray"}
}

func Success(text string) *TextComponent {
  return &TextComponent{text: text, color: "green"}
}

func Info(text string) *TextComponent {
  return &TextComponent{text: text, color: "blue"}
}

func Error(text string) *TextComponent {
  return &TextComponent{text: text, color: "red"}
}

func Warning(text string) *TextComponent {
  return &TextComponent{text: text, color: "yellow"}
}

func LineSpacing() glint.Component {
  return glint.Layout(glint.Text("")).Row()
}
