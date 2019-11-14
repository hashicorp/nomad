import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, triggerKeyEvent } from '@ember/test-helpers';
import { next } from '@ember/runloop';
import hbs from 'htmlbars-inline-precompile';
import sinon from 'sinon';

module('Integration | Component | exec-terminal', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders an incoming message', async function(assert) {
    const done = assert.async();

    const socket = {
      mockMessage(message) {
        if (this.onmessage) {
          this.onmessage({ data: JSON.stringify(message) });
        }
      },
    };
    this.set('socket', socket);

    await render(hbs`{{exec-terminal socket=socket}}`);

    window.xterm.onRender(() => {
      assert.ok(
        window.xterm.buffer
          .getLine(0)
          .translateToString()
          .includes('exec 🥳')
      );
      done();
    });

    socket.mockMessage({
      stdout: {
        data: 'ZXhlYyDwn6Wz', // FIXME?
      },
    });
  });

  skip('it routes encoded input to the socket', async function(assert) {
    const done = assert.async();

    const socket = {
      send: sinon.spy(),
    };
    this.set('socket', socket);

    await render(hbs`{{exec-terminal socket=socket}}`);

    await triggerKeyEvent('textarea', 'keydown', '!');

    // FIXME can’t figure out how to trigger a send
    // BUT is this even a sensible layer to test at?
    next(() => {
      assert.ok(socket.send.calledOnce);
      assert.ok(socket.send.calledWith({}));
      done();
    });
  });
});
