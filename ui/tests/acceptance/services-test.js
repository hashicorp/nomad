import { module, test } from 'qunit';
import { visit, currentURL, findAll } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import percySnapshot from '@percy/ember';
import Services from 'nomad-ui/tests/pages/services';
import Layout from 'nomad-ui/tests/pages/layout';
import defaultScenario from '../../mirage/scenarios/default';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';

module('Acceptance | services', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);
    defaultScenario(server);
    await Services.visit();
    await a11yAudit(assert);
  });

  module('traversal', function () {
    test('visiting /services by url', async function (assert) {
      defaultScenario(server);
      await Services.visit();
      assert.equal(currentURL(), '/services');
    });

    test('main menu correctly takes you to services', async function (assert) {
      assert.expect(1);
      defaultScenario(server);
      await visit('/');
      await Layout.gutter.visitServices();
      assert.equal(currentURL(), '/services');
      await percySnapshot(assert);
    });
  });

  module('services index table', function () {
    test('services table shows expected number of services', async function (assert) {
      server.createList('service', 3);
      await Services.visit();
      assert.equal(
        findAll('[data-test-service-row]').length,
        3,
        'correctly shows 3 services'
      );
    });
  });
});
