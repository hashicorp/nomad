/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { currentURL, find, findAll, visit, click } from '@ember/test-helpers';
import { module, skip, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import Jobs from 'nomad-ui/tests/pages/jobs/list';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import ClientDetail from 'nomad-ui/tests/pages/clients/detail';
import Layout from 'nomad-ui/tests/pages/layout';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';
import moment from 'moment';
import { run } from '@ember/runloop';
import { allScenarios } from '../../mirage/scenarios/default';
import {
  selectChoose,
  clickTrigger,
} from 'ember-power-select/test-support/helpers';

let job;
let node;
let managementToken;
let clientToken;
module('Acceptance | tokens', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    window.sessionStorage.clear();
    faker.seed(1);

    server.create('agent');
    server.create('node-pool');
    node = server.create('node');
    job = server.create('job');
    managementToken = server.create('token');
    clientToken = server.create('token');
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);

    await Tokens.visit();
    await a11yAudit(assert);
  });

  test('the token form sets the token in local storage', async function (assert) {
    const { secretId } = managementToken;

    await Tokens.visit();
    assert.equal(
      window.localStorage.nomadTokenSecret,
      null,
      'No token secret set'
    );
    assert.ok(document.title.includes('Authorization'));

    await Tokens.secret(secretId).submit();
    assert.equal(
      window.localStorage.nomadTokenSecret,
      secretId,
      'Token secret was set'
    );
  });

  // TODO: unskip once store.unloadAll reliably waits for in-flight requests to settle
  skip('the x-nomad-token header gets sent with requests once it is set', async function (assert) {
    const { secretId } = managementToken;

    await JobDetail.visit({ id: job.id });
    await ClientDetail.visit({ id: node.id });

    assert.ok(
      server.pretender.handledRequests.length > 1,
      'Requests have been made'
    );

    server.pretender.handledRequests.forEach((req) => {
      assert.notOk(getHeader(req, 'x-nomad-token'), `No token for ${req.url}`);
    });

    const requestPosition = server.pretender.handledRequests.length;

    await Tokens.visit();
    await Tokens.secret(secretId).submit();

    await JobDetail.visit({ id: job.id });
    await ClientDetail.visit({ id: node.id });

    const newRequests = server.pretender.handledRequests.slice(requestPosition);
    assert.ok(newRequests.length > 1, 'New requests have been made');

    // Cross-origin requests can't have a token
    newRequests.forEach((req) => {
      assert.equal(
        getHeader(req, 'x-nomad-token'),
        secretId,
        `Token set for ${req.url}`
      );
    });
  });

  test('an error message is shown when authenticating a token fails', async function (assert) {
    const { secretId } = managementToken;
    const bogusSecret = 'this-is-not-the-secret';
    assert.notEqual(
      secretId,
      bogusSecret,
      'bogus secret is not somehow coincidentally equal to the real secret'
    );

    await Tokens.visit();
    await Tokens.secret(bogusSecret).submit();

    assert.equal(
      window.localStorage.nomadTokenSecret,
      null,
      'Token secret is discarded on failure'
    );
    assert.ok(Tokens.errorMessage, 'Token error message is shown');
    assert.notOk(Tokens.successMessage, 'Token success message is not shown');
    assert.equal(Tokens.policies.length, 0, 'No token policies are shown');
  });

  test('a success message and a special management token message are shown when authenticating succeeds', async function (assert) {
    const { secretId } = managementToken;

    await Tokens.visit();
    await Tokens.secret(secretId).submit();

    await percySnapshot(assert);

    assert.ok(Tokens.successMessage, 'Token success message is shown');
    assert.notOk(Tokens.errorMessage, 'Token error message is not shown');
    assert.ok(Tokens.managementMessage, 'Token management message is shown');
    assert.equal(Tokens.policies.length, 0, 'No token policies are shown');
  });

  test('a success message and associated policies are shown when authenticating succeeds', async function (assert) {
    const { secretId } = clientToken;
    const policy = clientToken.policies.models[0];
    policy.update('description', 'Make sure there is a description');

    await Tokens.visit();
    await Tokens.secret(secretId).submit();

    assert.ok(Tokens.successMessage, 'Token success message is shown');
    assert.notOk(Tokens.errorMessage, 'Token error message is not shown');
    assert.notOk(
      Tokens.managementMessage,
      'Token management message is not shown'
    );
    assert.equal(
      Tokens.policies.length,
      clientToken.policies.length,
      'Each policy associated with the token is listed'
    );

    const policyElement = Tokens.policies.objectAt(0);

    assert.equal(policyElement.name, policy.name, 'Policy Name');
    assert.equal(
      policyElement.description,
      policy.description,
      'Policy Description'
    );
    assert.equal(policyElement.rules, policy.rules, 'Policy Rules');
  });

  test('setting a token clears the store', async function (assert) {
    const { secretId } = clientToken;

    await Jobs.visit();
    assert.ok(find('.job-row'), 'Jobs found');

    await Tokens.visit();
    await Tokens.secret(secretId).submit();

    server.pretender.get('/v1/jobs', function () {
      return [200, {}, '[]'];
    });

    await Jobs.visit();

    // If jobs are lingering in the store, they would show up
    assert.notOk(find('[data-test-job-row]'), 'No jobs found');
  });

  test('it handles expiring tokens', async function (assert) {
    // Soon-expiring token
    const expiringToken = server.create('token', {
      name: "Time's a-tickin",
      expirationTime: moment().add(1, 'm').toDate(),
    });

    await Tokens.visit();

    // Token with no TTL
    await Tokens.secret(clientToken.secretId).submit();
    assert
      .dom('[data-test-token-expiry]')
      .doesNotExist('No expiry shown for regular token');

    await Tokens.clear();

    // https://ember-concurrency.com/docs/testing-debugging/
    setTimeout(() => run.cancelTimers(), 500);

    // Token with TTL
    await Tokens.secret(expiringToken.secretId).submit();
    assert
      .dom('[data-test-token-expiry]')
      .exists('Expiry shown for TTL-having token');

    // TTL Action
    await Jobs.visit();
    assert
      .dom('.flash-message.alert-warning button')
      .exists('A global alert exists and has a clickable button');

    await click('.flash-message.alert-warning button');
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Redirected to tokens page on notification action'
    );
  });

  test('it handles expired tokens', async function (assert) {
    const expiredToken = server.create('token', {
      name: 'Well past due',
      expirationTime: moment().add(-5, 'm').toDate(),
    });

    // GC'd or non-existent token, from localStorage or otherwise
    window.localStorage.nomadTokenSecret = expiredToken.secretId;
    await Tokens.visit();
    assert
      .dom('[data-test-token-expired]')
      .exists('Warning banner shown for expired token');
  });

  test('it forces redirect on an expired token', async function (assert) {
    const expiredToken = server.create('token', {
      name: 'Well past due',
      expirationTime: moment().add(-5, 'm').toDate(),
    });

    window.localStorage.nomadTokenSecret = expiredToken.secretId;
    const expiredServerError = {
      errors: [
        {
          detail: 'ACL token expired',
        },
      ],
    };
    server.pretender.get('/v1/jobs', function () {
      return [500, {}, JSON.stringify(expiredServerError)];
    });

    await Jobs.visit();
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Redirected to tokens page due to an expired token'
    );
  });

  test('it forces redirect on a not-found token', async function (assert) {
    const longDeadToken = server.create('token', {
      name: 'dead and gone',
      expirationTime: moment().add(-5, 'h').toDate(),
    });

    window.localStorage.nomadTokenSecret = longDeadToken.secretId;
    const notFoundServerError = {
      errors: [
        {
          detail: 'ACL token not found',
        },
      ],
    };
    server.pretender.get('/v1/jobs', function () {
      return [500, {}, JSON.stringify(notFoundServerError)];
    });

    await Jobs.visit();
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Redirected to tokens page due to a token not being found'
    );
  });

  test('it notifies you when your token has 10 minutes remaining', async function (assert) {
    let notificationRendered = assert.async();
    let notificationNotRendered = assert.async();
    window.localStorage.clear();
    assert.equal(
      window.localStorage.nomadTokenSecret,
      null,
      'No token secret set'
    );
    assert.timeout(6000);
    const nearlyExpiringToken = server.create('token', {
      name: 'Not quite dead yet',
      expirationTime: moment().add(10, 'm').add(3, 's').toDate(),
    });

    await Tokens.visit();

    // Ember Concurrency makes testing iterations convoluted: https://ember-concurrency.com/docs/testing-debugging/
    // Waiting for half a second to validate that there's no warning;
    // then a further 5 seconds to validate that there is a warning, and to explicitly cancelAllTimers(),
    // short-circuiting our Ember Concurrency loop.
    setTimeout(() => {
      assert
        .dom('.flash-message.alert-warning')
        .doesNotExist('No notification yet for a token with 10m5s left');
      notificationNotRendered();
      setTimeout(async () => {
        await percySnapshot(assert, {
          percyCSS: '[data-test-expiration-timestamp] { display: none; }',
        });

        assert
          .dom('.flash-message.alert-warning')
          .exists('Notification is rendered at the 10m mark');
        notificationRendered();
        run.cancelTimers();
      }, 5000);
    }, 500);
    await Tokens.secret(nearlyExpiringToken.secretId).submit();
  });

  test('when the ott query parameter is present upon application load it’s exchanged for a token', async function (assert) {
    const { oneTimeSecret, secretId } = managementToken;

    await JobDetail.visit({ id: job.id, ott: oneTimeSecret });

    assert.notOk(
      currentURL().includes(oneTimeSecret),
      'OTT is cleared from the URL after loading'
    );

    await Tokens.visit();

    assert.equal(
      window.localStorage.nomadTokenSecret,
      secretId,
      'Token secret was set'
    );
  });

  test('SSO Sign-in flow: Manager', async function (assert) {
    server.create('auth-method', { name: 'vault' });
    server.create('auth-method', { name: 'cognito' });
    server.create('token', { name: 'Thelonious' });

    await Tokens.visit();
    assert.dom('[data-test-auth-method]').exists({ count: 2 });
    await click('button[data-test-auth-method]');
    assert.ok(currentURL().startsWith('/oidc-mock'));
    let managerButton = [...findAll('button')].filter((btn) =>
      btn.textContent.includes('Sign In as Manager')
    )[0];

    assert.dom(managerButton).exists();
    await click(managerButton);

    await percySnapshot(assert);

    assert.ok(currentURL().startsWith('/settings/tokens'));
    assert.dom('[data-test-token-name]').includesText('Token: Manager');
  });

  test('SSO Sign-in flow: Regular User', async function (assert) {
    server.create('auth-method', { name: 'vault' });
    server.create('token', { name: 'Thelonious' });

    await Tokens.visit();
    assert.dom('[data-test-auth-method]').exists({ count: 1 });
    await click('button[data-test-auth-method]');
    assert.ok(currentURL().startsWith('/oidc-mock'));
    let newTokenButton = [...findAll('button')].filter((btn) =>
      btn.textContent.includes('Sign In as Thelonious')
    )[0];
    assert.dom(newTokenButton).exists();
    await click(newTokenButton);

    assert.ok(currentURL().startsWith('/settings/tokens'));
    assert.dom('[data-test-token-name]').includesText('Token: Thelonious');
  });

  test('It shows an error on failed SSO', async function (assert) {
    server.create('auth-method', { name: 'vault' });
    await visit('/settings/tokens?state=failure');
    assert.ok(Tokens.ssoErrorMessage);
    await Tokens.clearSSOError();
    assert.equal(currentURL(), '/settings/tokens', 'State query param cleared');
    assert.notOk(Tokens.ssoErrorMessage);

    await click('button[data-test-auth-method]');
    assert.ok(currentURL().startsWith('/oidc-mock'));

    let failureButton = find('.button.error');
    assert.dom(failureButton).exists();
    await click(failureButton);
    assert.equal(
      currentURL(),
      '/settings/tokens?state=failure',
      'Redirected with failure state'
    );

    await percySnapshot(assert);
    assert.ok(Tokens.ssoErrorMessage);
  });

  test('JWT Sign-in flow: OIDC methods only', async function (assert) {
    server.create('auth-method', { name: 'Vault', type: 'OIDC' });
    server.create('auth-method', { name: 'Auth0', type: 'OIDC' });
    await Tokens.visit();
    assert
      .dom('[data-test-auth-method]')
      .exists({ count: 2 }, 'Both OIDC methods shown');
    assert
      .dom('label[for="token-input"]')
      .hasText(
        'Secret ID',
        'Secret ID input shown without JWT info when no such method exists'
      );
  });

  test('JWT Sign-in flow: JWT method', async function (assert) {
    server.create('auth-method', { name: 'Vault', type: 'OIDC' });
    server.create('auth-method', { name: 'Auth0', type: 'OIDC' });
    server.create('auth-method', { name: 'JWT-Local', type: 'JWT' });
    await Tokens.visit();
    assert
      .dom('[data-test-auth-method]')
      .exists(
        { count: 2 },
        'The newly added JWT method does not add a 3rd Auth Method button'
      );
    assert
      .dom('label[for="token-input"]')
      .hasText('Secret ID or JWT', 'Secret ID input now shows JWT info');

    // Expect to be signed in as a manager
    await Tokens.secret(
      'aaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.management'
    ).submit();
    assert.ok(currentURL().startsWith('/settings/tokens'));
    assert.dom('[data-test-token-name]').includesText('Token: Manager');
    await Tokens.clear();

    // Expect to be signed in as a client
    await Tokens.secret(
      'aaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.whateverlol'
    ).submit();
    assert.ok(currentURL().startsWith('/settings/tokens'));
    assert.dom('[data-test-token-name]').includesText(
      `Token: ${
        server.db.tokens.filter((token) => {
          return token.type === 'client';
        })[0].name
      }`
    );
    await Tokens.clear();

    // Expect to an error on bad JWT
    await Tokens.secret(
      'aaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.bad'
    ).submit();
    assert.ok(currentURL().startsWith('/settings/tokens'));
    assert.dom('[data-test-token-error]').exists();
  });

  test('JWT Sign-in flow: JWT Method Selector, Single JWT', async function (assert) {
    server.create('auth-method', { name: 'Vault', type: 'OIDC' });
    server.create('auth-method', { name: 'Auth0', type: 'OIDC' });
    server.create('auth-method', { name: 'JWT-Local', type: 'JWT' });
    await Tokens.visit();
    assert
      .dom('[data-test-token-submit]')
      .exists(
        { count: 1 },
        'Submit token/JWT button exists with only a single JWT '
      );
    assert
      .dom('[data-test-token-submit]')
      .hasText(
        'Sign in with secret',
        'Submit token/JWT button has correct text with only a single JWT '
      );
    await Tokens.secret('very-short-secret');
    assert
      .dom('[data-test-token-submit]')
      .hasText(
        'Sign in with secret',
        'A short secret still shows the "secret" verbiage on the button'
      );
    await Tokens.secret(
      'aaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.whateverlol'
    );
    assert
      .dom('[data-test-token-submit]')
      .hasText(
        'Sign in with JWT',
        'A JWT-shaped secret will change button text to reflect JWT sign-in'
      );

    assert
      .dom('[data-test-select-jwt]')
      .doesNotExist('No JWT selector shown with only a single method');
  });

  test('JWT Sign-in flow: JWT Method Selector, Multiple JWT', async function (assert) {
    server.create('auth-method', { name: 'Vault', type: 'OIDC' });
    server.create('auth-method', { name: 'Auth0', type: 'OIDC' });
    server.create('auth-method', {
      name: 'JWT-Local',
      type: 'JWT',
      default: false,
    });
    server.create('auth-method', {
      name: 'JWT-Regional',
      type: 'JWT',
      default: false,
    });
    server.create('auth-method', {
      name: 'JWT-Global',
      type: 'JWT',
      default: true,
    });
    await Tokens.visit();
    assert
      .dom('[data-test-token-submit]')
      .exists(
        { count: 1 },
        'Submit token/JWT button exists with only a single JWT '
      );
    assert
      .dom('[data-test-select-jwt]')
      .doesNotExist('No JWT selector shown with an empty token/secret');
    await Tokens.secret(
      'aaaaaaaaaaaaaaaaaaaaaaaaa.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.whateverlol'
    );
    assert
      .dom('[data-test-select-jwt]')
      .exists({ count: 1 }, 'JWT selector shown with multiple JWT methods');

    assert.equal(
      currentURL(),
      '/settings/tokens?jwtAuthMethod=JWT-Global',
      'Default JWT method is selected'
    );
    await clickTrigger('[data-test-select-jwt]');
    assert.dom('.dropdown-options').exists('Dropdown options are shown');

    await selectChoose('[data-test-select-jwt]', 'JWT-Regional');
    console.log(currentURL());
    assert.equal(
      currentURL(),
      '/settings/tokens?jwtAuthMethod=JWT-Regional',
      'Selected JWT method is shown'
    );
  });

  test('when the ott exchange fails an error is shown', async function (assert) {
    await visit('/?ott=fake');

    assert.ok(Layout.error.isPresent);
    assert.equal(Layout.error.title, 'Token Exchange Error');
    assert.equal(
      Layout.error.message,
      'Failed to exchange the one-time token.'
    );
  });

  test('Tokens are shown on the policies index page', async function (assert) {
    allScenarios.policiesTestCluster(server);
    // Create an expired token
    server.create('token', {
      name: 'Expired Token',
      id: 'just-expired',
      policyIds: [server.db.policies[0].name],
      expirationTime: new Date(new Date().getTime() - 10 * 60 * 1000), // 10 minutes ago
    });

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');
    assert.dom('[data-test-policy-token-count]').exists();
    const expectedFirstPolicyTokens = server.db.tokens.filter((token) => {
      return token.policyIds.includes(server.db.policies[0].name);
    });
    assert
      .dom('[data-test-policy-total-tokens]')
      .hasText(expectedFirstPolicyTokens.length.toString());
    assert.dom('[data-test-policy-expired-tokens]').hasText('(1 expired)');
    window.localStorage.nomadTokenSecret = null;
  });

  test('Tokens are shown on a policy page', async function (assert) {
    allScenarios.policiesTestCluster(server);
    // Create an expired token
    server.create('token', {
      name: 'Expired Token',
      id: 'just-expired',
      policyIds: [server.db.policies[0].name],
      expirationTime: new Date(new Date().getTime() - 10 * 60 * 1000), // 10 minutes ago
    });

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');

    await click('[data-test-policy-row]:first-child');
    assert.equal(currentURL(), `/policies/${server.db.policies[0].name}`);

    const expectedFirstPolicyTokens = server.db.tokens.filter((token) => {
      return token.policyIds.includes(server.db.policies[0].name);
    });

    assert
      .dom('[data-test-policy-token-row]')
      .exists(
        { count: expectedFirstPolicyTokens.length },
        'Expected number of tokens are shown'
      );
    assert.dom('[data-test-token-expiration-time]').hasText('10 minutes ago');

    window.localStorage.nomadTokenSecret = null;
  });

  test('Tokens Deletion', async function (assert) {
    allScenarios.policiesTestCluster(server);
    const testPolicy = server.db.policies[0];
    const existingTokens = server.db.tokens.filter((t) =>
      t.policyIds.includes(testPolicy.name)
    );
    // Create an expired token
    server.create('token', {
      name: 'Doomed Token',
      id: 'enjoying-my-day-here',
      policyIds: [testPolicy.name],
    });

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');

    await click('[data-test-policy-row]:first-child');
    assert.equal(currentURL(), `/policies/${testPolicy.name}`);
    assert
      .dom('[data-test-policy-token-row]')
      .exists(
        { count: existingTokens.length + 1 },
        'Expected number of tokens are shown'
      );

    const doomedTokenRow = [...findAll('[data-test-policy-token-row]')].find(
      (a) => a.textContent.includes('Doomed Token')
    );

    assert.dom(doomedTokenRow).exists();

    await click(doomedTokenRow.querySelector('button'));
    assert
      .dom(doomedTokenRow.querySelector('[data-test-confirm-button]'))
      .exists();
    await click(doomedTokenRow.querySelector('[data-test-confirm-button]'));
    assert.dom('.flash-message.alert-success').exists();
    assert
      .dom('[data-test-policy-token-row]')
      .exists(
        { count: existingTokens.length },
        'One fewer token after deletion'
      );
    await percySnapshot(assert);
    window.localStorage.nomadTokenSecret = null;
  });

  test('Test Token Creation', async function (assert) {
    allScenarios.policiesTestCluster(server);
    const testPolicy = server.db.policies[0];
    const existingTokens = server.db.tokens.filter((t) =>
      t.policyIds.includes(testPolicy.name)
    );

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/policies');

    await click('[data-test-policy-row]:first-child');
    assert.equal(currentURL(), `/policies/${testPolicy.name}`);

    assert
      .dom('[data-test-policy-token-row]')
      .exists(
        { count: existingTokens.length },
        'Expected number of tokens are shown'
      );

    await click('[data-test-create-test-token]');
    assert.dom('.flash-message.alert-success').exists();
    assert
      .dom('[data-test-policy-token-row]')
      .exists(
        { count: existingTokens.length + 1 },
        'One more token after test token creation'
      );
    assert
      .dom('[data-test-policy-token-row]:last-child [data-test-token-name]')
      .hasText(`Example Token for ${testPolicy.name}`);
    await percySnapshot(assert);
    window.localStorage.nomadTokenSecret = null;
  });

  function getHeader({ requestHeaders }, name) {
    // Headers are case-insensitive, but object property look up is not
    return (
      requestHeaders[name] ||
      requestHeaders[name.toLowerCase()] ||
      requestHeaders[name.toUpperCase()]
    );
  }
});
