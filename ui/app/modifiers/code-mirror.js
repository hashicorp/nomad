/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import { bind } from '@ember/runloop';
import codemirror from 'codemirror';
import Modifier from 'ember-modifier';

import 'codemirror/addon/edit/matchbrackets';
import 'codemirror/addon/selection/active-line';
import 'codemirror/mode/javascript/javascript';
import 'codemirror/mode/ruby/ruby';

export default class CodeMirrorModifier extends Modifier {
  get autofocus() {
    if (Object.hasOwn({ ...this.args.named }, 'autofocus')) {
      // spread (...) because proxy, and because Ember over-eagerly prevents named prop lookups for modifier args.
      return this.args.named.autofocus;
    } else {
      return !this.args.named.readOnly;
    }
  }

  element = null;
  args = {};

  modify(element, positional, named) {
    if (!this.element) {
      this.element = element;
      this.args = { positional, named };
      this._setup();
    }
  }

  didUpdateArguments() {
    this._editor.setOption('lineWrapping', this.args.named.lineWrapping);
    this._editor.setOption('readOnly', this.args.named.readOnly);
    if (!this.args.named.content) {
      return;
    }
    if (this._editor.getValue() !== this.args.named.content) {
      this._editor.setValue(this.args.named.content);
    }
  }

  @action
  _onChange(editor) {
    this.args.named.onUpdate(
      editor.getValue(),
      this._editor,
      this.args.named.type
    );
  }

  _setup() {
    if (this.element) {
      const editor = codemirror(this.element, {
        gutters: this.args.named.gutters || [],
        matchBrackets: true,
        showCursorWhenSelecting: true,
        styleActiveLine: true,
        tabSize: 2,
        // all values we can pass into the modifier
        extraKeys: this.args.named.extraKeys || '',
        lineNumbers: this.args.named.lineNumbers || true,
        mode: this.args.named.mode || 'application/json',
        readOnly: this.args.named.readOnly || false,
        theme: this.args.named.theme || 'hashi',
        value: this.args.named.content || '',
        viewportMargin: this.args.named.viewportMargin || '',
        screenReaderLabel: this.args.named.screenReaderLabel || '',
        lineWrapping: this.args.named.lineWrapping || false,
      });

      if (this.autofocus) {
        editor.focus();
      }

      editor.on('change', bind(this, this._onChange));

      this._editor = editor;
    }
  }
}
