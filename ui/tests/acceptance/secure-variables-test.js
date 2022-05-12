import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';

module('Acceptance | secure variables', function (hooks) {
  setupApplicationTest(hooks);

  test('visiting /variables', async function (assert) {
    await visit('/variables');

    assert.equal(currentURL(), '/variables');
  });
});
