/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render, rerender } from '@ember/test-helpers';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import Service from '@ember/service';
import { TrackedObject } from 'tracked-built-ins';
import AllocationServiceSidebar from 'nomad-ui/components/allocation-service-sidebar';

module(
  'Integration | Component | allocation-service-sidebar',
  function (hooks) {
    setupRenderingTest(hooks);

    hooks.beforeEach(function () {
      const mockSystem = Service.extend({
        agent: {
          config: {
            UI: {
              Consul: {
                BaseUIURL: '',
              },
            },
          },
        },
      });
      this.owner.register('service:system', mockSystem);
      this.system = this.owner.lookup('service:system');
    });

    test('it supports basic open/close states', async function (assert) {
      await componentA11yAudit(this.element, assert);

      const state = new TrackedObject({
        service: { name: 'Funky Service' },
      });
      const closeSidebar = () => {
        state.service = null;
      };
      const fns = { closeSidebar };

      await render(
        <template>
          <AllocationServiceSidebar @service={{state.service}} @fns={{fns}} />
        </template>,
      );
      assert.dom('h1').includesText('Funky Service');
      assert.dom('.sidebar').hasClass('open');

      state.service = null;
      await rerender();
      assert.dom(this.element).hasText('');
      assert.dom('.sidebar').doesNotHaveClass('open');

      state.service = { name: 'Funky Service' };
      await rerender();
      await click('[data-test-close-service-sidebar]');
      await rerender();
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

      const state = new TrackedObject({
        service: healthyService,
        allocation: { id: 'myAlloc', clientStatus: 'running' },
      });
      const closeSidebar = () => {
        state.service = null;
      };
      const fns = { closeSidebar };

      await render(
        <template>
          <AllocationServiceSidebar
            @service={{state.service}}
            @allocation={{state.allocation}}
            @fns={{fns}}
          />
        </template>,
      );
      assert.dom('h1 .aggregate-status').includesText('Healthy');
      assert
        .dom('table.health-checks tbody tr:not(.service-status-indicators)')
        .exists({ count: 2 }, 'has two rows');

      state.service = unhealthyService;
      await rerender();
      assert.dom('h1 .aggregate-status').includesText('Unhealthy');

      state.service = healthyService;
      state.allocation = { id: 'myAlloc2', clientStatus: 'failed' };
      await rerender();
      assert.dom('h1 .aggregate-status').includesText('Health Unknown');
    });

    test('it handles Consul services with reduced functionality', async function (assert) {
      const consulService = {
        name: 'Consul Service',
        provider: 'consul',
        healthChecks: [],
      };

      const state = new TrackedObject({
        service: consulService,
      });
      const closeSidebar = () => {
        state.service = null;
      };
      const fns = { closeSidebar };

      await render(
        <template>
          <AllocationServiceSidebar @service={{state.service}} @fns={{fns}} />
        </template>,
      );
      assert.dom('h1 .aggregate-status').doesNotExist();
      assert.dom('table.health-checks').doesNotExist();
      assert.dom('[data-test-consul-link-notice]').doesNotExist();

      this.system.agent.config.UI.Consul.BaseUIURL = 'http://localhost:8500';
      await render(
        <template>
          <AllocationServiceSidebar @service={{state.service}} @fns={{fns}} />
        </template>,
      );

      assert.dom('[data-test-consul-link-notice]').exists();
    });
  },
);
