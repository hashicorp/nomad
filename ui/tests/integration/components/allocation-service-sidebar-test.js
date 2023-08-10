/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import Service from '@ember/service';
import EmberObject from '@ember/object';

module(
  'Integration | Component | allocation-service-sidebar',
  function (hooks) {
    setupRenderingTest(hooks);
    hooks.beforeEach(function () {
      const mockSystem = Service.extend({
        agent: EmberObject.create({
          config: {
            UI: {
              Consul: {
                BaseUIURL: '',
              },
            },
          },
        }),
      });
      this.owner.register('service:system', mockSystem);
      this.system = this.owner.lookup('service:system');
    });

    test('it supports basic open/close states', async function (assert) {
      assert.expect(7);
      await componentA11yAudit(this.element, assert);

      this.set('closeSidebar', () => this.set('service', null));

      this.set('service', { name: 'Funky Service' });
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom('h1').includesText('Funky Service');
      assert.dom('.sidebar').hasClass('open');

      this.set('service', null);
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom(this.element).hasText('');
      assert.dom('.sidebar').doesNotHaveClass('open');

      this.set('service', { name: 'Funky Service' });
      await click('[data-test-close-service-sidebar]');
      assert.dom(this.element).hasText('');
      assert.dom('.sidebar').doesNotHaveClass('open');
    });

    test('it correctly aggregates service health', async function (assert) {
      const healthyService = {
        name: 'Funky Service',
        provider: 'nomad',
        healthChecks: [
          { Check: 'one', Status: 'success', Alloc: 'myAlloc' },
          { Check: 'two', Status: 'success', Alloc: 'myAlloc' },
        ],
      };
      const unhealthyService = {
        name: 'Funky Service',
        provider: 'nomad',
        healthChecks: [
          { Check: 'one', Status: 'failure', Alloc: 'myAlloc' },
          { Check: 'two', Status: 'success', Alloc: 'myAlloc' },
        ],
      };

      this.set('closeSidebar', () => this.set('service', null));
      this.set('allocation', { id: 'myAlloc', clientStatus: 'running' });
      this.set('service', healthyService);
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @allocation={{this.allocation}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom('h1 .aggregate-status').includesText('Healthy');
      assert
        .dom('table.health-checks tbody tr:not(.service-status-indicators)')
        .exists({ count: 2 }, 'has two rows');

      this.set('service', unhealthyService);
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @allocation={{this.allocation}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom('h1 .aggregate-status').includesText('Unhealthy');

      this.set('service', healthyService);
      this.set('allocation', { id: 'myAlloc2', clientStatus: 'failed' });
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @allocation={{this.allocation}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom('h1 .aggregate-status').includesText('Health Unknown');
    });

    test('it handles Consul services with reduced functionality', async function (assert) {
      const consulService = {
        name: 'Consul Service',
        provider: 'consul',
        healthChecks: [],
      };

      this.set('closeSidebar', () => this.set('service', null));
      this.set('service', consulService);
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom('h1 .aggregate-status').doesNotExist();
      assert.dom('table.health-checks').doesNotExist();
      assert.dom('[data-test-consul-link-notice]').doesNotExist();

      this.system.agent.config.UI.Consul.BaseUIURL = 'http://localhost:8500';

      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );

      assert.dom('[data-test-consul-link-notice]').exists();
    });
  }
);
