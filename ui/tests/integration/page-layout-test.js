import { test, moduleForComponent } from 'ember-qunit';
import { find, click } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleForComponent('page-layout', 'Integration | Component | page layout', {
  integration: true,
  beforeEach() {
    this.server = startMirage();
  },
  afterEach() {
    this.server.shutdown();
  },
});

test('the global-header hamburger menu opens the gutter menu', function(assert) {
  this.render(hbs`{{page-layout}}`);

  assert.notOk(
    find('[data-test-gutter-menu]').classList.contains('is-open'),
    'Gutter menu is not open'
  );
  click('[data-test-header-gutter-toggle]');

  return wait().then(() => {
    assert.ok(find('[data-test-gutter-menu]').classList.contains('is-open'), 'Gutter menu is open');
  });
});

test('the gutter-menu hamburger menu closes the gutter menu', function(assert) {
  this.render(hbs`{{page-layout}}`);

  click('[data-test-header-gutter-toggle]');

  return wait()
    .then(() => {
      assert.ok(
        find('[data-test-gutter-menu]').classList.contains('is-open'),
        'Gutter menu is open'
      );
      click('[data-test-gutter-gutter-toggle]');
      return wait();
    })
    .then(() => {
      assert.notOk(
        find('[data-test-gutter-menu]').classList.contains('is-open'),
        'Gutter menu is not open'
      );
    });
});

test('the gutter-menu backdrop closes the gutter menu', function(assert) {
  this.render(hbs`{{page-layout}}`);

  click('[data-test-header-gutter-toggle]');

  return wait()
    .then(() => {
      assert.ok(
        find('[data-test-gutter-menu]').classList.contains('is-open'),
        'Gutter menu is open'
      );
      click('[data-test-gutter-backdrop]');
      return wait();
    })
    .then(() => {
      assert.notOk(
        find('[data-test-gutter-menu]').classList.contains('is-open'),
        'Gutter menu is not open'
      );
    });
});
