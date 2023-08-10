/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, find, findAll, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import moment from 'moment';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | scale-events-chart', function (hooks) {
  setupRenderingTest(hooks);
  setupCodeMirror(hooks);

  const events = [
    {
      time: new Date('2020-08-05T04:06:00'),
      count: 2,
      hasCount: true,
      meta: {},
      message: '',
      error: false,
    },
    {
      time: new Date('2020-08-06T04:06:00'),
      count: 3,
      hasCount: true,
      meta: {},
      message: '',
      error: false,
    },
    {
      time: new Date('2020-08-07T04:06:00'),
      count: 4,
      hasCount: true,
      meta: {},
      message: '',
      error: false,
    },
    {
      time: new Date('2020-08-06T04:06:00'),
      hasCount: false,
      meta: { prop: { deep: true }, five: 5 },
      message: 'Something went wrong',
      error: true,
    },
    {
      time: new Date('2020-08-05T04:06:00'),
      hasCount: false,
      meta: {},
      message: 'Something insightful',
      error: false,
    },
  ];

  test('each event is rendered as an annotation', async function (assert) {
    assert.expect(2);

    this.set('events', events);
    await render(hbs`<ScaleEventsChart @events={{this.events}} />`);

    assert.equal(
      findAll('[data-test-annotation]').length,
      events.filter((ev) => ev.count == null).length
    );
    await componentA11yAudit(this.element, assert);
  });

  test('clicking an annotation presents details for the event', async function (assert) {
    assert.expect(6);

    const annotation = events.rejectBy('hasCount').sortBy('time').reverse()[0];

    this.set('events', events);
    await render(hbs`<ScaleEventsChart @events={{this.events}} />`);

    assert.notOk(find('[data-test-event-details]'));
    await click('[data-test-annotation] button');

    assert.ok(find('[data-test-event-details]'));
    assert.equal(
      find('[data-test-timestamp]').textContent,
      moment(annotation.time).format('MMM DD HH:mm:ss ZZ')
    );
    assert.equal(find('[data-test-message]').textContent, annotation.message);
    assert.equal(
      getCodeMirrorInstance('[data-test-json-viewer]').getValue(),
      JSON.stringify(annotation.meta, null, 2)
    );

    await componentA11yAudit(this.element, assert);
  });

  test('clicking an active annotation closes event details', async function (assert) {
    this.set('events', events);

    await render(hbs`<ScaleEventsChart @events={{this.events}} />`);
    assert.notOk(find('[data-test-event-details]'));

    await click('[data-test-annotation] button');
    assert.ok(find('[data-test-event-details]'));

    await click('[data-test-annotation] button');
    assert.notOk(find('[data-test-event-details]'));
  });
});
