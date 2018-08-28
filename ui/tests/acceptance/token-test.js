import { find } from 'ember-native-dom-helpers';
import { test, skip } from 'ember-qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';
import Jobs from 'nomad-ui/tests/pages/jobs/list';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import ClientDetail from 'nomad-ui/tests/pages/clients/detail';

let job;
let node;
let managementToken;
let clientToken;

moduleForAcceptance('Acceptance | tokens', {
  beforeEach() {
    server.create('agent');
    node = server.create('node');
    job = server.create('job');
    managementToken = server.create('token');
    clientToken = server.create('token');
  },
});

test('the token form sets the token in local storage', function(assert) {
  const { secretId } = managementToken;

  Tokens.visit();

  andThen(() => {
    assert.ok(window.localStorage.nomadTokenSecret == null, 'No token secret set');

    Tokens.secret(secretId).submit();

    andThen(() => {
      assert.equal(window.localStorage.nomadTokenSecret, secretId, 'Token secret was set');
    });
  });
});

// TODO: unskip once store.unloadAll reliably waits for in-flight requests to settle
skip('the X-Nomad-Token header gets sent with requests once it is set', function(assert) {
  const { secretId } = managementToken;
  let requestPosition = 0;

  JobDetail.visit({ id: job.id });
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    assert.ok(server.pretender.handledRequests.length > 1, 'Requests have been made');

    server.pretender.handledRequests.forEach(req => {
      assert.notOk(getHeader(req, 'X-Nomad-Token'), `No token for ${req.url}`);
    });

    requestPosition = server.pretender.handledRequests.length;
  });

  Tokens.visit();

  andThen(() => {
    Tokens.secret(secretId).submit();
  });

  JobDetail.visit({ id: job.id });
  ClientDetail.visit({ id: node.id });

  andThen(() => {
    const newRequests = server.pretender.handledRequests.slice(requestPosition);
    assert.ok(newRequests.length > 1, 'New requests have been made');

    // Cross-origin requests can't have a token
    newRequests.forEach(req => {
      assert.equal(getHeader(req, 'X-Nomad-Token'), secretId, `Token set for ${req.url}`);
    });
  });
});

test('an error message is shown when authenticating a token fails', function(assert) {
  const { secretId } = managementToken;
  const bogusSecret = 'this-is-not-the-secret';
  assert.notEqual(
    secretId,
    bogusSecret,
    'bogus secret is not somehow coincidentally equal to the real secret'
  );

  Tokens.visit();

  andThen(() => {
    Tokens.secret(bogusSecret).submit();

    andThen(() => {
      assert.ok(
        window.localStorage.nomadTokenSecret == null,
        'Token secret is discarded on failure'
      );
      assert.ok(Tokens.errorMessage, 'Token error message is shown');
      assert.notOk(Tokens.successMessage, 'Token success message is not shown');
      assert.equal(Tokens.policies.length, 0, 'No token policies are shown');
    });
  });
});

test('a success message and a special management token message are shown when authenticating succeeds', function(assert) {
  const { secretId } = managementToken;

  Tokens.visit();

  andThen(() => {
    Tokens.secret(secretId).submit();

    andThen(() => {
      assert.ok(Tokens.successMessage, 'Token success message is shown');
      assert.notOk(Tokens.errorMessage, 'Token error message is not shown');
      assert.ok(Tokens.managementMessage, 'Token management message is shown');
      assert.equal(Tokens.policies.length, 0, 'No token policies are shown');
    });
  });
});

test('a success message and associated policies are shown when authenticating succeeds', function(assert) {
  const { secretId } = clientToken;
  const policy = clientToken.policies.models[0];
  policy.update('description', 'Make sure there is a description');

  Tokens.visit();

  andThen(() => {
    Tokens.secret(secretId).submit();

    andThen(() => {
      assert.ok(Tokens.successMessage, 'Token success message is shown');
      assert.notOk(Tokens.errorMessage, 'Token error message is not shown');
      assert.notOk(Tokens.managementMessage, 'Token management message is not shown');
      assert.equal(
        Tokens.policies.length,
        clientToken.policies.length,
        'Each policy associated with the token is listed'
      );

      const policyElement = Tokens.policies.objectAt(0);

      assert.equal(policyElement.name, policy.name, 'Policy Name');
      assert.equal(policyElement.description, policy.description, 'Policy Description');
      assert.equal(policyElement.rules, policy.rules, 'Policy Rules');
    });
  });
});

test('setting a token clears the store', function(assert) {
  const { secretId } = clientToken;

  Jobs.visit();

  andThen(() => {
    assert.ok(find('.job-row'), 'Jobs found');
  });

  Tokens.visit();

  andThen(() => {
    Tokens.secret(secretId).submit();
  });

  // Don't return jobs from the API the second time around
  andThen(() => {
    server.pretender.get('/v1/jobs', function() {
      return [200, {}, '[]'];
    });
  });

  Jobs.visit();

  // If jobs are lingering in the store, they would show up
  assert.notOk(find('[data-test-job-row]'), 'No jobs found');
});

function getHeader({ requestHeaders }, name) {
  // Headers are case-insensitive, but object property look up is not
  return (
    requestHeaders[name] || requestHeaders[name.toLowerCase()] || requestHeaders[name.toUpperCase()]
  );
}
