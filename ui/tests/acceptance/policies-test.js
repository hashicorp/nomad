import { module, test } from 'qunit';
import { visit, currentURL, click, typeIn } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { allScenarios } from '../../mirage/scenarios/default';
import { setupMirage } from 'ember-cli-mirage/test-support';
import percySnapshot from '@percy/ember';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | policies', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('Policies index route looks good', async function (assert) {
    assert.expect(4);
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');
    assert.dom('[data-test-gutter-link="policies"]').exists();
    assert.equal(currentURL(), '/policies');
    assert
      .dom('[data-test-policy-row]')
      .exists({ count: server.db.policies.length });
    await a11yAudit(assert);
    await percySnapshot(assert);
  });

  test('Prevents policies access if you lack a management token', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[1].secretId;
    await visit('/policies');
    assert.equal(currentURL(), '/jobs');
    assert.dom('[data-test-gutter-link="policies"]').doesNotExist();
  });

  test('Modifying an existing policy', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');
    await click('[data-test-policy-row]:first-child');
    assert.equal(currentURL(), `/policies/${server.db.policies[0].name}`);
    assert.dom('[data-test-policy-editor]').exists();
    assert.dom('[data-test-title]').includesText(server.db.policies[0].name);
    await click('button[type="submit"]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/policies');
  });

  test('Doesnt let you save a bad name', async function (assert) {
    allScenarios.policiesTestCluster(server);
    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');
    await click('[data-test-create-policy]');
    assert.equal(currentURL(), '/policies/new');
    await typeIn('[data-test-policy-name]', 'My Fun Policy');
    await click('button[type="submit"]');
    assert.dom('.flash-message.alert-error').exists();
    assert.equal(currentURL(), '/policies/new');
    document.querySelector('[data-test-policy-name]').value = ''; // clear
    await typeIn('[data-test-policy-name]', 'My-Fun-Policy');
    await click('button[type="submit"]');
    assert.dom('.flash-message.alert-success').exists();
    assert.equal(currentURL(), '/policies');
  });
});
