/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, find, findAll, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | scale-events-accordion', function (hooks) {
  setupRenderingTest(hooks);
  setupCodeMirror(hooks);

  hooks.beforeEach(function () {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('node-pool');
    this.server.create('node');
    this.taskGroupWithEvents = async function (events) {
      const job = this.server.create('job', { createAllocations: false });
      const group = job.taskGroups.models[0];
      job.jobScale.taskGroupScales.models
        .findBy('name', group.name)
        .update({ events });

      const jobModel = await this.store.find(
        'job',
        JSON.stringify([job.id, 'default'])
      );
      await jobModel.get('scaleState');
      return jobModel.taskGroups.findBy('name', group.name);
    };
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  const commonTemplate = hbs`<ScaleEventsAccordion @events={{this.events}} />`;

  test('it shows an accordion with an entry for each event', async function (assert) {
    assert.expect(2);

    const eventCount = 5;
    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', eventCount)
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);

    assert.equal(
      findAll('[data-test-scale-events] [data-test-accordion-head]').length,
      eventCount
    );
    await componentA11yAudit(this.element, assert);
  });

  test('when an event is an error, an error icon is shown', async function (assert) {
    assert.expect(2);

    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', 1, { error: true })
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);

    assert.ok(find('[data-test-error]'));
    await componentA11yAudit(this.element, assert);
  });

  test('when an event has a count higher than previous count, a danger up arrow is shown', async function (assert) {
    assert.expect(4);

    const count = 5;
    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', 1, {
        count,
        previousCount: count - 1,
        error: false,
      })
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);

    assert.notOk(find('[data-test-error]'));
    assert.equal(find('[data-test-count]').textContent, count);
    assert.ok(
      find('[data-test-count-icon]')
        .querySelector('.icon')
        .classList.contains('is-danger')
    );
    await componentA11yAudit(this.element, assert);
  });

  test('when an event has a count lower than previous count, a primary down arrow is shown', async function (assert) {
    const count = 5;
    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', 1, {
        count,
        previousCount: count + 1,
        error: false,
      })
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);

    assert.notOk(find('[data-test-error]'));
    assert.equal(find('[data-test-count]').textContent, count);
    assert.ok(
      find('[data-test-count-icon]')
        .querySelector('.icon')
        .classList.contains('is-primary')
    );
  });

  test('when an event has no count, the count is omitted', async function (assert) {
    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', 1, { count: null })
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);

    assert.notOk(find('[data-test-count]'));
    assert.notOk(find('[data-test-count-icon]'));
  });

  test('when an event has no meta properties, the accordion entry is not expandable', async function (assert) {
    assert.expect(2);

    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', 1, { meta: {} })
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);

    assert.ok(
      find('[data-test-accordion-toggle]').classList.contains('is-invisible')
    );
    await componentA11yAudit(this.element, assert);
  });

  test('when an event has meta properties, the accordion entry is expanding, presenting the meta properties in a json viewer', async function (assert) {
    assert.expect(4);

    const meta = {
      prop: 'one',
      prop2: 'two',
      deep: {
        prop: 'here',
        'dot.separate.prop': 12,
      },
    };
    const taskGroup = await this.taskGroupWithEvents(
      server.createList('scale-event', 1, { meta })
    );
    this.set('events', taskGroup.scaleState.events);

    await render(commonTemplate);
    assert.notOk(find('[data-test-accordion-body]'));

    await click('[data-test-accordion-toggle]');
    assert.ok(find('[data-test-accordion-body]'));

    assert.equal(
      getCodeMirrorInstance('[data-test-json-viewer]').getValue(),
      JSON.stringify(meta, null, 2)
    );
    await componentA11yAudit(this.element, assert);
  });
});
