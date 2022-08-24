import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';

module('Acceptance | job services', function (hooks) {
  setupApplicationTest(hooks);

  test('visiting /job-services', async function (assert) {
    await visit('/job-services');

    assert.equal(currentURL(), '/job-services');
  });
});
