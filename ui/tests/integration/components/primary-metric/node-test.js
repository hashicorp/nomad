/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { find, render } from '@ember/test-helpers';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import hbs from 'htmlbars-inline-precompile';
import { setupPrimaryMetricMocks, primaryMetric } from './primary-metric';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { formatScheduledHertz } from 'nomad-ui/utils/units';

module('Integration | Component | PrimaryMetric::Node', function (hooks) {
  setupRenderingTest(hooks);
  setupPrimaryMetricMocks(hooks);

  hooks.beforeEach(function () {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node-pool');
    this.server.create('node');
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  const template = hbs`
    <PrimaryMetric::Node
      @node={{this.resource}}
      @metric={{this.metric}} />
  `;

  const preload = async (store) => {
    await store.findAll('node');
  };

  const findResource = (store) => store.peekAll('node').get('firstObject');

  test('Must pass an accessibility audit', async function (assert) {
    assert.expect(1);

    await preload(this.store);

    const resource = findResource(this.store);
    this.setProperties({ resource, metric: 'cpu' });

    await render(template);
    await componentA11yAudit(this.element, assert);
  });

  test('When the node has a reserved amount for the metric, a horizontal annotation is shown', async function (assert) {
    this.server.create('node', 'reserved', { id: 'withAnnotation' });
    await preload(this.store);

    const resource = this.store.peekRecord('node', 'withAnnotation');
    this.setProperties({ resource, metric: 'cpu' });

    await render(template);

    assert.ok(find('[data-test-annotation]'));
    assert.equal(
      find('[data-test-annotation]').textContent.trim(),
      `${formatScheduledHertz(resource.reserved.cpu, 'MHz')} reserved`
    );
  });

  primaryMetric({
    template,
    preload,
    findResource,
  });
});
