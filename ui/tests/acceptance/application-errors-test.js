import { find, visit } from 'ember-native-dom-helpers';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { test } from 'qunit';

moduleForAcceptance('Acceptance | application errors ', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job');
  },
});

test('transitioning away from an error page resets the global error', function(assert) {
  server.pretender.get('/v1/nodes', () => [500, {}, null]);

  visit('/nodes');

  andThen(() => {
    assert.ok(find('.error-message'), 'Application has errored');
  });

  visit('/jobs');

  andThen(() => {
    assert.notOk(find('.error-message'), 'Application is no longer in an error state');
  });
});

test('the 403 error page links to the ACL tokens page', function(assert) {
  const job = server.db.jobs[0];

  server.pretender.get(`/v1/job/${job.id}`, () => [403, {}, null]);

  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.ok(find('.error-message'), 'Error message is shown');
    assert.equal(
      find('.error-message .title').textContent,
      'Not Authorized',
      'Error message is for 403'
    );
  });

  andThen(() => {
    click('.error-message a');
  });

  andThen(() => {
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Error message contains a link to the tokens page'
    );
  });
});
