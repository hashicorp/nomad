import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module(
  'Integration | Component | allocation-service-sidebar',
  function (hooks) {
    setupRenderingTest(hooks);

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
  }
);
