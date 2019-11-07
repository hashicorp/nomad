import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

module('Integration | Component | page layout', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    this.server = startMirage();
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  test('the global-header hamburger menu opens the gutter menu', async function(assert) {
    await render(hbs`{{page-layout}}`);

    assert.notOk(
      find('[data-test-gutter-menu]').classList.contains('is-open'),
      'Gutter menu is not open'
    );
    await click('[data-test-header-gutter-toggle]');

    assert.ok(find('[data-test-gutter-menu]').classList.contains('is-open'), 'Gutter menu is open');
  });

  test('the gutter-menu hamburger menu closes the gutter menu', async function(assert) {
    await render(hbs`{{page-layout}}`);

    await click('[data-test-header-gutter-toggle]');

    assert.ok(find('[data-test-gutter-menu]').classList.contains('is-open'), 'Gutter menu is open');
    await click('[data-test-gutter-gutter-toggle]');

    assert.notOk(
      find('[data-test-gutter-menu]').classList.contains('is-open'),
      'Gutter menu is not open'
    );
  });

  test('the gutter-menu backdrop closes the gutter menu', async function(assert) {
    await render(hbs`{{page-layout}}`);

    await click('[data-test-header-gutter-toggle]');

    assert.ok(find('[data-test-gutter-menu]').classList.contains('is-open'), 'Gutter menu is open');
    await click('[data-test-gutter-backdrop]');

    assert.notOk(
      find('[data-test-gutter-menu]').classList.contains('is-open'),
      'Gutter menu is not open'
    );
  });
});
