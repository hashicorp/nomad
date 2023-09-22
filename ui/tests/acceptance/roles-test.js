import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';

module('Acceptance | roles', function (hooks) {
  setupApplicationTest(hooks);

  test('visiting /roles', async function (assert) {
    await visit('/roles');

    assert.equal(currentURL(), '/roles');
  });
});
