import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import { find, click } from 'ember-native-dom-helpers';
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
    click('[data-test-header-gutter-toggle]');

    return settled().then(() => {
      assert.ok(find('[data-test-gutter-menu]').classList.contains('is-open'), 'Gutter menu is open');
    });
  });

  test('the gutter-menu hamburger menu closes the gutter menu', async function(assert) {
    await render(hbs`{{page-layout}}`);

    click('[data-test-header-gutter-toggle]');

    return settled()
      .then(() => {
        assert.ok(
          find('[data-test-gutter-menu]').classList.contains('is-open'),
          'Gutter menu is open'
        );
        click('[data-test-gutter-gutter-toggle]');
        return settled();
      })
      .then(() => {
        assert.notOk(
          find('[data-test-gutter-menu]').classList.contains('is-open'),
          'Gutter menu is not open'
        );
      });
  });

  test('the gutter-menu backdrop closes the gutter menu', async function(assert) {
    await render(hbs`{{page-layout}}`);

    click('[data-test-header-gutter-toggle]');

    return settled()
      .then(() => {
        assert.ok(
          find('[data-test-gutter-menu]').classList.contains('is-open'),
          'Gutter menu is open'
        );
        click('[data-test-gutter-backdrop]');
        return settled();
      })
      .then(() => {
        assert.notOk(
          find('[data-test-gutter-menu]').classList.contains('is-open'),
          'Gutter menu is not open'
        );
      });
  });
});
