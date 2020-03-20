import ExecSocketXtermAdapter from 'nomad-ui/utils/classes/exec-socket-xterm-adapter';
import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { Terminal } from 'xterm-vendor';

module('Integration | Utility | exec-socket-xterm-adapter', function(hooks) {
  setupRenderingTest(hooks);

  test('resizing the window passes a resize message through the socket', async function(assert) {
    let done = assert.async();

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      {{exec-terminal terminal=terminal}}
    `);

    await settled();

    let mockSocket = new Object({
      send(message) {
        assert.deepEqual(
          message,
          JSON.stringify({ tty_size: { width: terminal.cols, height: terminal.rows } })
        );
        done();
      },
    });

    new ExecSocketXtermAdapter(terminal, mockSocket);

    window.dispatchEvent(new Event('resize'));

    await settled();
  });

  test('stdout frames without data are ignored', async function(assert) {
    assert.expect(0);

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      {{exec-terminal terminal=terminal}}
    `);

    await settled();

    let mockSocket = new Object({
      send() {},
    });

    new ExecSocketXtermAdapter(terminal, mockSocket);

    mockSocket.onmessage({
      data: '{"stdout":{"exited":"true"}}',
    });

    await settled();
  });

  test('stderr frames are ignored', async function(assert) {
    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      {{exec-terminal terminal=terminal}}
    `);

    await settled();

    let mockSocket = new Object({
      send() {},
    });

    new ExecSocketXtermAdapter(terminal, mockSocket);

    mockSocket.onmessage({
      data: '{"stdout":{"data":"c2gtMy4yIPCfpbMk"}}',
    });

    mockSocket.onmessage({
      data: '{"stderr":{"data":"c2gtMy4yIPCfpbMk"}}',
    });

    await settled();

    assert.equal(
      terminal.buffer
        .getLine(0)
        .translateToString()
        .trim(),
      'sh-3.2 🥳$'
    );
  });
});
