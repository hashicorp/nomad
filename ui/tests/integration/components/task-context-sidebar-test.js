import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | task-context-sidebar', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    assert.expect(2);

    await render(hbs`<TaskContextSidebar />`);

    assert.dom(this.element).hasText('');

    // Template block usage:
    await render(hbs`
      <TaskContextSidebar>
        template block text
      </TaskContextSidebar>
    `);

    assert.dom(this.element).hasText('template block text');
    await componentA11yAudit(this.element, assert);
  });
});
