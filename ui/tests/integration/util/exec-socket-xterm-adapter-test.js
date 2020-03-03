import ExecSocketXtermAdapter from 'nomad-ui/utils/classes/exec-socket-xterm-adapter';
import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { find, render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { Terminal } from 'xterm';

module('Integration | Utility | exec-socket-xterm-adapter', function(hooks) {
  setupRenderingTest(hooks);

  test('resizing the window passes a resize message through the socket', async function(assert) {
    let done = assert.async();

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      {{exec-terminal terminal=terminal}}
    `);

    terminal.write('/bin/long-command');

    await settled();

    let mockSocket = new Object({
      send(message) {
        assert.deepEqual(message, JSON.stringify({ tty_size: { width: 138, height: 24 } }));
        assert.equal(terminal.cols, 138);
        done();
      },
    });

    new ExecSocketXtermAdapter(terminal, mockSocket);

    let terminalElement = find('.terminal');
    terminalElement.style.width = '50%';
    // FIXME height doesn’t work yet
    window.dispatchEvent(new Event('resize'));

    await settled();
  });
});
