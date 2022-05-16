import { module, test } from 'qunit';
import { visit, currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import defaultScenario from '../../mirage/scenarios/default';

import Variables from 'nomad-ui/tests/pages/variables';
import Layout from 'nomad-ui/tests/pages/layout';

const SECURE_TOKEN_ID = '53cur3-v4r14bl35';

module('Acceptance | secure variables', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  module('Guarding page access', function () {
    test('it redirects to jobs and hides the gutter link when the token lacks permissions', async function (assert) {
      await Variables.visit();
      assert.equal(currentURL(), '/jobs');
      assert.ok(Layout.gutter.variables.isHidden);
    });

    test('it allows access for management level tokens', async function (assert) {
      defaultScenario(server);
      window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;

      await Variables.visit();
      assert.equal(currentURL(), '/variables');
      assert.ok(Layout.gutter.variables.isVisible);
    });

    test('it allows access for list-variables allowed ACL rules', async function (assert) {
      defaultScenario(server);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;

      await Variables.visit();
      assert.equal(currentURL(), '/variables');
      assert.ok(Layout.gutter.variables.isVisible);
    });

    test('it passes an accessibility audit', async function (assert) {
      defaultScenario(server);
      const variablesToken = server.db.tokens.find(SECURE_TOKEN_ID);
      window.localStorage.nomadTokenSecret = variablesToken.secretId;
      await Variables.visit();
      await a11yAudit(assert);
    });
  });
});
