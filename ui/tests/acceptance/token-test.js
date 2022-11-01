/* eslint-disable qunit/require-expect */
import { currentURL, find, visit, click } from '@ember/test-helpers';
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
    assert.equal(document.title, 'Tokens - Nomad');

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
    await Tokens.clear();
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
      .dom('.flash-message.alert-error button')
      .exists('A global alert exists and has a clickable button');

    await click('.flash-message.alert-error button');
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
      console.log('uhhhh');
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
    await Tokens.clear();
    // Ember Concurrency makes testing iterations convoluted: https://ember-concurrency.com/docs/testing-debugging/
    // Waiting for half a second to validate that there's no warning;
    // then a further 5 seconds to validate that there is a warning, and to explicitly cancelAllTimers(),
    // short-circuiting our Ember Concurrency loop.
    setTimeout(() => {
      assert
        .dom('.flash-message.alert-error')
        .doesNotExist('No notification yet for a token with 10m5s left');
      notificationNotRendered();
      setTimeout(() => {
        assert
          .dom('.flash-message.alert-error')
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

  test('when the ott exchange fails an error is shown', async function (assert) {
    await visit('/?ott=fake');

    assert.ok(Layout.error.isPresent);
    assert.equal(Layout.error.title, 'Token Exchange Error');
    assert.equal(
      Layout.error.message,
      'Failed to exchange the one-time token.'
    );
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
