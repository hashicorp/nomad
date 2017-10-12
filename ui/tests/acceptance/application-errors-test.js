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
  server.pretender.get('/v1/nodes', () => [403, {}, null]);

  visit('/nodes');

  andThen(() => {
    assert.ok(find('.error-message'), 'Application has errored');
  });

  visit('/jobs');

  andThen(() => {
    assert.notOk(find('.error-message'), 'Application is no longer in an error state');
  });
});
