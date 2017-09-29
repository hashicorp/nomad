import { fillIn, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let job;
let node;

moduleForAcceptance('Acceptance | tokens', {
  beforeEach() {
    server.create('agent');
    node = server.create('node');
    job = server.create('job');
  },
});

test('the token form sets the token in session storage', function(assert) {
  const secret = 'this-is-the-secret';
  const accessor = 'this-is-the-accessor';

  visit('/settings/tokens');

  andThen(() => {
    assert.ok(window.sessionStorage.nomadTokenSecret == null, 'No token secret set');
    assert.ok(window.sessionStorage.nomadTokenAccessor == null, 'No token accessor set');

    andThen(() => {
      fillIn('.token-secret', secret);
    });

    andThen(() => {
      assert.equal(window.sessionStorage.nomadTokenSecret, secret, 'Token secret was set');
      assert.ok(window.sessionStorage.nomadTokenAccessor == null, 'Token accessor was not set');
    });

    andThen(() => {
      fillIn('.token-accessor', accessor);
    });

    andThen(() => {
      assert.equal(window.sessionStorage.nomadTokenAccessor, accessor, 'Token accessor was set');
    });
  });
});

test('the X-Nomad-Token header gets sent with requests once it is set', function(assert) {
  const secret = 'this-is-the-secret';
  let requestPosition = 0;

  visit(`/jobs/${job.id}`);
  visit(`/nodes/${node.id}`);

  andThen(() => {
    assert.ok(server.pretender.handledRequests.length > 1, 'Requests have been made');

    server.pretender.handledRequests.forEach(req => {
      assert.notOk(req.requestHeaders['X-Nomad-Token'], `No token for ${req.url}`);
    });

    requestPosition = server.pretender.handledRequests.length;
  });

  visit('/settings/tokens');
  andThen(() => {
    fillIn('.token-secret', secret);
  });

  visit(`/jobs/${job.id}`);
  visit(`/nodes/${node.id}`);

  andThen(() => {
    const newRequests = server.pretender.handledRequests.slice(requestPosition);
    assert.ok(newRequests.length > 1, 'New requests have been made');

    newRequests.forEach(req => {
      assert.equal(req.requestHeaders['X-Nomad-Token'], secret, `Token set for ${req.url}`);
    });
  });
});
