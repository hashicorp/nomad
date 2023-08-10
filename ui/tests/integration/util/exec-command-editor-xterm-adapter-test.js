/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ExecCommandEditorXtermAdapter from 'nomad-ui/utils/classes/exec-command-editor-xterm-adapter';
import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { Terminal } from 'xterm';
import KEYS from 'nomad-ui/utils/keys';

module(
  'Integration | Utility | exec-command-editor-xterm-adapter',
  function (hooks) {
    setupRenderingTest(hooks);

    test('it can wrap to a previous line while backspacing', async function (assert) {
      assert.expect(2);

      let done = assert.async();

      await render(hbs`
      <div id='terminal'></div>
    `);

      let terminal = new Terminal({ cols: 10 });
      terminal.open(document.getElementById('terminal'));

      terminal.write('/bin/long-command');

      new ExecCommandEditorXtermAdapter(
        terminal,
        (command) => {
          assert.equal(command, '/bin/long');
          done();
        },
        '/bin/long-command'
      );

      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);
      await terminal.simulateCommandDataEvent(KEYS.DELETE);

      await settled();

      assert.equal(
        terminal.buffer.active.getLine(0).translateToString().trim(),
        '/bin/long'
      );

      await terminal.simulateCommandDataEvent(KEYS.ENTER);
    });

    test('it ignores arrow keys and unprintable characters other than ^U', async function (assert) {
      assert.expect(4);

      let done = assert.async();

      await render(hbs`
      <div id='terminal'></div>
    `);

      let terminal = new Terminal({ cols: 72 });
      terminal.open(document.getElementById('terminal'));

      terminal.write('/bin/bash');

      new ExecCommandEditorXtermAdapter(
        terminal,
        (command) => {
          assert.equal(command, '/bin/bash!');
          done();
        },
        '/bin/bash'
      );

      await terminal.simulateCommandDataEvent(KEYS.RIGHT_ARROW);
      await terminal.simulateCommandDataEvent(KEYS.RIGHT_ARROW);
      await terminal.simulateCommandDataEvent(KEYS.LEFT_ARROW);
      await terminal.simulateCommandDataEvent(KEYS.UP_ARROW);
      await terminal.simulateCommandDataEvent(KEYS.UP_ARROW);
      await terminal.simulateCommandDataEvent(KEYS.DOWN_ARROW);
      await terminal.simulateCommandDataEvent(KEYS.CONTROL_A);
      await terminal.simulateCommandDataEvent('!');

      await settled();

      assert.equal(terminal.buffer.active.cursorY, 0);
      assert.equal(terminal.buffer.active.cursorX, 10);

      assert.equal(
        terminal.buffer.active.getLine(0).translateToString().trim(),
        '/bin/bash!'
      );

      await terminal.simulateCommandDataEvent(KEYS.ENTER);
    });

    test('it supports typing ^U to delete the entire command', async function (assert) {
      assert.expect(2);

      let done = assert.async();

      await render(hbs`
      <div id='terminal'></div>
    `);

      let terminal = new Terminal({ cols: 10 });
      terminal.open(document.getElementById('terminal'));

      terminal.write('to-delete');

      new ExecCommandEditorXtermAdapter(
        terminal,
        (command) => {
          assert.equal(command, '!');
          done();
        },
        'to-delete'
      );

      await terminal.simulateCommandDataEvent(KEYS.CONTROL_U);

      await settled();

      assert.equal(
        terminal.buffer.active.getLine(0).translateToString().trim(),
        ''
      );

      await terminal.simulateCommandDataEvent('!');
      await terminal.simulateCommandDataEvent(KEYS.ENTER);
    });
  }
);
