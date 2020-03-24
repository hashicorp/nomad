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
});
