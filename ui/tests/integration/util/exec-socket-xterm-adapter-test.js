/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/no-conditional-assertions */
import ExecSocketXtermAdapter from 'nomad-ui/utils/classes/exec-socket-xterm-adapter';
import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { Terminal } from 'xterm';
import { HEARTBEAT_INTERVAL } from 'nomad-ui/utils/classes/exec-socket-xterm-adapter';
import sinon from 'sinon';

module('Integration | Utility | exec-socket-xterm-adapter', function (hooks) {
  setupRenderingTest(hooks);

  test('initiating socket sends authentication handshake', async function (assert) {
    assert.expect(1);

    let done = assert.async();

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      <ExecTerminal @terminal={{terminal}} />
    `);

    let firstMessage = true;
    let mockSocket = new Object({
      send(message) {
        if (firstMessage) {
          firstMessage = false;
          assert.deepEqual(
            message,
            JSON.stringify({ version: 1, auth_token: 'mysecrettoken' })
          );
          mockSocket.onclose();
          done();
        }
      },
    });

    new ExecSocketXtermAdapter(terminal, mockSocket, 'mysecrettoken');

    mockSocket.onopen();

    await settled();
  });

  test('initiating socket sends authentication handshake even if unauthenticated', async function (assert) {
    assert.expect(1);

    let done = assert.async();

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      <ExecTerminal @terminal={{terminal}} />
    `);

    let firstMessage = true;
    let mockSocket = new Object({
      send(message) {
        if (firstMessage) {
          firstMessage = false;
          assert.deepEqual(
            message,
            JSON.stringify({ version: 1, auth_token: '' })
          );
          mockSocket.onclose();
          done();
        }
      },
    });

    new ExecSocketXtermAdapter(terminal, mockSocket, null);

    mockSocket.onopen();

    await settled();
  });

  test('a heartbeat is sent periodically', async function (assert) {
    assert.expect(1);

    let done = assert.async();

    const clock = sinon.useFakeTimers({
      now: new Date(),
      shouldAdvanceTime: true,
    });

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      <ExecTerminal @terminal={{terminal}} />
    `);

    let mockSocket = new Object({
      send(message) {
        if (!message.includes('version') && !message.includes('tty_size')) {
          assert.deepEqual(message, JSON.stringify({}));
          clock.restore();
          mockSocket.onclose();
          done();
        }
      },
    });

    new ExecSocketXtermAdapter(terminal, mockSocket, null);
    mockSocket.onopen();
    await settled();
    clock.tick(HEARTBEAT_INTERVAL);
  });

  test('resizing the window passes a resize message through the socket', async function (assert) {
    assert.expect(1);

    let done = assert.async();

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      <ExecTerminal @terminal={{terminal}} />
    `);

    let mockSocket = new Object({
      send(message) {
        assert.deepEqual(
          message,
          JSON.stringify({
            tty_size: { width: terminal.cols, height: terminal.rows },
          })
        );
        mockSocket.onclose();
        done();
      },
    });

    new ExecSocketXtermAdapter(terminal, mockSocket, '');

    window.dispatchEvent(new Event('resize'));

    await settled();
  });

  test('stdout frames without data are ignored', async function (assert) {
    assert.expect(0);

    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      <ExecTerminal @terminal={{terminal}} />
    `);

    let mockSocket = new Object({
      send() {},
    });

    new ExecSocketXtermAdapter(terminal, mockSocket, '');

    mockSocket.onmessage({
      data: '{"stdout":{"exited":"true"}}',
    });

    await settled();
    mockSocket.onclose();
  });

  test('stderr frames are ignored', async function (assert) {
    let terminal = new Terminal();
    this.set('terminal', terminal);

    await render(hbs`
      <ExecTerminal @terminal={{terminal}} />
    `);

    let mockSocket = new Object({
      send() {},
    });

    new ExecSocketXtermAdapter(terminal, mockSocket, '');

    mockSocket.onmessage({
      data: '{"stdout":{"data":"c2gtMy4yIPCfpbMk"}}',
    });

    mockSocket.onmessage({
      data: '{"stderr":{"data":"c2gtMy4yIPCfpbMk"}}',
    });

    await settled();

    assert.equal(
      terminal.buffer.active.getLine(0).translateToString().trim(),
      'sh-3.2 🥳$'
    );

    mockSocket.onclose();
  });
});
