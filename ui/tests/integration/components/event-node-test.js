import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | event-node', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`<EventNode />`);

    assert.dom(this.element).hasText('');

    // Template block usage:
    await render(hbs`
      <EventNode>
        template block text
      </EventNode>
    `);

    assert.dom(this.element).hasText('template block text');
  });
});
