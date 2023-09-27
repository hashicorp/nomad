/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* eslint-disable qunit/require-expect */
import {
  currentURL,
  find,
  findAll,
  visit,
  click,
  fillIn,
} from '@ember/test-helpers';
import { module, skip, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import Jobs from 'nomad-ui/tests/pages/jobs/list';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import ClientDetail from 'nomad-ui/tests/pages/clients/detail';
import Layout from 'nomad-ui/tests/pages/layout';
import AccessControl from 'nomad-ui/tests/pages/access-control';
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
    assert.ok(document.title.includes('Sign In'));

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
      expirationTime: moment().add(10, 'm').add(5, 's').toDate(),
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

  test('Tokens are shown on the Access Control Policies index page', async function (assert) {
    allScenarios.policiesTestCluster(server);
    let firstPolicy = server.db.policies.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];
    // Create an expired token
    server.create('token', {
      name: 'Expired Token',
      id: 'just-expired',
      policyIds: [firstPolicy.name],
      expirationTime: new Date(new Date().getTime() - 10 * 60 * 1000), // 10 minutes ago
    });

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    assert.dom('[data-test-policy-total-tokens]').exists();
    const expectedFirstPolicyTokens = server.db.tokens.filter((token) => {
      return token.policyIds.includes(firstPolicy.name);
    });
    assert
      .dom('[data-test-policy-total-tokens]')
      .hasText(expectedFirstPolicyTokens.length.toString());
    assert.dom('[data-test-policy-expired-tokens]').hasText('(1 expired)');
    window.localStorage.nomadTokenSecret = null;
  });

  test('Tokens are shown on a policy page', async function (assert) {
    allScenarios.policiesTestCluster(server);
    let firstPolicy = server.db.policies.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];

    // Create an expired token
    server.create('token', {
      name: 'Expired Token',
      id: 'just-expired',
      policyIds: [firstPolicy.name],
      expirationTime: new Date(new Date().getTime() - 10 * 60 * 1000), // 10 minutes ago
    });

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');
    await click('[data-test-policy-name]');
    assert.equal(currentURL(), `/access-control/policies/${firstPolicy.name}`);

    const expectedFirstPolicyTokens = server.db.tokens.filter((token) => {
      return token.policyIds.includes(firstPolicy.name);
    });

    assert
      .dom('[data-test-policy-token-row]')
      .exists(
        { count: expectedFirstPolicyTokens.length },
        'Expected number of tokens are shown'
      );

    const expiredTokenRow = [...findAll('[data-test-policy-token-row]')].find(
      (a) => a.textContent.includes('Expired Token')
    );

    assert.dom(expiredTokenRow).exists();
    assert
      .dom(expiredTokenRow.querySelector('[data-test-token-expiration-time]'))
      .hasText('10 minutes ago');

    window.localStorage.nomadTokenSecret = null;
  });

  test('Tokens Deletion from Policy page', async function (assert) {
    allScenarios.policiesTestCluster(server);
    let testPolicy = server.db.policies.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];

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
    await visit('/access-control/policies');

    await click('[data-test-policy-name]:first-child');
    assert.equal(currentURL(), `/access-control/policies/${testPolicy.name}`);
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

  test('Test Token Creation from Policy Page', async function (assert) {
    allScenarios.policiesTestCluster(server);
    let testPolicy = server.db.policies.sort((a, b) => {
      return a.name.localeCompare(b.name);
    })[0];

    const existingTokens = server.db.tokens.filter((t) =>
      t.policyIds.includes(testPolicy.name)
    );

    window.localStorage.nomadTokenSecret = server.db.tokens[0].secretId;
    await visit('/access-control/policies');

    await click('[data-test-policy-name]');
    assert.equal(currentURL(), `/access-control/policies/${testPolicy.name}`);

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

  module('Roles', function (hooks) {
    // Set up a token with a role
    hooks.beforeEach(function () {
      window.localStorage.clear();
      window.sessionStorage.clear();
      faker.seed(1);
      allScenarios.rolesTestCluster(server);
    });

    test('Policies are derived from role', async function (assert) {
      assert.expect(19);

      await Tokens.visit();

      let token;

      // User with 1 role, containing 1 policy, and no direct policies
      token = server.db.tokens.findBy(
        (t) => t.name === 'High Level Role Token'
      );
      await Tokens.secret(token.secretId).submit();

      assert.dom('[data-test-token-role]').exists({ count: 1 });
      assert.dom('[data-test-role-name]').hasText('high-level');
      assert.dom('[data-test-role-policies] li').exists({ count: 1 });
      assert.dom('[data-test-role-policies] li').hasText('job-writer');

      assert.dom('[data-test-token-policy]').exists({ count: 1 });
      assert.dom('[data-test-policy-name]').hasText('job-writer');

      await Tokens.clear();

      // User with 1 role, containing 2 policies, and a direct policy
      token = server.db.tokens.findBy(
        (t) => t.name === 'Policy And Role Token'
      );
      await Tokens.secret(token.secretId).submit();

      assert.dom('[data-test-token-role]').exists({ count: 1 });
      assert.dom('[data-test-role-name]').hasText('reader');
      assert.dom('[data-test-role-policies] li').exists({ count: 2 });
      let policyLinks = findAll('[data-test-role-policies] li');
      assert.dom(policyLinks[0]).hasText('client-reader');
      assert.dom(policyLinks[1]).hasText('job-reader');

      assert.dom('[data-test-token-policy]').exists({ count: 3 });
      let policyBlocks = findAll('[data-test-policy-name]');
      assert.dom(policyBlocks[0]).hasText('operator');
      assert.dom(policyBlocks[1]).hasText('client-reader');
      assert.dom(policyBlocks[2]).hasText('job-reader');

      await percySnapshot(assert);

      await Tokens.clear();

      // User with 2 roles, each containing 1 policy, and one of the policies is also directly on their token
      token = server.db.tokens.findBy(
        (t) => t.name === 'Multi Role And Policy Token'
      );
      await Tokens.secret(token.secretId).submit();

      assert.equal(token.roleIds.length, 2);
      assert.equal(token.policyIds.length, 1);

      assert.dom('[data-test-token-role]').exists({ count: 2 });
      assert.dom('[data-test-token-policy]').exists({ count: 2 });
    });

    test('Token priveleges are derived from role', async function (assert) {
      // First, check that a node reader can read nodes if the policy to do so only exists at their role level
      await visit('/clients');
      // Expect to see some nodes
      let nodes = server.db.nodes;
      assert.dom('[data-test-client-node-row]').exists({ count: nodes.length });

      // Head back and sign in as Clientless Role Token
      await Tokens.visit();
      let token = server.db.tokens.findBy(
        (t) => t.name === 'Clientless Role Token'
      );
      await Tokens.secret(token.secretId).submit();

      await visit('/clients');
      // Expect no rows, and a denied message
      assert.dom('[data-test-client-node-row]').doesNotExist();
      assert.dom('[data-test-error]').exists();

      // Pop over to the jobs page and make sure the Run button is disabled
      await visit('/jobs');
      assert.dom('[data-test-run-job]').hasTagName('button');
      assert.dom('[data-test-run-job]').isDisabled();

      // Sign out, and sign back in as a high-level role token
      await Tokens.visit();
      await Tokens.clear();
      token = server.db.tokens.findBy(
        (t) => t.name === 'High Level Role Token'
      );
      await Tokens.secret(token.secretId).submit();

      await visit('/jobs');
      // Expect the Run button/link to work now
      assert.dom('[data-test-run-job]').hasTagName('a');
      assert.dom('[data-test-run-job]').hasAttribute('href', '/ui/jobs/run');
    });
  });

  module('Access Control Tokens section', function (hooks) {
    hooks.beforeEach(async function () {
      window.localStorage.clear();
      window.sessionStorage.clear();
      faker.seed(1);
      allScenarios.rolesTestCluster(server);
      await Tokens.visit();
      const managementToken = server.db.tokens.findBy(
        (t) => t.type === 'management'
      );
      const { secretId } = managementToken;
      await Tokens.secret(secretId).submit();
      await AccessControl.visitTokens();
    });

    hooks.afterEach(async function () {
      await Tokens.visit();
      await Tokens.clear();
    });

    test('Tokens index, general', async function (assert) {
      assert.equal(currentURL(), '/access-control/tokens');
      // Number of token rows equivalent to number in db
      assert
        .dom('[data-test-token-row]')
        .exists({ count: server.db.tokens.length });

      await percySnapshot(assert);
    });

    test('Tokens index, management token handling', async function (assert) {
      // two management tokens, one of which is yours; yours cannot be deleted or clicked into.
      assert.dom('[data-test-token-type="management"]').exists({ count: 2 });
      const managementToken = server.db.tokens.findBy(
        (t) => t.type === 'management'
      );
      const managementTokenRow = [...findAll('[data-test-token-row]')].find(
        (row) => row.textContent.includes(managementToken.name)
      );
      const otherManagerRow = [...findAll('[data-test-token-row]')].find(
        (row) =>
          row.textContent.includes('management') &&
          !row.textContent.includes(managementToken.name)
      );
      assert
        .dom(managementTokenRow.querySelector('[data-test-token-name] a'))
        .doesNotExist('Cannot click into and edit your own token');
      assert
        .dom(otherManagerRow.querySelector('[data-test-token-name] a'))
        .exists('Can click into and edit another manager token');
      assert
        .dom(
          managementTokenRow.querySelector('[data-test-delete-token] button')
        )
        .isDisabled('Cannot delete your own token');
      assert
        .dom(otherManagerRow.querySelector('[data-test-delete-token] button'))
        .isNotDisabled('Can delete another manager token');
    });

    test('Tokens index, table sorting', async function (assert) {
      const nameCells = findAll('[data-test-token-name]');
      const nameCellText = nameCells.map((cell) => cell.textContent.trim());
      const sortedNameCellText = nameCellText.slice().sort();
      assert.deepEqual(
        nameCellText,
        sortedNameCellText,
        'Names are sorted alphabetically'
      );

      // Click on the first thead tr th to reverse
      assert
        .dom('table.acl-table thead tr th')
        .hasAttribute('aria-sort', 'ascending');
      await click('table.acl-table thead tr th button');
      assert
        .dom('table.acl-table thead tr th')
        .hasAttribute('aria-sort', 'descending');

      const reversedNameCells = findAll('[data-test-token-name]');
      const reversedNameCellText = reversedNameCells.map((cell) =>
        cell.textContent.trim()
      );
      const reversedSortedNameCellText = nameCellText.slice().sort().reverse();

      assert.deepEqual(
        reversedNameCellText,
        reversedSortedNameCellText,
        'Names are reversed alphabetically'
      );
    });

    test('Tokens index, deletion', async function (assert) {
      const numberOfTokens = server.db.tokens.length;
      assert
        .dom('[data-test-token-row]')
        .exists(
          { count: numberOfTokens },
          'Number of tokens matches number in db'
        );
      const tokenToDelete = server.db.tokens.findBy((t) => t.type === 'client');
      const tokenRowToDelete = [...findAll('[data-test-token-row]')].find(
        (row) => row.textContent.includes(tokenToDelete.name)
      );
      await click(
        tokenRowToDelete.querySelector('[data-test-delete-token] button')
      );
      assert.dom('.flash-message.alert-success').exists();
      assert
        .dom('[data-test-token-row]')
        .exists(
          { count: numberOfTokens - 1 },
          'Number of token rows decreased after deletion'
        );

      const nameCells = findAll('[data-test-token-name]');
      const nameCellText = nameCells.map((cell) => cell.textContent.trim());
      assert.notOk(
        nameCellText.includes(tokenToDelete.name),
        'Deleted token name not found among name cells'
      );
    });

    test('Tokens index, clicking into a token page', async function (assert) {
      const tokenToClick = server.db.tokens.findBy((t) => t.type === 'client');
      const tokenRowToClick = [...findAll('[data-test-token-row]')].find(
        (row) => row.textContent.includes(tokenToClick.name)
      );
      await click(tokenRowToClick.querySelector('[data-test-token-name] a'));
      assert.equal(currentURL(), `/access-control/tokens/${tokenToClick.id}`);
      assert.dom('[data-test-token-name-input]').hasValue(tokenToClick.name);
    });

    test('Tokens index, roles and policies attached to a token show up as links', async function (assert) {
      // Staying on the index page, Rows should have a Roles column with either "No Roles" or a bunch of links to roles. Ditto policies.
      const tokenWithRolesAndPolicies = server.db.tokens.findBy(
        (t) => t.name === 'Multi Role And Policy Token'
      );
      const tokenRowWithRolesAndPolicies = [
        ...findAll('[data-test-token-row]'),
      ].find((row) => row.textContent.includes(tokenWithRolesAndPolicies.name));
      const rolesCell = tokenRowWithRolesAndPolicies.querySelector(
        '[data-test-token-roles]'
      );
      const policiesCell = tokenRowWithRolesAndPolicies.querySelector(
        '[data-test-token-policies]'
      );
      assert.dom(rolesCell).exists();
      assert.dom(policiesCell).exists();

      const rolesCellTags = rolesCell
        .querySelector('.tag-group')
        .querySelectorAll('span');
      const policiesCellTags = policiesCell
        .querySelector('.tag-group')
        .querySelectorAll('span');
      assert.equal(rolesCellTags.length, 2);
      assert.equal(policiesCellTags.length, 1);

      const policyLessToken = server.db.tokens.findBy(
        (t) => t.name === 'High Level Role Token'
      );
      const policyLessTokenRow = [...findAll('[data-test-token-row]')].find(
        (row) => row.textContent.includes(policyLessToken.name)
      );
      const rolesCell2 = policyLessTokenRow.querySelector(
        '[data-test-token-roles]'
      );
      const policiesCell2 = policyLessTokenRow.querySelector(
        '[data-test-token-policies]'
      );
      assert.dom(rolesCell2).exists();
      assert.dom(policiesCell2).exists();

      const rolesCellTags2 = rolesCell2
        .querySelector('.tag-group')
        .querySelectorAll('span');
      const policiesCellTags2 = policiesCell2
        .querySelector('.tag-group')
        .querySelectorAll('span');
      assert.equal(rolesCellTags2.length, 1);
      assert.equal(policiesCellTags2.length, 0);
    });

    test('Token page, general', async function (assert) {
      const token = server.db.tokens.findBy((t) => t.id === 'cl4y-t0k3n');
      await visit(`/access-control/tokens/${token.id}`);
      assert.dom('[data-test-token-name-input]').hasValue(token.name);
      assert.dom('[data-test-token-accessor]').hasValue(token.accessorId);
      assert.dom('[data-test-token-secret]').hasValue(token.secretId);
      assert.dom('[data-test-token-type="client"]').isChecked();
      assert.dom('[data-test-token-type="management"]').isNotChecked();

      assert.dom('.expiration-time').hasText('Token expires in an hour');

      assert.dom('[data-test-token-roles]').exists();
      assert.dom('[data-test-token-policies]').exists();

      // All possible policies are shown
      const allPolicies = server.db.policies;
      const allPolicyRows = findAll('[data-test-token-policies] tbody tr');
      assert.equal(
        allPolicyRows.length,
        allPolicies.length,
        'All policies are shown'
      );

      // The policies/roles belonging to this token are checked
      const tokenPolicies = token.policyIds;

      const checkedPolicyRows = findAll(
        '[data-test-token-policies] tbody tr input:checked'
      );

      assert.equal(
        checkedPolicyRows.length,
        tokenPolicies.length,
        'All policies belonging to this token are checked'
      );

      const checkedPolicyNames = checkedPolicyRows.map((row) =>
        row
          .closest('tr')
          .querySelector('[data-test-policy-name]')
          .textContent.trim()
      );
      assert.deepEqual(
        checkedPolicyNames.sort(),
        tokenPolicies.sort(),
        'All policies belonging to this token are checked'
      );

      const allRoles = server.db.roles;
      const allRoleRows = findAll('[data-test-token-roles] tbody tr');
      assert.equal(allRoleRows.length, allRoles.length, 'All roles are shown');

      const tokenRoles = token.roleIds;

      const checkedRoleRows = findAll(
        '[data-test-token-roles] tbody tr input:checked'
      );

      assert.equal(
        checkedRoleRows.length,
        tokenRoles.length,
        'All roles belonging to this token are checked'
      );

      const checkedRoleNames = checkedRoleRows.map((row) =>
        row
          .closest('tr')
          .querySelector('[data-test-role-name]')
          .textContent.trim()
      );

      assert.deepEqual(
        checkedRoleNames.sort(),
        tokenRoles.sort(),
        'All roles belonging to this token are checked'
      );
    });
    test('Token name can be edited', async function (assert) {
      const token = server.db.tokens.findBy((t) => t.id === 'cl4y-t0k3n');
      await visit(`/access-control/tokens/${token.id}`);
      assert.dom('[data-test-token-name-input]').hasValue(token.name);
      await fillIn('[data-test-token-name-input]', 'Mud-Token');
      await click('[data-test-token-save]');
      assert.dom('.flash-message.alert-success').exists();
      await AccessControl.visitTokens();
      assert.dom('[data-test-token-name="Mud-Token"]').exists({ count: 1 });
    });

    test('Token policies and roles can be edited', async function (assert) {
      const token = server.db.tokens.findBy((t) => t.id === 'cl4y-t0k3n');
      await visit(`/access-control/tokens/${token.id}`);

      // The policies/roles belonging to this token are checked
      const tokenPolicies = token.policyIds;

      const checkedPolicyRows = findAll(
        '[data-test-token-policies] tbody tr input:checked'
      );

      assert.equal(
        checkedPolicyRows.length,
        tokenPolicies.length,
        'All policies belonging to this token are checked'
      );

      const checkedPolicyNames = checkedPolicyRows.map((row) =>
        row
          .closest('tr')
          .querySelector('[data-test-policy-name]')
          .textContent.trim()
      );
      assert.deepEqual(
        checkedPolicyNames.sort(),
        tokenPolicies.sort(),
        'All policies belonging to this token are checked'
      );

      // Try unchecking ALL checked roles and policies and saving
      // First, find all checked ones
      const checkedPolicies = findAll(
        '[data-test-token-policies] tbody tr input:checked'
      );
      const checkedRoles = findAll(
        '[data-test-token-roles] tbody tr input:checked'
      );
      // Then uncheck them
      checkedPolicies.forEach((policy) => {
        policy.click();
      });
      checkedRoles.forEach((role) => {
        role.click();
      });
      await click('[data-test-token-save]');
      assert.dom('.flash-message.alert-critical').exists();

      // Try selecting a single role
      await click('[data-test-token-roles] tbody tr input');
      await click('[data-test-token-save]');
      assert.dom('.flash-message.alert-success').exists();

      await percySnapshot(assert);

      await AccessControl.visitTokens();
      // Policies cell for our clay token should read "No Policies"
      const clayToken = server.db.tokens.findBy((t) => t.id === 'cl4y-t0k3n');
      const clayTokenRow = [...findAll('[data-test-token-row]')].find((row) =>
        row.textContent.includes(clayToken.name)
      );
      const policiesCell = clayTokenRow.querySelector(
        '[data-test-token-policies]'
      );
      assert.dom(policiesCell).exists();
      assert.dom(policiesCell).hasText('No Policies');

      // Roles cell should have 1 tag
      const rolesCell = clayTokenRow.querySelector('[data-test-token-roles]');
      const rolesCellTags = rolesCell
        .querySelector('.tag-group')
        .querySelectorAll('span');
      assert.equal(rolesCellTags.length, 1);
    });
    test('Token can be deleted', async function (assert) {
      const token = server.db.tokens.findBy((t) => t.id === 'cl4y-t0k3n');
      await visit(`/access-control/tokens/${token.id}`);
      await click('[data-test-delete-token]');
      assert.dom('.flash-message.alert-success').exists();
      await AccessControl.visitTokens();
      assert.dom('[data-test-token-name="cl4y-t0k3n"]').doesNotExist();
    });
    test('New Token creation', async function (assert) {
      await click('[data-test-create-token]');
      assert.equal(currentURL(), '/access-control/tokens/new');
      await fillIn('[data-test-token-name-input]', 'Timeless Token');
      await click('[data-test-token-save]');
      assert.dom('.flash-message.alert-success').exists();
      await AccessControl.visitTokens();
      assert
        .dom('[data-test-token-name="Timeless Token"]')
        .exists({ count: 1 });
      const newTokenRow = [...findAll('[data-test-token-row]')].find((row) =>
        row.textContent.includes('Timeless Token')
      );
      const newTokenExpirationCell = newTokenRow.querySelector(
        '[data-test-token-expiration-time]'
      );
      assert.dom(newTokenExpirationCell).hasText('Never');

      // Now create one with a TTL
      await click('[data-test-create-token]');
      assert.equal(currentURL(), '/access-control/tokens/new');
      await fillIn('[data-test-token-name-input]', 'TTL Token');
      // Select the "8 hours" radio within the .expiration-time div
      await click('.expiration-time input[value="8h"]');
      await click('[data-test-token-save]');
      assert.dom('.flash-message.alert-success').exists();
      await AccessControl.visitTokens();
      assert.dom('[data-test-token-name="TTL Token"]').exists({ count: 1 });
      const ttlTokenRow = [...findAll('[data-test-token-row]')].find((row) =>
        row.textContent.includes('TTL Token')
      );
      const ttlTokenExpirationCell = ttlTokenRow.querySelector(
        '[data-test-token-expiration-time]'
      );
      assert.dom(ttlTokenExpirationCell).hasText('in 8 hours');

      // Now create one with an expiration time
      await click('[data-test-create-token]');
      assert.equal(currentURL(), '/access-control/tokens/new');
      await fillIn('[data-test-token-name-input]', 'Expiring Token');
      // select the Custom radio button
      await click('.expiration-time input[value="custom"]');
      assert
        .dom('[data-test-token-expiration-time-input]')
        .exists('HTML datetime-local picker exists');
      await percySnapshot(assert);
      // select a date/time for 100 minutes into the future in GMT
      const soon = new Date();
      soon.setMinutes(soon.getMinutes() + 100);
      var tzoffset = new Date().getTimezoneOffset() * 60000; //offset in milliseconds
      var soonString = new Date(soon - tzoffset).toISOString().slice(0, -1);
      await fillIn('[data-test-token-expiration-time-input]', soonString);
      await click('[data-test-token-save]');
      assert.dom('.flash-message.alert-success').exists();
      await AccessControl.visitTokens();
      assert
        .dom('[data-test-token-name="Expiring Token"]')
        .exists({ count: 1 });
      const expiringTokenRow = [...findAll('[data-test-token-row]')].find(
        (row) => row.textContent.includes('Expiring Token')
      );
      const expiringTokenExpirationCell = expiringTokenRow.querySelector(
        '[data-test-token-expiration-time]'
      );
      assert
        .dom(expiringTokenExpirationCell)
        .hasText('in 2 hours', 'Expiration time is relativized and rounded');
    });
  });
});
