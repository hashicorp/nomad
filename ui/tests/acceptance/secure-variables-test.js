import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Variables from 'nomad-ui/tests/pages/variables';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | secure variables', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function (assert) {
    await Variables.visit();
    assert.equal(currentURL(), '/jobs');
  });

  // test('it passes an accessibility audit', async function (assert) {
  //   await Variables.visit();
  //   await a11yAudit(assert);
  // });

  // test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function (assert) {
  //   window.localStorage.nomadTokenSecret = clientToken.secretId;
  //   await Optimize.visit();

  //   assert.equal(currentURL(), '/jobs?namespace=*');
  //   assert.ok(Layout.gutter.optimize.isHidden);
  // });
});
