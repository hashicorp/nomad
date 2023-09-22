import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | roles', function (hooks) {
  setupApplicationTest(hooks);

  test('visiting /roles', async function (assert) {
    assert.expect(1);
    await visit('/access-control/roles');
    await a11yAudit(assert);

    assert.equal(currentURL(), '/access-control/roles');
  });
});
