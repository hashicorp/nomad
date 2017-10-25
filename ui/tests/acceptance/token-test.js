import { find, findAll, fillIn, click, visit } from 'ember-native-dom-helpers';
import Ember from 'ember';
import { test, skip } from 'ember-qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const { $ } = Ember;

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

test('the token form sets the token in session storage', function(assert) {
  const { secretId } = managementToken;

  visit('/settings/tokens');

  andThen(() => {
    assert.ok(window.sessionStorage.nomadTokenSecret == null, 'No token secret set');

    fillIn('.token-secret', secretId);
    click('.token-submit');

    andThen(() => {
      assert.equal(window.sessionStorage.nomadTokenSecret, secretId, 'Token secret was set');
    });
  });
});

// TODO: unskip once store.unloadAll reliably waits for in-flight requests to settle
skip('the X-Nomad-Token header gets sent with requests once it is set', function(assert) {
  const { secretId } = managementToken;
  let requestPosition = 0;

  visit(`/jobs/${job.id}`);
  visit(`/nodes/${node.id}`);

  andThen(() => {
    assert.ok(server.pretender.handledRequests.length > 1, 'Requests have been made');

    server.pretender.handledRequests.forEach(req => {
      assert.notOk(getHeader(req, 'X-Nomad-Token'), `No token for ${req.url}`);
    });

    requestPosition = server.pretender.handledRequests.length;
  });

  visit('/settings/tokens');
  andThen(() => {
    fillIn('.token-secret', secretId);
    click('.token-submit');
  });

  visit(`/jobs/${job.id}`);
  visit(`/nodes/${node.id}`);

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

  visit('/settings/tokens');

  andThen(() => {
    fillIn('.token-secret', bogusSecret);
    click('.token-submit');

    andThen(() => {
      assert.ok(
        window.sessionStorage.nomadTokenSecret == null,
        'Token secret is discarded on failure'
      );
      assert.ok(find('.token-error'), 'Token error message is shown');
      assert.notOk(find('.token-success'), 'Token success message is not shown');
      assert.notOk(find('.token-policy'), 'No token policies are shown');
    });
  });
});

test('a success message and a special management token message are shown when authenticating succeeds', function(
  assert
) {
  const { secretId } = managementToken;

  visit('/settings/tokens');

  andThen(() => {
    fillIn('.token-secret', secretId);
    click('.token-submit');

    andThen(() => {
      assert.ok(find('.token-success'), 'Token success message is shown');
      assert.notOk(find('.token-error'), 'Token error message is not shown');
      assert.ok(find('.token-management-message'), 'Token management message is shown');
      assert.notOk(find('.token-policy'), 'No token policies are shown');
    });
  });
});

test('a success message and associated policies are shown when authenticating succeeds', function(
  assert
) {
  const { secretId } = clientToken;
  const policy = clientToken.policies.models[0];
  policy.update('description', 'Make sure there is a description');

  visit('/settings/tokens');

  andThen(() => {
    fillIn('.token-secret', secretId);
    click('.token-submit');

    andThen(() => {
      assert.ok(find('.token-success'), 'Token success message is shown');
      assert.notOk(find('.token-error'), 'Token error message is not shown');
      assert.notOk(find('.token-management-message'), 'Token management message is not shown');
      assert.equal(
        findAll('.token-policy').length,
        clientToken.policies.length,
        'Each policy associated with the token is listed'
      );

      const policyElement = $(find('.token-policy'));

      assert.equal(
        policyElement
          .find('.boxed-section-head')
          .text()
          .trim(),
        policy.name,
        'Policy Name'
      );
      assert.equal(
        policyElement
          .find('.boxed-section-body p.content')
          .text()
          .trim(),
        policy.description,
        'Policy Description'
      );
      assert.equal(
        policyElement.find('.boxed-section-body pre code').text(),
        policy.rules,
        'Policy Rules'
      );
    });
  });
});

test('setting a token clears the store', function(assert) {
  const { secretId } = clientToken;

  visit('/jobs');

  andThen(() => {
    assert.ok(find('.job-row'), 'Jobs found');
  });

  visit('/settings/tokens');

  andThen(() => {
    fillIn('.token-secret', secretId);
    click('.token-submit');
  });

  // Don't return jobs from the API the second time around
  andThen(() => {
    server.pretender.get('/v1/jobs', function() {
      return [200, {}, '[]'];
    });
  });

  visit('/jobs');

  // If jobs are lingering in the store, they would show up
  assert.notOk(find('.job-row'), 'No jobs found');
});

function getHeader({ requestHeaders }, name) {
  // Headers are case-insensitive, but object property look up is not
  return (
    requestHeaders[name] || requestHeaders[name.toLowerCase()] || requestHeaders[name.toUpperCase()]
  );
}
