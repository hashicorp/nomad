import ExecCommandEditorXtermAdapter from 'nomad-ui/utils/classes/exec-command-editor-xterm-adapter';
import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { Terminal } from 'xterm-vendor';

module('Integration | Utility | exec-command-editor-xterm-adapter', function(hooks) {
  setupRenderingTest(hooks);

  test('it can wrap to a previous line while backspacing', async function(assert) {
    let done = assert.async();

    await render(hbs`
      <div id='terminal'></div>
    `);

    let terminal = new Terminal({ cols: 10 });
    terminal.open(document.getElementById('terminal'));

    terminal.write('/bin/long-command');

    new ExecCommandEditorXtermAdapter(
      terminal,
      command => {
        assert.equal(command, '/bin/long');
        done();
      },
      '/bin/long-command'
    );

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });

    await settled();

    assert.equal(
      terminal.buffer
        .getLine(0)
        .translateToString()
        .trim(),
      '/bin/long'
    );

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Enter' } });
  });

  test('it supports typing ^U to delete the entire command', async function(assert) {
    let done = assert.async();

    await render(hbs`
      <div id='terminal'></div>
    `);

    let terminal = new Terminal({ cols: 10 });
    terminal.open(document.getElementById('terminal'));

    terminal.write('to-delete');

    new ExecCommandEditorXtermAdapter(
      terminal,
      command => {
        assert.equal(command, '!');
        done();
      },
      'to-delete'
    );

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'u', ctrlKey: true } });

    await settled();

    assert.equal(
      terminal.buffer
        .getLine(0)
        .translateToString()
        .trim(),
      ''
    );

    await terminal.simulateCommandKeyEvent({ key: '!', domEvent: {} });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Enter' } });
  });

  test('it supports left and right arrow keys for moving around within the command', async function(assert) {
    let done = assert.async();

    await render(hbs`
      <div id='terminal'></div>
    `);

    let terminal = new Terminal({ cols: 72 });
    terminal.open(document.getElementById('terminal'));

    terminal.write('command: ');
    terminal.write('to-edit');

    new ExecCommandEditorXtermAdapter(
      terminal,
      command => {
        assert.equal(command, 'did-edit!');
        done();
      },
      'to-edit'
    );

    await settled();

    assert.equal(terminal.buffer.cursorX, 16);

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });

    await settled();

    assert.equal(terminal.buffer.cursorX, 11);

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });

    await terminal.simulateCommandKeyEvent({ key: 'i', domEvent: {} });
    await terminal.simulateCommandKeyEvent({ key: 'd', domEvent: {} });

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });

    // Try to move beyond the beginning of the command
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowLeft' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Backspace' } });

    await settled();

    assert.equal(terminal.buffer.cursorX, 9);

    await terminal.simulateCommandKeyEvent({ key: 'd', domEvent: {} });

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });

    // Try to move beyond the end of the command
    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'ArrowRight' } });

    await settled();

    assert.equal(terminal.buffer.cursorX, 17);

    await terminal.simulateCommandKeyEvent({ key: '!', domEvent: {} });

    await settled();

    assert.equal(
      terminal.buffer
        .getLine(0)
        .translateToString()
        .trim(),
      'command: did-edit!'
    );

    await terminal.simulateCommandKeyEvent({ domEvent: { key: 'Enter' } });
  });
});
