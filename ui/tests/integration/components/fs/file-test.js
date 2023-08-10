/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, click, render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Pretender from 'pretender';
import { logEncode } from '../../../../mirage/data/logs';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

const { assign } = Object;
const HOST = '1.1.1.1:1111';

module('Integration | Component | fs/file', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    this.server = new Pretender(function () {
      this.get('/v1/agent/members', () => [
        200,
        {},
        JSON.stringify({ ServerRegion: 'default', Members: [] }),
      ]);
      this.get('/v1/regions', () => [
        200,
        {},
        JSON.stringify(['default', 'region-2']),
      ]);
      this.get('/v1/client/fs/stream/:alloc_id', () => [
        200,
        {},
        logEncode(['Hello World'], 0),
      ]);
      this.get('/v1/client/fs/cat/:alloc_id', () => [200, {}, 'Hello World']);
      this.get('/v1/client/fs/readat/:alloc_id', () => [
        200,
        {},
        'Hello World',
      ]);
    });
    this.system = this.owner.lookup('service:system');
  });

  hooks.afterEach(function () {
    this.server.shutdown();
    window.localStorage.clear();
  });

  const commonTemplate = hbs`
    <Fs::File @allocation={{this.allocation}} @taskState={{this.taskState}} @file={{this.file}} @stat={{this.stat}} />
  `;

  const fileStat = (type, size = 0) => ({
    stat: {
      Size: size,
      ContentType: type,
    },
  });
  const makeProps = (props = {}) =>
    assign(
      {},
      {
        allocation: {
          id: 'alloc-1',
          node: {
            httpAddr: HOST,
          },
        },
        taskState: {
          name: 'task-name',
        },
        file: 'path/to/file',
        stat: {
          Size: 12345,
          ContentType: 'text/plain',
        },
      },
      props
    );

  test('When a file is text-based, the file mode is streaming', async function (assert) {
    assert.expect(3);

    const props = makeProps(fileStat('text/plain', 500));
    this.setProperties(props);

    await render(commonTemplate);

    assert.ok(
      find('[data-test-file-box] [data-test-log-cli]'),
      'The streaming file component was rendered'
    );
    assert.notOk(
      find('[data-test-file-box] [data-test-image-file]'),
      'The image file component was not rendered'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('When a file is an image, the file mode is image', async function (assert) {
    assert.expect(3);

    const props = makeProps(fileStat('image/png', 1234));
    this.setProperties(props);

    await render(commonTemplate);

    assert.ok(
      find('[data-test-file-box] [data-test-image-file]'),
      'The image file component was rendered'
    );
    assert.notOk(
      find('[data-test-file-box] [data-test-log-cli]'),
      'The streaming file component was not rendered'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('When the file is neither text-based or an image, the unsupported file type empty state is shown', async function (assert) {
    assert.expect(4);

    const props = makeProps(fileStat('wat/ohno', 1234));
    this.setProperties(props);

    await render(commonTemplate);

    assert.notOk(
      find('[data-test-file-box] [data-test-image-file]'),
      'The image file component was not rendered'
    );
    assert.notOk(
      find('[data-test-file-box] [data-test-log-cli]'),
      'The streaming file component was not rendered'
    );
    assert.ok(
      find('[data-test-unsupported-type]'),
      'Unsupported file type message is shown'
    );
    await componentA11yAudit(this.element, assert);
  });

  test('The unsupported file type empty state includes a link to the raw file', async function (assert) {
    const props = makeProps(fileStat('wat/ohno', 1234));
    this.setProperties(props);

    await render(commonTemplate);

    assert.ok(
      find('[data-test-unsupported-type] [data-test-log-action="raw"]'),
      'Unsupported file type message includes a link to the raw file'
    );

    assert.notOk(
      find('[data-test-header] [data-test-log-action="raw"]'),
      'Raw link is no longer in the header'
    );
  });

  test('The view raw button goes to the correct API url', async function (assert) {
    const props = makeProps(fileStat('image/png', 1234));
    this.setProperties(props);

    await render(commonTemplate);
    click('[data-test-log-action="raw"]');
    await settled();
    assert.ok(
      this.server.handledRequests.find(
        ({ url: url }) =>
          url ===
          `/v1/client/fs/cat/${props.allocation.id}?path=${encodeURIComponent(
            `${props.taskState.name}/${props.file}`
          )}`
      ),
      'Request to file is made'
    );
  });

  test('The view raw button respects the active region', async function (assert) {
    const region = 'region-2';
    window.localStorage.nomadActiveRegion = region;

    const props = makeProps(fileStat('image/png', 1234));
    this.setProperties(props);

    await this.system.get('regions');
    await render(commonTemplate);

    click('[data-test-log-action="raw"]');
    await settled();
    assert.ok(
      this.server.handledRequests.find(
        ({ url: url }) =>
          url ===
          `/v1/client/fs/cat/${props.allocation.id}?path=${encodeURIComponent(
            `${props.taskState.name}/${props.file}`
          )}&region=${region}`
      ),
      'Request to file is made with region'
    );
  });

  test('The head and tail buttons are not shown when the file is small', async function (assert) {
    const props = makeProps(fileStat('application/json', 5000));
    this.setProperties(props);

    await render(commonTemplate);

    assert.notOk(find('[data-test-log-action="head"]'), 'No head button');
    assert.notOk(find('[data-test-log-action="tail"]'), 'No tail button');

    this.set('stat.Size', 100000);

    await settled();

    assert.ok(find('[data-test-log-action="head"]'), 'Head button is shown');
    assert.ok(find('[data-test-log-action="tail"]'), 'Tail button is shown');
  });

  test('The  head and tail buttons are not shown for an image file', async function (assert) {
    const props = makeProps(fileStat('image/svg', 5000));
    this.setProperties(props);

    await render(commonTemplate);

    assert.notOk(find('[data-test-log-action="head"]'), 'No head button');
    assert.notOk(find('[data-test-log-action="tail"]'), 'No tail button');

    this.set('stat.Size', 100000);

    await settled();

    assert.notOk(find('[data-test-log-action="head"]'), 'Still no head button');
    assert.notOk(find('[data-test-log-action="tail"]'), 'Still no tail button');
  });

  test('Yielded content goes in the top-left header area', async function (assert) {
    assert.expect(2);

    const props = makeProps(fileStat('image/svg', 5000));
    this.setProperties(props);

    await render(hbs`
      <Fs::File @allocation={{this.allocation}} @taskState={{this.taskState}} @file={{this.file}} @stat={{this.stat}}>
        <div data-test-yield-spy>Yielded content</div>
      </Fs::File>
    `);

    assert.ok(
      find('[data-test-header] [data-test-yield-spy]'),
      'Yielded content shows up in the header'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('The body is full-bleed and dark when the file is streaming', async function (assert) {
    const props = makeProps(fileStat('application/json', 5000));
    this.setProperties(props);

    await render(commonTemplate);

    const classes = Array.from(find('[data-test-file-box]').classList);
    assert.ok(classes.includes('is-dark'), 'Body is dark');
    assert.ok(classes.includes('is-full-bleed'), 'Body is full-bleed');
  });

  test('The body has padding and a light background when the file is not streaming', async function (assert) {
    const props = makeProps(fileStat('image/jpeg', 5000));
    this.setProperties(props);

    await render(commonTemplate);

    let classes = Array.from(find('[data-test-file-box]').classList);
    assert.notOk(classes.includes('is-dark'), 'Body is not dark');
    assert.notOk(classes.includes('is-full-bleed'), 'Body is not full-bleed');

    this.set('stat.ContentType', 'something/unknown');

    await settled();

    classes = Array.from(find('[data-test-file-box]').classList);
    assert.notOk(classes.includes('is-dark'), 'Body is still not dark');
    assert.notOk(
      classes.includes('is-full-bleed'),
      'Body is still not full-bleed'
    );
  });
});
