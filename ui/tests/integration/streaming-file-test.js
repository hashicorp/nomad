import { run } from '@ember/runloop';
import { find, settled } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import Pretender from 'pretender';
import { logEncode } from '../../mirage/data/logs';
import fetch from 'nomad-ui/utils/fetch';
import Log from 'nomad-ui/utils/classes/log';

const { assign } = Object;

const stringifyValues = obj =>
  Object.keys(obj).reduce((newObj, key) => {
    newObj[key] = obj[key].toString();
    return newObj;
  }, {});

const makeLogger = (url, params) =>
  Log.create({
    url,
    params,
    plainText: true,
    logFetch: url => fetch(url).then(res => res),
  });

module('Integration | Component | streaming file', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    this.server = new Pretender(function() {
      this.get('/file/endpoint', () => [200, {}, 'Hello World']);
      this.get('/file/stream', () => [200, {}, logEncode(['Hello World'], 0)]);
    });
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  const commonTemplate = hbs`
    {{streaming-file logger=logger mode=mode isStreaming=isStreaming}}
  `;

  test('when mode is `head`, the logger signals head', async function(assert) {
    const url = '/file/endpoint';
    const params = { path: 'hello/world.txt', offset: 0, limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'head',
      isStreaming: false,
    });

    await this.render(commonTemplate);
    await settled();

    const request = this.server.handledRequests[0];
    assert.equal(this.server.handledRequests.length, 1, 'One request made');
    assert.equal(request.url.split('?')[0], url, `URL is ${url}`);
    assert.deepEqual(
      request.queryParams,
      stringifyValues(assign({ origin: 'start' }, params)),
      'Query params are correct'
    );
    assert.equal(find('[data-test-output]').textContent, 'Hello World');
  });

  test('when mode is `tail`, the logger signals tail', async function(assert) {
    const url = '/file/endpoint';
    const params = { path: 'hello/world.txt', limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'tail',
      isStreaming: false,
    });

    await this.render(commonTemplate);
    await settled();

    const request = this.server.handledRequests[0];
    assert.equal(this.server.handledRequests.length, 1, 'One request made');
    assert.equal(request.url.split('?')[0], url, `URL is ${url}`);
    assert.deepEqual(
      request.queryParams,
      stringifyValues(assign({ origin: 'end', offset: 50000 }, params)),
      'Query params are correct'
    );
    assert.equal(find('[data-test-output]').textContent, 'Hello World');
  });

  test('when mode is `streaming` and `isStreaming` is true, streaming starts', async function(assert) {
    const url = '/file/stream';
    const params = { path: 'hello/world.txt', limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'streaming',
      isStreaming: true,
    });

    assert.ok(true);

    run.later(run, run.cancelTimers, 500);

    await this.render(commonTemplate);
    await settled();

    const request = this.server.handledRequests[0];
    assert.equal(request.url.split('?')[0], url, `URL is ${url}`);
    assert.equal(find('[data-test-output]').textContent, 'Hello World');
  });
});
