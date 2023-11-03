import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';

module('Acceptance | actions', function (hooks) {
  setupApplicationTest(hooks);

  test('visiting /actions', async function (assert) {
    await visit('/actions');

    assert.equal(currentURL(), '/actions');
  });
});
