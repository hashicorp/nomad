import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module(
  'Integration | Component | allocation-service-sidebar',
  function (hooks) {
    setupRenderingTest(hooks);

    test('it supports basic open/close states', async function (assert) {
      this.set('closeSidebar', () => this.set('service', null));

      this.set('service', { name: 'Funky Service' });
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom(this.element).hasText('Service Details for Funky Service');
      assert.dom('.sidebar').hasClass('open');

      this.set('service', null);
      await render(
        hbs`<AllocationServiceSidebar @service={{this.service}} @fns={{hash closeSidebar=this.closeSidebar}} />`
      );
      assert.dom(this.element).hasText('');
      assert.dom('.sidebar').doesNotHaveClass('open');

      await click('[data-test-close-service-sidebar');
      assert.dom(this.element).hasText('');
      assert.dom('.sidebar').doesNotHaveClass('open');
    });
  }
);
